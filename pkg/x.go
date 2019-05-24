package x

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/otiai10/copy"
	"k8s.io/klog"

	"gopkg.in/yaml.v3"
)

type KustomizeOpts struct {
	Images     []KustomizeImage `yaml:"images"`
	NamePrefix string           `yaml:"namePrefix"`
	NameSuffix string           `yaml:"nameSuffix"`
	Namespace  string           `yaml:"Namespace"`
}

type KustomizeImage struct {
	Name    string `yaml:"name"`
	NewName string `yaml:"newName"`
	NewTag  string `yaml:"newTag"`
	Digest  string `yaml:"digest"`
}

func (img KustomizeImage) String() string {
	res := img.Name
	if img.NewName != "" {
		res = res + "=" + img.NewName
	}
	if img.NewTag != "" {
		res = res + ":" + img.NewTag
	}
	if img.Digest != "" {
		res = res + "@" + img.Digest
	}
	return res
}

type ApplyOpts struct {
	*ChartifyOpts

	chart   string
	DryRun  bool
	Install bool
	Timeout int

	TLS     bool
	TLSCert string
	TLSKey  string

	Adopt []string

	Out io.Writer
}

type TemplateOpts struct {
	*ChartifyOpts

	IncludeReleaseConfigmap bool
	IncludeReleaseSecret    bool

	Out io.Writer
}

type AdoptOpts struct {
	*ClientOpts

	Namespace string

	Out io.Writer
}

type ChartifyOpts struct {
	// Debug when set to true passes `--debug` flag to `helm` in order to enable debug logging
	Debug bool

	// ReleaseName is the name of Helm release being installed
	ReleaseName string

	// ValuesFiles are a list of Helm chart values files
	ValuesFiles []string

	// SetValues is a list of adhoc Helm chart values being passed via helm's `--set` flags
	SetValues []string

	// Namespace is the default namespace in which the K8s manifests rendered by the chart are associated
	Namespace string

	// ChartVersion is the semver of the Helm chart being used to render the original K8s manifests before various tweaks applied by helm-x
	ChartVersion string

	// KubeContext is the
	// TODO: This isn't actually an option for chartify. Move to elsewhere!
	KubeContext string

	// TillerNamespace is the namespace Tiller or Helm v3 creates "release" objects(configmaps or secrets depending on the storage backend chosen)
	TillerNamespace string

	Injectors []string
	Injects   []string

	AdhocChartDependencies []string

	JsonPatches []string

	StrategicMergePatches []string
}

type ClientOpts struct {
	KubeContext     string
	TillerNamespace string
	TLS             bool
	TLSCert         string
	TLSKey          string
}

