package helmx

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type KustomizeBuildOpts struct {
	ValuesFiles []string
	SetValues   []string
}

func (o *KustomizeBuildOpts) SetKustomizeBuildOption(opts *KustomizeBuildOpts) error {
	*opts = *o
	return nil
}

type KustomizeBuildOption interface {
	SetKustomizeBuildOption(opts *KustomizeBuildOpts) error
}

func (r *Runner) KustomizeBuild(srcDir string, tempDir string, opts ...KustomizeBuildOption) (string, error) {
	kustomizeOpts := KustomizeOpts{}
	u := &KustomizeBuildOpts{}

	for i := range opts {
		if err := opts[i].SetKustomizeBuildOption(u); err != nil {
			return "", err
		}
	}

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

	evaluatedPath, err := filepath.EvalSymlinks(tempDir)
	if err != nil {
		return "", err
	}
	relPath, err := filepath.Rel(evaluatedPath, path.Join(prevDir, srcDir))
	if err != nil {
		return "", err
	}
	baseFile := []byte("bases:\n- " + relPath + "\n")
	if err := ioutil.WriteFile(path.Join(tempDir, "kustomization.yaml"), baseFile, 0644); err != nil {
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
	kustomizeFile := filepath.Join(tempDir, "kustomized.yaml")
	out, err := r.Run("kustomize", "-o", kustomizeFile, "build", "--load_restrictor=none", tempDir)
	if err != nil {
		return "", err
	}
	fmt.Println(out)

	return kustomizeFile, nil
}
