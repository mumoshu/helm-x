package helmx

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/otiai10/copy"
	"k8s.io/klog"
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

type TemplateOpts struct {
	*ChartifyOpts

	IncludeReleaseConfigmap bool
	IncludeReleaseSecret    bool

	Out io.Writer
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

// copyToTempDir checks if the path is local or a repo (in this order) and copies it to a temp directory
// It will perform a `helm fetch` if required
func (r *Runner) copyToTempDir(path string) (string, error) {
	tempDir := mkRandomDir(os.TempDir())
	exists, err := exists(path)
	if err != nil {
		return "", err
	}
	if !exists {
		return r.fetchAndUntarUnderDir(path, tempDir)
	}
	err = copy.Copy(path, tempDir)
	if err != nil {
		return "", err
	}
	return tempDir, nil
}

func (r *Runner) fetchAndUntarUnderDir(path, tempDir string) (string, error) {
	command := fmt.Sprintf("helm fetch %s --untar -d %s", path, tempDir)
	_, stderr, err := r.DeprecatedCapture(command)
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

func (r *Runner) untarUnderDir(path, tempDir string) (string, error) {
	command := fmt.Sprintf("tar -zxvf %s -C %s", path, tempDir)
	_, stderr, err := r.DeprecatedCapture(command)
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

func (r *Runner) template(o templateOptions) error {
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
		additionalFlags += createFlagChain("namespace", []string{o.namespace})
	}

	for _, file := range o.files {
		command := fmt.Sprintf("helm template --debug=false %s --name %s -x %s%s", o.chart, o.name, file, additionalFlags)
		stdout, stderr, err := r.DeprecatedCapture(command)
		if err != nil || len(stderr) != 0 {
			return fmt.Errorf(string(stderr))
		}
		if err := ioutil.WriteFile(file, stdout, 0644); err != nil {
			return err
		}
	}

	return nil
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

	return r.commander.run(binary, args[1:], os.Stdout, os.Stderr)
}

// DeprecatedCapture takes a command as a string and executes it, and returns the captured stdout and stderr
func (r *Runner) DeprecatedCapture(cmd string) ([]byte, []byte, error) {
	args := strings.Split(cmd, " ")
	binary := args[0]
	_, err := exec.LookPath(binary)
	if err != nil {
		return nil, nil, err
	}

	return r.Capture(binary, args[1:])
}

func (r *Runner) Capture(binary string, args []string) ([]byte, []byte, error) {
	klog.Infof("running %s %s", binary, strings.Join(args, " "))
	_, err := exec.LookPath(binary)
	if err != nil {
		return nil, nil, err
	}

	var stdout, stderr bytes.Buffer
	err = r.commander.run(binary, args, &stdout, &stderr)
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