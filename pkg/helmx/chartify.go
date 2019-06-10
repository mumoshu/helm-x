package helmx

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"k8s.io/klog"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

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

	// TillerNamespace is the namespace Tiller or Helm v3 creates "release" objects(configmaps or secrets depending on the storage backend chosen)
	TillerNamespace string

	Injectors []string
	Injects   []string

	AdhocChartDependencies []string

	JsonPatches []string

	StrategicMergePatches []string
}

// Chartify creates a temporary Helm chart from a directory or a remote chart, and applies various transformations.
// Returns the full path to the temporary directory containing the generated chart if succeeded.
func (r *Runner) Chartify(dirOrChart string, u ChartifyOpts) (string, error) {
	tempDir, err := r.copyToTempDir(dirOrChart)
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
		if err := r.template(templateOptions); err != nil {
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
			_, err := r.Run("kustomize", args...)
			if err != nil {
				return "", err
			}
		}
		if kustomizeOpts.NamePrefix != "" {
			_, err := r.Run("kustomize", "edit", "set", "nameprefix", kustomizeOpts.NamePrefix)
			if err != nil {
				fmt.Println(err)
				return "", err
			}
		}
		if kustomizeOpts.NameSuffix != "" {
			// "--" is there to avoid `namesuffix -acme` to fail due to `-a` being considered as a flag
			_, err := r.Run("kustomize", "edit", "set", "namesuffix", "--", kustomizeOpts.NameSuffix)
			if err != nil {
				return "", err
			}
		}
		if kustomizeOpts.Namespace != "" {
			_, err := r.Run("kustomize", "edit", "set", "namespace", kustomizeOpts.Namespace)
			if err != nil {
				return "", err
			}
		}
		kustomizeFile := filepath.Join(dstTemplatesDir, "kustomized.yaml")
		out, err := r.Run("kustomize", "-o", kustomizeFile, "build", tempDir)
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
		out, err := r.Run("helm", "repo", "list")
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
		_, err := r.Run("helm", "dependency", "build", tempDir)
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
			subchartDir, err := r.untarUnderDir(match, chartsDir)
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
			if err := r.template(templateOptions); err != nil {
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
			_, err := r.Run("kustomize", "build", tempDir, "--output", renderedFile)
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
	if err := r.Inject(injectOptions); err != nil {
		return "", err
	}

	return tempDir, nil
}