// Chartify creates a temporary Helm chart from a directory or a remote chart, and applies various transformations.
// Returns the full path to the temporary directory containing the generated chart if succeeded.
func Chartify(dirOrChart string, u ChartifyOpts) (string, error) {
	tempDir, err := copyToTempDir(dirOrChart)
	if err != nil {
		return "", err
	}

	isChart, err := exists(filepath.Join(tempDir, "Chart.yaml"))
	if err != nil {
		return "", err
	}

	generatedManifestFiles := []string{}

	if isChart {
		templateFileOptions := fileOptions{
			basePath:     tempDir,
			matchSubPath: "templates",
			fileType:     "yaml",
		}
		templateFiles, err := getFilesToActOn(templateFileOptions)
		if err != nil {
			return "", err
		}

		templateOptions := templateOptions{
			files:       templateFiles,
			chart:       tempDir,
			name:        u.ReleaseName,
			namespace:   u.Namespace,
			values:      u.SetValues,
			valuesFiles: u.ValuesFiles,
		}
		if err := template(templateOptions); err != nil {
			return "", err
		}

		generatedManifestFiles = append([]string{}, templateFiles...)
	}

	dstTemplatesDir := filepath.Join(tempDir, "templates")
	dirExists, err := exists(dstTemplatesDir)
	if err != nil {
		return "", err
	}
	if !dirExists {
		if err := os.Mkdir(dstTemplatesDir, 0755); err != nil {
			return "", err
		}
	}

	isKustomization, err := exists(filepath.Join(tempDir, "kustomization.yaml"))
	if err != nil {
		return "", err
	}

	if isKustomization {
		kustomizeOpts := KustomizeOpts{}

		for _, f := range u.ValuesFiles {
			valsFileContent, err := ioutil.ReadFile(f)
			if err != nil {
				return "", err
			}
			if err := yaml.Unmarshal(valsFileContent, &kustomizeOpts); err != nil {
				return "", err
			}
		}

		if len(u.SetValues) > 0 {
			panic("--set is not yet supported for kustomize-based apps! Use -f/--values flag instead.")
		}

		prevDir, err := os.Getwd()
		if err != nil {
			return "", err
		}
		defer func() {
			if err := os.Chdir(prevDir); err != nil {
				panic(err)
			}
		}()
		if err := os.Chdir(tempDir); err != nil {
			return "", err
		}

		if len(kustomizeOpts.Images) > 0 {
			args := []string{"edit", "set", "image"}
			for _, image := range kustomizeOpts.Images {
				args = append(args, image.String())
			}
			_, err := RunCommand("kustomize", args...)
			if err != nil {
				return "", err
			}
		}
		if kustomizeOpts.NamePrefix != "" {
			_, err := RunCommand("kustomize", "edit", "set", "nameprefix", kustomizeOpts.NamePrefix)
			if err != nil {
				fmt.Println(err)
				return "", err
			}
		}
		if kustomizeOpts.NameSuffix != "" {
			// "--" is there to avoid `namesuffix -acme` to fail due to `-a` being considered as a flag
			_, err := RunCommand("kustomize", "edit", "set", "namesuffix", "--", kustomizeOpts.NameSuffix)
			if err != nil {
				return "", err
			}
		}
		if kustomizeOpts.Namespace != "" {
			_, err := RunCommand("kustomize", "edit", "set", "Namespace", kustomizeOpts.Namespace)
			if err != nil {
				return "", err
			}
		}
		kustomizeFile := filepath.Join(dstTemplatesDir, "kustomized.yaml")
		out, err := RunCommand("kustomize", "-o", kustomizeFile, "build", tempDir)
		if err != nil {
			return "", err
		}
		fmt.Println(out)

		generatedManifestFiles = append(generatedManifestFiles, kustomizeFile)
	}

	if !isChart && !isKustomization {
		manifestFileOptions := fileOptions{
			basePath: tempDir,
			fileType: "yaml",
		}
		manifestFiles, err := getFilesToActOn(manifestFileOptions)
		if err != nil {
			return "", err
		}
		for _, f := range manifestFiles {
			dst := filepath.Join(dstTemplatesDir, filepath.Base(f))
			if err := os.Rename(f, dst); err != nil {
				return "", err
			}
			generatedManifestFiles = append(generatedManifestFiles, dst)
		}
	}

	var requirementsYamlContent string
	if !isChart {
		if u.ChartVersion == "" {
			return "", fmt.Errorf("--version is required when applying manifests")
		}
		chartyaml := fmt.Sprintf("name: \"%s\"\nversion: %s\nappVersion: %s\n", u.ReleaseName, u.ChartVersion, u.ChartVersion)
		if err := ioutil.WriteFile(filepath.Join(tempDir, "Chart.yaml"), []byte(chartyaml), 0644); err != nil {
			return "", err
		}
	} else {
		bytes, err := ioutil.ReadFile(filepath.Join(tempDir, "requirements.yaml"))
		if os.IsNotExist(err) {
			requirementsYamlContent = `dependencies:`
		} else if err != nil {
			return "", err
		} else {
			parsed := map[string]interface{}{}
			if err := yaml.Unmarshal(bytes, &parsed); err != nil {
				return "", err
			}
			if _, ok := parsed["dependencies"]; !ok {
				bytes = []byte(`dependencies:`)
			}
			requirementsYamlContent = string(bytes)
		}
	}

	for _, d := range u.AdhocChartDependencies {
		aliasChartVer := strings.Split(d, "=")
		chartAndVer := strings.Split(aliasChartVer[len(aliasChartVer)-1], ":")
		repoAndChart := strings.Split(chartAndVer[0], "/")
		repo := repoAndChart[0]
		chart := repoAndChart[1]
		var ver string
		if len(chartAndVer) == 1 {
			ver = "*"
		} else {
			ver = chartAndVer[1]
		}
		var alias string
		if len(aliasChartVer) == 1 {
			alias = chart
		} else {
			alias = aliasChartVer[0]
		}

		var repoUrl string
		out, err := RunCommand("helm", "repo", "list")
		if err != nil {
			return "", err
		}
		lines := strings.Split(out, "\n")
		re := regexp.MustCompile(`\s+`)
		for lineNum, line := range lines {
			if lineNum == 0 {
				continue
			}
			tokens := re.Split(line, -1)
			if len(tokens) < 2 {
				return "", fmt.Errorf("unexpected format of `helm repo list` at line %d \"%s\" in:\n%s", lineNum, line, out)
			}
			if tokens[0] == repo {
				repoUrl = tokens[1]
				break
			}
		}
		if repoUrl == "" {
			return "", fmt.Errorf("no helm list entry found for repository \"%s\"", repo)
		}

		requirementsYamlContent = requirementsYamlContent + fmt.Sprintf(`
- name: %s
  repository: %s
  condition: %s.enabled
  alias: %s
`, chart, repoUrl, alias, alias)
		requirementsYamlContent = requirementsYamlContent + fmt.Sprintf(`  version: "%s"
`, ver)
	}

	if err := ioutil.WriteFile(filepath.Join(tempDir, "requirements.yaml"), []byte(requirementsYamlContent), 0644); err != nil {
		return "", err
	}

	{
		debugOut, err := ioutil.ReadFile(filepath.Join(tempDir, "requirements.yaml"))
		if err != nil {
			return "", err
		}
		klog.Infof("using requirements.yaml:\n%s", debugOut)
	}

	{
		_, err := RunCommand("helm", "dependency", "build", tempDir)
		if err != nil {
			return "", err
		}

		matches, err := filepath.Glob(filepath.Join(tempDir, "charts", "*-*.tgz"))
		if err != nil {
			return "", err
		}

		for _, match := range matches {
			chartsDir := filepath.Join(tempDir, "charts")

			klog.Infof("unarchiving subchart %s to %s", match, chartsDir)
			subchartDir, err := untarUnderDir(match, chartsDir)
			if err != nil {
				return "", fmt.Errorf("fetchAndUntarUnderDir: %v", err)
			}

			templateFileOptions := fileOptions{
				basePath:     subchartDir,
				matchSubPath: "templates",
				fileType:     "yaml",
			}
			templateFiles, err := getFilesToActOn(templateFileOptions)
			if err != nil {
				return "", err
			}

			templateOptions := templateOptions{
				files:       templateFiles,
				chart:       subchartDir,
				name:        u.ReleaseName,
				namespace:   u.Namespace,
				values:      u.SetValues,
				valuesFiles: u.ValuesFiles,
			}
			if err := template(templateOptions); err != nil {
				return "", err
			}

			generatedManifestFiles = append([]string{}, templateFiles...)
		}

		_ = os.Remove(filepath.Join(tempDir, "requirements.yaml"))
		_ = os.Remove(filepath.Join(tempDir, "requirements.lock"))
	}

	{
		if isChart && (len(u.JsonPatches) > 0 || len(u.StrategicMergePatches) > 0) {
			kustomizationYamlContent := `kind: ""
apiversion: ""
resources:
`
			for _, f := range generatedManifestFiles {
				f = strings.Replace(f, tempDir+"/", "", 1)
				kustomizationYamlContent += `- ` + f + "\n"
			}

			if len(u.JsonPatches) > 0 {
				kustomizationYamlContent += `patchesJson6902:
`
				for i, f := range u.JsonPatches {
					fileBytes, err := ioutil.ReadFile(f)
					if err != nil {
						return "", err
					}

					type jsonPatch struct {
						Target map[string]string        `yaml:"target"`
						Patch  []map[string]interface{} `yaml:"patch"`
						Path   string                   `yaml:"path"`
					}
					patch := jsonPatch{}
					if err := yaml.Unmarshal(fileBytes, &patch); err != nil {
						return "", err
					}

					buf := &bytes.Buffer{}
					encoder := yaml.NewEncoder(buf)
					encoder.SetIndent(2)
					if err := encoder.Encode(map[string]interface{}{"target": patch.Target}); err != nil {
						return "", err
					}
					targetBytes := buf.Bytes()

					for i, line := range strings.Split(string(targetBytes), "\n") {
						if i == 0 {
							line = "- " + line
						} else {
							line = "  " + line
						}
						kustomizationYamlContent += line + "\n"
					}

					var path string
					if patch.Path != "" {
						path = patch.Path
					} else if len(patch.Patch) > 0 {
						buf := &bytes.Buffer{}
						encoder := yaml.NewEncoder(buf)
						encoder.SetIndent(2)
						err := encoder.Encode(patch.Patch)
						if err != nil {
							return "", err
						}
						jsonPatchData := buf.Bytes()
						path = filepath.Join("jsonpatches", fmt.Sprintf("patch.%d.yaml", i))
						abspath := filepath.Join(tempDir, path)
						if err := os.Mkdir(filepath.Dir(abspath), 0755); err != nil {
							return "", err
						}
						klog.Infof("%s:\n%s", path, jsonPatchData)
						if err := ioutil.WriteFile(abspath, jsonPatchData, 0644); err != nil {
							return "", err
						}
					} else {
						return "", fmt.Errorf("either \"path\" or \"patch\" must be set in %s", f)
					}
					kustomizationYamlContent += "  path: " + path + "\n"
				}
			}

			if len(u.StrategicMergePatches) > 0 {
				kustomizationYamlContent += `patchesStrategicMerge:
`
				for i, f := range u.StrategicMergePatches {
					bytes, err := ioutil.ReadFile(f)
					if err != nil {
						return "", err
					}
					path := filepath.Join("strategicmergepatches", fmt.Sprintf("patch.%d.yaml", i))
					abspath := filepath.Join(tempDir, path)
					if err := os.Mkdir(filepath.Dir(abspath), 0755); err != nil {
						return "", err
					}
					if err := ioutil.WriteFile(abspath, bytes, 0644); err != nil {
						return "", err
					}
					kustomizationYamlContent += `- ` + path + "\n"
				}
			}

			if err := ioutil.WriteFile(filepath.Join(tempDir, "kustomization.yaml"), []byte(kustomizationYamlContent), 0644); err != nil {
				return "", err
			}

			klog.Infof("generated and using kustomization.yaml:\n%s", kustomizationYamlContent)

			renderedFile := filepath.Join(tempDir, "templates/rendered.yaml")
			klog.Infof("generating %s", renderedFile)
			_, err := RunCommand("kustomize", "build", tempDir, "--output", renderedFile)
			if err != nil {
				return "", err
			}

			for _, f := range generatedManifestFiles {
				klog.Infof("removing %s", f)
				if err := os.Remove(f); err != nil {
					return "", err
				}
			}

			generatedManifestFiles = []string{renderedFile}
		}
	}

	injectOptions := InjectOpts{
		injectors: u.Injectors,
		injects:   u.Injects,
		files:     generatedManifestFiles,
	}
	if err := Inject(injectOptions); err != nil {
		return "", err
	}

	return tempDir, nil
}

