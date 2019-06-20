package helmx

import (
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

	//ReleaseName string

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

	JsonPatches           []string
	StrategicMergePatches []string
}

type ChartifyOption interface {
	SetChartifyOption(opts *ChartifyOpts) error
}

type chartifyOptsSetter struct {
	o *ChartifyOpts
}

func (s *chartifyOptsSetter) SetChartifyOption(opts *ChartifyOpts) error {
	*opts = *s.o
	return nil
}

func (s *ChartifyOpts) SetChartifyOption(opts *ChartifyOpts) error {
	*opts = *s
	return nil
}

func WithChartifyOpts(opts *ChartifyOpts) ChartifyOption {
	return &chartifyOptsSetter{
		o: opts,
	}
}

// Chartify creates a temporary Helm chart from a directory or a remote chart, and applies various transformations.
// Returns the full path to the temporary directory containing the generated chart if succeeded.
//
// Parameters:
// * `release` is the name of Helm release being installed
func (r *Runner) Chartify(release, dirOrChart string, opts ...ChartifyOption) (string, error) {
	u := &ChartifyOpts{}

	for i := range opts {
		if err := opts[i].SetChartifyOption(u); err != nil {
			return "", err
		}
	}

	isKustomization, err := exists(filepath.Join(dirOrChart, "kustomization.yaml"))
	if err != nil {
		return "", err
	}

	var tempDir string
	if !isKustomization {
		tempDir, err = r.copyToTempDir(dirOrChart)
		if err != nil {
			return "", err
		}
	} else {
		tempDir = mkRandomDir(os.TempDir())
	}

	isChart, err := exists(filepath.Join(tempDir, "Chart.yaml"))
	if err != nil {
		return "", err
	}

	generatedManifestFiles := []string{}

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

	if isKustomization {
		kustomOpts := &KustomizeBuildOpts{
			ValuesFiles: u.ValuesFiles,
			SetValues:   u.SetValues,
		}
		kustomizeFile, err := r.KustomizeBuild(dirOrChart, tempDir, kustomOpts)
		if err != nil {
			return "", err
		}

		generatedManifestFiles = append(generatedManifestFiles, kustomizeFile)
	}

	if !isChart && !isKustomization {
		manifestFileOptions := SearchFileOpts{
			basePath: tempDir,
			fileType: "yaml",
		}
		manifestFiles, err := r.SearchFiles(manifestFileOptions)
		if err != nil {
			return "", err
		}
		generatedManifestFiles = append(generatedManifestFiles, manifestFiles...)
	}

	var requirementsYamlContent string
	if !isChart {
		ver := u.ChartVersion
		if u.ChartVersion == "" {
			ver = "1.0.0"
			klog.Infof("using the default chart version 1.0.0 due to that no ChartVersion is specified")
		}
		chartyaml := fmt.Sprintf("name: \"%s\"\nversion: %s\nappVersion: %s\n", release, ver, ver)
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
			return "", fmt.Errorf("no helm list entry found for repository \"%s\". please `helm repo add` it!", repo)
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
		// Flatten the chart by fetching dependent chart archives and merging their K8s manifests into the temporary local chart
		// So that we can uniformly patch them with JSON patch, Strategic-Merge patch, or with injectors
		_, err := r.Run("helm", "dependency", "build", tempDir)
		if err != nil {
			return "", err
		}

		matches, err := filepath.Glob(filepath.Join(tempDir, "charts", "*-*.tgz"))
		if err != nil {
			return "", err
		}

		if isChart || len(matches) > 0 {
			templateFileOptions := SearchFileOpts{
				basePath:     tempDir,
				matchSubPath: "templates",
				fileType:     "yaml",
			}
			templateFiles, err := r.SearchFiles(templateFileOptions)
			if err != nil {
				return "", err
			}

			templateOptions := ReplaceWithRenderedOpts{
				Debug:        u.Debug,
				Namespace:    u.Namespace,
				SetValues:    u.SetValues,
				ValuesFiles:  u.ValuesFiles,
				ChartVersion: u.ChartVersion,
			}
			generated, err := r.ReplaceWithRendered(release, tempDir, templateFiles, templateOptions)
			if err != nil {
				return "", err
			}

			generatedManifestFiles = generated
		}
	}

	// We've already rendered resources from the chart and its subcharts to the helmx.1.rendered directory
	// No need to double-render them by leaving requirements.yaml/lock
	_ = os.Remove(filepath.Join(tempDir, "requirements.yaml"))
	_ = os.Remove(filepath.Join(tempDir, "requirements.lock"))

	{
		if isChart && (len(u.JsonPatches) > 0 || len(u.StrategicMergePatches) > 0) {
			patchOpts := &PatchOpts{
				JsonPatches:           u.JsonPatches,
				StrategicMergePatches: u.StrategicMergePatches,
			}
			patchedAndConcatenated, err := r.Patch(tempDir, generatedManifestFiles, patchOpts)
			if err != nil {
				return "", err
			}

			generatedManifestFiles = []string{patchedAndConcatenated}

			final := filepath.Join(tempDir, "templates", "helmx.all.yaml")
			klog.Infof("copying %s to %s", patchedAndConcatenated, final)
			if err := CopyFile(patchedAndConcatenated, final); err != nil {
				return "", err
			}
		} else {
			dsts := []string{}
			for i, f := range generatedManifestFiles {
				dst := filepath.Join(dstTemplatesDir, fmt.Sprintf("%d-%s", i, filepath.Base(f)))
				if err := os.Rename(f, dst); err != nil {
					return "", err
				}
				dsts = append(dsts, dst)
			}
			generatedManifestFiles = dsts
		}
	}

	injectOptions := InjectOpts{
		injectors: u.Injectors,
		injects:   u.Injects,
	}
	if err := r.Inject(generatedManifestFiles, injectOptions); err != nil {
		return "", err
	}

	return tempDir, nil
}
