package helmx

import (
	"bytes"
	"fmt"
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"k8s.io/klog"
	"os"
	"path/filepath"
	"strings"
)

type PatchOpts struct {
	JsonPatches []string

	StrategicMergePatches []string
}

func (o *PatchOpts) SetPatchOption(opts *PatchOpts) error {
	*opts = *o
	return nil
}

type PatchOption interface {
	SetPatchOption(*PatchOpts) error
}

func (r *Runner) Patch(tempDir string, generatedManifestFiles []string, opts ...PatchOption) (string, error) {
	u := &PatchOpts{}

	for i := range opts {
		if err := opts[i].SetPatchOption(u); err != nil {
			return "", err
		}
	}

	klog.Infof("patching files: %v", generatedManifestFiles)

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

	renderedFile := filepath.Join(tempDir, "helmx.2.patched.yaml")
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

	return renderedFile, nil
}