func Adopt(tillerNs, release, namespace string, resources []string) error {
	storage, err := NewConfigMapsStorage(tillerNs)
	if err != nil {
		return err
	}

	kubectlArgs := []string{"get", "-o=json", "--export"}

	var ns string
	if namespace != "" {
		ns = namespace
	} else {
		ns = "default"
	}
	kubectlArgs = append(kubectlArgs, "-n="+ns)

	kubectlArgs = append(kubectlArgs, resources...)

	jsonData, err := RunCommand("kubectl", kubectlArgs...)
	if err != nil {
		return err
	}

	var manifest string

	if len(resources) == 1 {
		item := map[string]interface{}{}

		if err := json.Unmarshal([]byte(jsonData), &item); err != nil {
			return err
		}

		yamlData, err := YamlMarshal(item)
		if err != nil {
			return err
		}

		item = export(item)

		yamlData, err = YamlMarshal(item)
		if err != nil {
			return err
		}

		metadata := item["metadata"].(map[string]interface{})
		escaped := fmt.Sprintf("%s.%s", metadata["name"], strings.ToLower(item["kind"].(string)))
		manifest += manifest + fmt.Sprintf("\n---\n# Source: helm-x-dummy-chart/templates/%s.yaml\n", escaped) + string(yamlData)
	} else {
		type jsonVal struct {
			Items []map[string]interface{} `json:"items"`
		}
		v := jsonVal{}

		if err := json.Unmarshal([]byte(jsonData), &v); err != nil {
			return err
		}

		for _, item := range v.Items {
			yamlData, err := YamlMarshal(item)
			if err != nil {
				return err
			}

			item = export(item)

			yamlData, err = YamlMarshal(item)
			if err != nil {
				return err
			}

			metadata := item["metadata"].(map[string]interface{})
			escaped := fmt.Sprintf("%s.%s", metadata["name"], strings.ToLower(item["kind"].(string)))
			manifest += manifest + fmt.Sprintf("\n---\n# Source: helm-x-dummy-chart/templates/%s.yaml\n", escaped) + string(yamlData)
		}
	}

	if manifest == "" {
		return fmt.Errorf("no resources to be adopted")
	}

	if err := storage.AdoptRelease(release, ns, manifest); err != nil {
		return err
	}

	return nil
}

