package helmx

import (
	"k8s.io/klog"
	"os"
	"os/exec"
	"strings"
)

type KustomizeOpts struct {
	Images     []KustomizeImage `yaml:"images"`
	NamePrefix string           `yaml:"namePrefix"`
	NameSuffix string           `yaml:"nameSuffix"`
	Namespace  string           `yaml:"namespace"`
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

type ClientOpts struct {
	KubeContext string
	TLS         bool
	TLSCert     string
	TLSKey      string

	TillerStorageBackend string
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

// DeprecatedExec takes a command as a string and executes it
func (r *Runner) DeprecatedExec(cmd string) error {
	klog.Infof("running %s", cmd)
	args := strings.Split(cmd, " ")
	binary := args[0]
	_, err := exec.LookPath(binary)
	if err != nil {
		return err
	}

	return r.commander.RunCommand(binary, args[1:], os.Stdout, os.Stderr)
}

// DeprecatedCaptureBytes takes a command as a string and executes it, and returns the captured stdout and stderr
func (r *Runner) DeprecatedCaptureBytes(cmd string) ([]byte, []byte, error) {
	args := strings.Split(cmd, " ")
	binary := args[0]
	_, err := exec.LookPath(binary)
	if err != nil {
		return nil, nil, err
	}

	return r.CaptureBytes(binary, args[1:])
}

func (r *Runner) CaptureBytes(binary string, args []string) ([]byte, []byte, error) {
	return r.commander.CaptureBytes(binary, args)
}