func export(item map[string]interface{}) map[string]interface{} {
	metadata := item["metadata"].(map[string]interface{})
	if generateName, ok := metadata["generateName"]; ok {
		metadata["name"] = generateName
	}

	delete(metadata, "generateName")
	delete(metadata, "generation")
	delete(metadata, "resourceVersion")
	delete(metadata, "selfLink")
	delete(metadata, "uid")

	item["metadata"] = metadata

	delete(item, "status")

	return item
}

// copyToTempDir checks if the path is local or a repo (in this order) and copies it to a temp directory
// It will perform a `helm fetch` if required
func copyToTempDir(path string) (string, error) {
	tempDir := mkRandomDir(os.TempDir())
	exists, err := exists(path)
	if err != nil {
		return "", err
	}
	if !exists {
		return fetchAndUntarUnderDir(path, tempDir)
	}
	err = copy.Copy(path, tempDir)
	if err != nil {
		return "", err
	}
	return tempDir, nil
}

func fetchAndUntarUnderDir(path, tempDir string) (string, error) {
	command := fmt.Sprintf("helm fetch %s --untar -d %s", path, tempDir)
	_, stderr, err := Capture(command)
	if err != nil || len(stderr) != 0 {
		return "", fmt.Errorf(string(stderr))
	}
	files, err := ioutil.ReadDir(tempDir)
	if err != nil {
		return "", err
	}
	if len(files) != 1 {
		return "", fmt.Errorf("%d additional files found in temp direcotry. This is very strange", len(files)-1)
	}
	return filepath.Join(tempDir, files[0].Name()), nil
}

func untarUnderDir(path, tempDir string) (string, error) {
	command := fmt.Sprintf("tar -zxvf %s -C %s", path, tempDir)
	_, stderr, err := Capture(command)
	if err != nil {
		return "", fmt.Errorf("%v: %s", err, string(stderr))
	}
	if err := os.Remove(path); err != nil {
		return "", err
	}
	files, err := ioutil.ReadDir(tempDir)
	if err != nil {
		return "", err
	}
	if len(files) != 1 {
		fs := []string{}
		for _, f := range files {
			fs = append(fs, f.Name())
		}
		return "", fmt.Errorf("%d additional files found in temp direcotry. This is very strange:\n%s", len(files)-1, strings.Join(fs, "\n"))
	}
	return filepath.Join(tempDir, files[0].Name()), nil
}

type fileOptions struct {
	basePath     string
	matchSubPath string
	fileType     string
}

// getFilesToActOn returns a slice of files that are within the base path, has a matching sub path and file type
func getFilesToActOn(o fileOptions) ([]string, error) {
	var files []string

	err := filepath.Walk(o.basePath, func(path string, info os.FileInfo, err error) error {
		if !strings.Contains(path, o.matchSubPath+"/") {
			return nil
		}
		if !strings.HasSuffix(path, o.fileType) {
			return nil
		}
		files = append(files, path)
		return nil
	})

	if err != nil {
		return nil, err
	}

	return files, nil
}

type templateOptions struct {
	files       []string
	chart       string
	name        string
	values      []string
	valuesFiles []string
	namespace   string
}

func template(o templateOptions) error {
	var additionalFlags string
	additionalFlags += createFlagChain("set", o.values)
	defaultValuesPath := filepath.Join(o.chart, "values.yaml")
	exists, err := exists(defaultValuesPath)
	if err != nil {
		return err
	}
	if exists {
		additionalFlags += createFlagChain("f", []string{defaultValuesPath})
	}
	additionalFlags += createFlagChain("f", o.valuesFiles)
	if o.namespace != "" {
		additionalFlags += createFlagChain("Namespace", []string{o.namespace})
	}

	for _, file := range o.files {
		command := fmt.Sprintf("helm template --debug=false %s --name %s -x %s%s", o.chart, o.name, file, additionalFlags)
		stdout, stderr, err := Capture(command)
		if err != nil || len(stderr) != 0 {
			return fmt.Errorf(string(stderr))
		}
		if err := ioutil.WriteFile(file, stdout, 0644); err != nil {
			return err
		}
	}

	return nil
}

type InjectOpts struct {
	injectors []string
	injects   []string
	files     []string
}

func Inject(o InjectOpts) error {
	var flagsTemplate string
	for _, inj := range o.injectors {

		tokens := strings.Split(inj, ",")
		injector := tokens[0]
		injectFlags := tokens[1:]
		for _, flag := range injectFlags {
			flagSplit := strings.Split(flag, "=")
			switch len(flagSplit) {
			case 1:
				flagsTemplate += flagSplit[0]
			case 2:
				key, val := flagSplit[0], flagSplit[1]
				flagsTemplate += createFlagChain(key, []string{val})
			default:
				return fmt.Errorf("inject-flags must be in the form of key1=value1[,key2=value2,...]: %v", flag)
			}
		}
		for _, file := range o.files {
			flags := strings.Replace(flagsTemplate, "FILE", file, 1)
			command := fmt.Sprintf("%s %s", injector, flags)
			stdout, stderr, err := Capture(command)
			if err != nil {
				return fmt.Errorf(string(stderr))
			}
			if err := ioutil.WriteFile(file, stdout, 0644); err != nil {
				return err
			}
		}
	}

	for _, tmpl := range o.injects {
		for _, file := range o.files {
			cmd := strings.Replace(tmpl, "FILE", file, 1)

			stdout, stderr, err := Capture(cmd)
			if err != nil {
				return fmt.Errorf(string(stderr))
			}
			if err := ioutil.WriteFile(file, stdout, 0644); err != nil {
				return err
			}
		}
	}

	return nil
}

type UpgradeOpts struct {
	Chart       string
	ReleaseName string
	SetValues   []string
	ValuesFiles []string
	Namespace   string
	KubeContext string
	Timeout     int
	Install     bool
	DryRun      bool
	Debug       bool
	TLS         bool
	TLSCert     string
	TLSKey      string
	kubeConfig  string
}

func Upgrade(o UpgradeOpts) error {
	var additionalFlags string
	additionalFlags += createFlagChain("set", o.SetValues)
	additionalFlags += createFlagChain("f", o.ValuesFiles)
	additionalFlags += createFlagChain("timeout", []string{fmt.Sprintf("%d", o.Timeout)})
	if o.Install {
		additionalFlags += createFlagChain("install", []string{""})
	}
	if o.Namespace != "" {
		additionalFlags += createFlagChain("Namespace", []string{o.Namespace})
	}
	if o.KubeContext != "" {
		additionalFlags += createFlagChain("kube-context", []string{o.KubeContext})
	}
	if o.DryRun {
		additionalFlags += createFlagChain("dry-run", []string{""})
	}
	if o.Debug {
		additionalFlags += createFlagChain("debug", []string{""})
	}
	if o.TLS {
		additionalFlags += createFlagChain("tls", []string{""})
	}
	if o.TLSCert != "" {
		additionalFlags += createFlagChain("tls-cert", []string{o.TLSCert})
	}
	if o.TLSKey != "" {
		additionalFlags += createFlagChain("tls-key", []string{o.TLSKey})
	}

	command := fmt.Sprintf("helm upgrade %s %s%s", o.ReleaseName, o.Chart, additionalFlags)
	stdout, stderr, err := Capture(command)
	if err != nil || len(stderr) != 0 {
		return fmt.Errorf(string(stderr))
	}
	fmt.Println(string(stdout))

	return nil
}

func Template(chart string, o TemplateOpts) error {
	var additionalFlags string
	additionalFlags += createFlagChain("set", o.SetValues)
	additionalFlags += createFlagChain("f", o.ValuesFiles)
	if o.Namespace != "" {
		additionalFlags += createFlagChain("Namespace", []string{o.Namespace})
	}
	if o.KubeContext != "" {
		additionalFlags += createFlagChain("kube-context", []string{o.KubeContext})
	}
	if o.ReleaseName != "" {
		additionalFlags += createFlagChain("name", []string{o.ReleaseName})
	}
	if o.Debug {
		additionalFlags += createFlagChain("debug", []string{""})
	}
	if o.ChartVersion != "" {
		additionalFlags += createFlagChain("--version", []string{o.ChartVersion})
	}

	command := fmt.Sprintf("helm template %s%s", chart, additionalFlags)
	stdout, stderr, err := Capture(command)
	if err != nil || len(stderr) != 0 {
		return fmt.Errorf(string(stderr))
	}

	var output string

	if o.IncludeReleaseConfigmap || o.IncludeReleaseSecret {
		repoNameAndChart := strings.Split(chart, "/")

		chartWithoutRepoName := repoNameAndChart[len(repoNameAndChart)-1]

		ver := o.ChartVersion

		releaseManifests := []ReleaseManifest{}

		if o.IncludeReleaseConfigmap {
			releaseManifests = append(releaseManifests, ReleaseToConfigMap)
		}

		if o.IncludeReleaseSecret {
			releaseManifests = append(releaseManifests, ReleaseToSecret)
		}

		output, err = TurnHelmTemplateToInstall(chartWithoutRepoName, ver, o.TillerNamespace, o.ReleaseName, o.Namespace, string(stdout), releaseManifests...)
		if err != nil {
			return err
		}
	} else {
		output = string(stdout)
	}

	fmt.Println(output)

	return nil
}

type DiffOpts struct {
	*ChartifyOpts

	Chart       string
	ReleaseName string
	SetValues   []string
	ValuesFiles []string
	Namespace   string
	KubeContext string
	TLS         bool
	TLSCert     string
	TLSKey      string
	kubeConfig  string

	Out io.Writer
}

func Diff(o DiffOpts) error {
	var additionalFlags string
	additionalFlags += createFlagChain("set", o.SetValues)
	additionalFlags += createFlagChain("f", o.ValuesFiles)
	additionalFlags += createFlagChain("allow-unreleased", []string{""})
	additionalFlags += createFlagChain("detailed-exitcode", []string{""})
	additionalFlags += createFlagChain("context", []string{"3"})
	additionalFlags += createFlagChain("reset-values", []string{""})
	additionalFlags += createFlagChain("suppress-secrets", []string{""})
	if o.Namespace != "" {
		additionalFlags += createFlagChain("Namespace", []string{o.Namespace})
	}
	if o.KubeContext != "" {
		additionalFlags += createFlagChain("kube-context", []string{o.KubeContext})
	}
	if o.TLS {
		additionalFlags += createFlagChain("tls", []string{""})
	}
	if o.TLSCert != "" {
		additionalFlags += createFlagChain("tls-cert", []string{o.TLSCert})
	}
	if o.TLSKey != "" {
		additionalFlags += createFlagChain("tls-key", []string{o.TLSKey})
	}

	command := fmt.Sprintf("helm diff upgrade %s %s%s", o.ReleaseName, o.Chart, additionalFlags)
	err := Exec(command)
	if err != nil {
		return err
	}

	return nil
}

// Exec takes a command as a string and executes it
func Exec(cmd string) error {
	klog.Infof("running %s", cmd)
	args := strings.Split(cmd, " ")
	binary := args[0]
	_, err := exec.LookPath(binary)
	if err != nil {
		return err
	}

	command := exec.Command(binary, args[1:]...)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	err = command.Run()
	return err
}

// Capture takes a command as a string and executes it, and returns the captured stdout and stderr
func Capture(cmd string) ([]byte, []byte, error) {
	klog.Infof("running %s", cmd)
	args := strings.Split(cmd, " ")
	binary := args[0]
	_, err := exec.LookPath(binary)
	if err != nil {
		return nil, nil, err
	}

	command := exec.Command(binary, args[1:]...)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err = command.Run()
	if err != nil {
		log.Print(stderr.String())
		log.Fatal(err)
	}
	return stdout.Bytes(), stderr.Bytes(), err
}

// MkRandomDir creates a new directory with a random name made of numbers
func mkRandomDir(basepath string) string {
	r := strconv.Itoa((rand.New(rand.NewSource(time.Now().UnixNano()))).Int())
	path := filepath.Join(basepath, r)
	os.Mkdir(path, 0755)

	return path
}

// exists returns whether the given file or directory exists or not
func exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return true, err
}

func createFlagChain(flag string, input []string) string {
	chain := ""
	dashes := "--"
	if len(flag) == 1 {
		dashes = "-"
	}

	for _, i := range input {
		if i != "" {
			i = " " + i
		}
		chain = fmt.Sprintf("%s %s%s%s", chain, dashes, flag, i)
	}

	return chain
}
