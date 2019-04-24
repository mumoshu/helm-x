package main

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/mumoshu/helm-x/pkg"
	"github.com/spf13/pflag"
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
	"github.com/spf13/cobra"
	"k8s.io/klog"

	"gopkg.in/yaml.v3"
)

var Version string

func main() {
	klog.InitFlags(nil)

	cmd := NewRootCmd()
	if err := cmd.Execute(); err != nil {
		log.Fatal("Failed to execute command")
	}
}

func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "helm-x [apply|diff|template]",
		Short:   "Turn Kubernetes manifests, Kustomization, Helm Chart into Helm release. Sidecar injection supported.",
		Long:    ``,
		Version: Version,
	}

	out := cmd.OutOrStdout()

	cmd.AddCommand(NewApplyCommand(out))
	cmd.AddCommand(NewDiffCommand(out))
	cmd.AddCommand(NewTemplateCommand(out))

	return cmd
}

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

type applyCmd struct {
	*commonOpts

	chart   string
	dryRun  bool
	timeout int

	tls     bool
	tlsCert string
	tlsKey  string

	out io.Writer
}

type templateCmd struct {
	*commonOpts

	chart string

	out io.Writer
}

type diffCmd struct {
	*commonOpts

	chart string

	tls     bool
	tlsCert string
	tlsKey  string

	out io.Writer
}

type commonOpts struct {
	debug       bool
	release     string
	valueFiles  []string
	values      []string
	namespace   string
	version     string
	kubeContext string

	injectors []string
	injects   []string
}

func chartify(dirOrChart string, u commonOpts) (string, error) {
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
			name:        u.release,
			namespace:   u.namespace,
			values:      u.values,
			valuesFiles: u.valueFiles,
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

		for _, f := range u.valueFiles {
			valsFileContent, err := ioutil.ReadFile(f)
			if err != nil {
				return "", err
			}
			if err := yaml.Unmarshal(valsFileContent, &kustomizeOpts); err != nil {
				return "", err
			}
		}

		if len(u.values) > 0 {
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
			_, err := apply.RunCommand("kustomize", args...)
			if err != nil {
				return "", err
			}
		}
		if kustomizeOpts.NamePrefix != "" {
			_, err := apply.RunCommand("kustomize", "edit", "set", "nameprefix", kustomizeOpts.NamePrefix)
			if err != nil {
				fmt.Println(err)
				return "", err
			}
		}
		if kustomizeOpts.NameSuffix != "" {
			// "--" is there to avoid `namesuffix -acme` to fail due to `-a` being considered as a flag
			_, err := apply.RunCommand("kustomize", "edit", "set", "namesuffix", "--", kustomizeOpts.NameSuffix)
			if err != nil {
				return "", err
			}
		}
		if kustomizeOpts.Namespace != "" {
			_, err := apply.RunCommand("kustomize", "edit", "set", "namespace", kustomizeOpts.Namespace)
			if err != nil {
				return "", err
			}
		}
		kustomizeFile := filepath.Join(dstTemplatesDir, "kustomized.yaml")
		out, err := apply.RunCommand("kustomize", "-o", kustomizeFile, "build", tempDir)
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

	if !isChart {
		if u.version == "" {
			return "", fmt.Errorf("--version is required when applying manifests")
		}
		chartyaml := fmt.Sprintf("name: \"%s\"\nversion: %s\nappVersion: %s\n", u.release, u.version, u.version)
		if err := ioutil.WriteFile(filepath.Join(tempDir, "Chart.yaml"), []byte(chartyaml), 0644); err != nil {
			return "", err
		}
	}
	injectOptions := injectOptions{
		injectors: u.injectors,
		injects:   u.injects,
		files:     generatedManifestFiles,
	}
	if err := inject(injectOptions); err != nil {
		return "", err
	}

	return tempDir, nil
}

// NewApplyCommand represents the apply command
func NewApplyCommand(out io.Writer) *cobra.Command {
	u := &applyCmd{out: out}

	cmd := &cobra.Command{
		Use:   "apply [RELEASE] [DIR_OR_CHART]",
		Short: "Install or upgrade the helm release from the directory or the chart specified",
		Long: `Install or upgrade the helm release from the directory or the chart specified

Under the hood, this generates Kubernetes manifests from (1)directory containing manifests/kustomization/local helm chart or (2)remote helm chart, then inject sidecars, and finally install the result as a Helm release

When DIR_OR_CHART is a local helm chart, this copies it into a temporary directory, renders all the templates into manifests by running "helm template", and then run injectors to update manifests, and install the temporary chart by running "helm upgrade --install".

It's better than installing it with "kubectl apply -f", as you can leverage various helm sub-commands like "helm test" if you included tests in the "templates/tests" directory of the chart.
It's also better in regard to security and reproducibility, as creating a helm release allows helm to detect Kubernetes resources removed from the desired state but still exist in the cluster, and automatically delete unnecessary resources.

When DIR_OR_CHART is a local directory containing Kubernetes manifests, this copies all the manifests into a temporary directory, and turns it into a local Helm chart by generating a Chart.yaml whose version and appVersion are set to the value of the --version flag.

When DIR_OR_CHART contains kustomization.yaml, this runs "kustomize build" to generate manifests, and then run injectors to update manifests, and install the temporary chart by running "helm upgrade --install".
`,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 2 {
				return errors.New("requires two arguments")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			release := args[0]
			dir := args[1]

			u.release = release
			tempDir, err := chartify(dir, *u.commonOpts)
			if err != nil {
				cmd.SilenceUsage = true
				return err
			}

			if !u.debug {
				defer os.RemoveAll(tempDir)
			} else {
				klog.Infof("helm chart has been written to %s for you to see. please remove it afterwards", tempDir)
			}

			upgradeOptions := upgradeOptions{
				chart:       tempDir,
				name:        release,
				values:      u.values,
				valuesFiles: u.valueFiles,
				namespace:   u.namespace,
				kubeContext: u.kubeContext,
				timeout:     u.timeout,
				dryRun:      u.dryRun,
				debug:       u.debug,
				tls:         u.tls,
				tlsCert:     u.tlsCert,
				tlsKey:      u.tlsKey,
			}
			if err := upgrade(upgradeOptions); err != nil {
				cmd.SilenceUsage = true
				return err
			}

			return nil
		},
	}
	f := cmd.Flags()

	u.commonOpts = commonFlags(f)

	//f.StringVar(&u.release, "name", "", "release name (default \"release-name\")")
	f.IntVar(&u.timeout, "timeout", 300, "time in seconds to wait for any individual Kubernetes operation (like Jobs for hooks)")

	f.BoolVar(&u.dryRun, "dry-run", false, "simulate an upgrade")

	f.BoolVar(&u.tls, "tls", false, "enable TLS for request")
	f.StringVar(&u.tlsCert, "tls-cert", "", "path to TLS certificate file (default: $HELM_HOME/cert.pem)")
	f.StringVar(&u.tlsKey, "tls-key", "", "path to TLS key file (default: $HELM_HOME/key.pem)")

	return cmd
}

// NewTemplateCommand represents the template command
func NewTemplateCommand(out io.Writer) *cobra.Command {
	u := &templateCmd{out: out}

	cmd := &cobra.Command{
		Use:   "template [DIR_OR_CHART]",
		Short: "Print Kubernetes manifests that would be generated by `helm x apply`",
		Long: `Print Kubernetes manifests that would be generated by ` + "`helm x apply`" + `

Under the hood, this generates Kubernetes manifests from (1)directory containing manifests/kustomization/local helm chart or (2)remote helm chart, then inject sidecars, and finally print the resulting manifests

When DIR_OR_CHART is a local helm chart, this copies it into a temporary directory, renders all the templates into manifests by running "helm template", and then run injectors to update manifests, and prints the results.

When DIR_OR_CHART is a local directory containing Kubernetes manifests, this copies all the manifests into a temporary directory, and turns it into a local Helm chart by generating a Chart.yaml whose version and appVersion are set to the value of the --version flag.

When DIR_OR_CHART contains kustomization.yaml, this runs "kustomize build" to generate manifests, and then run injectors to update manifests, and prints the results.
`,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("requires one argument")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := args[0]

			tempDir, err := chartify(dir, *u.commonOpts)
			if err != nil {
				cmd.SilenceUsage = true
				return err
			}

			if !u.debug {
				klog.Infof("helm chart has been written to %s for you to see. please remove it afterwards", tempDir)
				defer os.RemoveAll(tempDir)
			}

			opts := runTemplateOptions{
				chart:       tempDir,
				name:        u.release,
				values:      u.values,
				valuesFiles: u.valueFiles,
				namespace:   u.namespace,
				kubeContext: u.kubeContext,
				debug:       u.debug,
			}
			if err := runTemplate(opts); err != nil {
				cmd.SilenceUsage = true
				return err
			}

			return nil
		},
	}
	f := cmd.Flags()

	u.commonOpts = commonFlags(f)

	f.StringVar(&u.release, "name", "release-name", "release name (default \"release-name\")")

	return cmd
}

// NewDiffCommand represents the diff command
func NewDiffCommand(out io.Writer) *cobra.Command {
	u := &diffCmd{out: out}

	cmd := &cobra.Command{
		Use:   "diff [RELEASE] [DIR_OR_CHART]",
		Short: "Show a diff explaining what `helm x apply` would change",
		Long: `Show a diff explaining what ` + "`helm x apply`" + ` would change.

Under the hood, this generates Kubernetes manifests from (1)directory containing manifests/kustomization/local helm chart or (2)remote helm chart, then inject sidecars, and finally print the resulting manifests

When DIR_OR_CHART is a local helm chart, this copies it into a temporary directory, renders all the templates into manifests by running "helm template", and then run injectors to update manifests, and prints the results.

When DIR_OR_CHART is a local directory containing Kubernetes manifests, this copies all the manifests into a temporary directory, and turns it into a local Helm chart by generating a Chart.yaml whose version and appVersion are set to the value of the --version flag.

When DIR_OR_CHART contains kustomization.yaml, this runs "kustomize build" to generate manifests, and then run injectors to update manifests, and prints the results.
`,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 2 {
				return errors.New("requires two arguments")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			release := args[0]
			dir := args[1]

			u.release = release
			tempDir, err := chartify(dir, *u.commonOpts)
			if err != nil {
				cmd.SilenceUsage = true
				return err
			}

			if !u.debug {
				klog.Infof("helm chart has been written to %s for you to see. please remove it afterwards", tempDir)
				defer os.RemoveAll(tempDir)
			}

			diffOptions := diffOptions{
				chart:       tempDir,
				name:        release,
				values:      u.values,
				valuesFiles: u.valueFiles,
				namespace:   u.namespace,
				kubeContext: u.kubeContext,
				tls:         u.tls,
				tlsCert:     u.tlsCert,
				tlsKey:      u.tlsKey,
			}
			if err := diff(diffOptions); err != nil {
				cmd.SilenceUsage = true
				return err
			}

			return nil
		},
	}
	f := cmd.Flags()

	u.commonOpts = commonFlags(f)

	//f.StringVar(&u.release, "name", "", "release name (default \"release-name\")")

	f.BoolVar(&u.tls, "tls", false, "enable TLS for request")
	f.StringVar(&u.tlsCert, "tls-cert", "", "path to TLS certificate file (default: $HELM_HOME/cert.pem)")
	f.StringVar(&u.tlsKey, "tls-key", "", "path to TLS key file (default: $HELM_HOME/key.pem)")

	return cmd
}

func commonFlags(f *pflag.FlagSet) *commonOpts {
	u := &commonOpts{}

	f.StringArrayVar(&u.injectors, "injector", []string{}, "DEPRECATED: Use `--inject \"CMD ARG1 ARG2\"` instead. injector to use (must be pre-installed) and flags to be passed in the syntax of `'CMD SUBCMD,FLAG1=VAL1,FLAG2=VAL2'`. Flags should be without leading \"--\" (can specify multiple). \"FILE\" in values are replaced with the Kubernetes manifest file being injected. Example: \"--injector 'istioctl kube-inject f=FILE,injectConfigFile=inject-config.yaml,meshConfigFile=mesh.config.yaml\"")
	f.StringArrayVar(&u.injects, "inject", []string{}, "injector to use (must be pre-installed) and flags to be passed in the syntax of `'istioctl kube-inject -f FILE'`. \"FILE\" is replaced with the Kubernetes manifest file being injected")

	f.StringArrayVarP(&u.valueFiles, "values", "f", []string{}, "specify values in a YAML file or a URL (can specify multiple)")
	f.StringArrayVar(&u.values, "set", []string{}, "set values on the command line (can specify multiple)")
	f.StringVar(&u.namespace, "namespace", "", "namespace to install the release into (only used if --install is set). Defaults to the current kube config namespace")
	f.StringVar(&u.version, "version", "", "specify the exact chart version to use. If this is not specified, the latest version is used")
	f.StringVar(&u.kubeContext, "kubecontext", "", "name of the kubeconfig context to use")

	f.BoolVar(&u.debug, "debug", false, "enable verbose output")

	return u
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
		tempDir = filepath.Join(tempDir, files[0].Name())
	} else {
		err = copy.Copy(path, tempDir)
		if err != nil {
			return "", err
		}
	}
	return tempDir, nil
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
		additionalFlags += createFlagChain("namespace", []string{o.namespace})
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

type injectOptions struct {
	injectors []string
	injects   []string
	files     []string
}

func inject(o injectOptions) error {
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

type upgradeOptions struct {
	chart       string
	name        string
	values      []string
	valuesFiles []string
	namespace   string
	kubeContext string
	timeout     int
	install     bool
	dryRun      bool
	debug       bool
	tls         bool
	tlsCert     string
	tlsKey      string
	kubeConfig  string
}

func upgrade(o upgradeOptions) error {
	var additionalFlags string
	additionalFlags += createFlagChain("set", o.values)
	additionalFlags += createFlagChain("f", o.valuesFiles)
	additionalFlags += createFlagChain("timeout", []string{fmt.Sprintf("%d", o.timeout)})
	additionalFlags += createFlagChain("install", []string{""})
	if o.namespace != "" {
		additionalFlags += createFlagChain("namespace", []string{o.namespace})
	}
	if o.kubeContext != "" {
		additionalFlags += createFlagChain("kube-context", []string{o.kubeContext})
	}
	if o.dryRun {
		additionalFlags += createFlagChain("dry-run", []string{""})
	}
	if o.debug {
		additionalFlags += createFlagChain("debug", []string{""})
	}
	if o.tls {
		additionalFlags += createFlagChain("tls", []string{""})
	}
	if o.tlsCert != "" {
		additionalFlags += createFlagChain("tls-cert", []string{o.tlsCert})
	}
	if o.tlsKey != "" {
		additionalFlags += createFlagChain("tls-key", []string{o.tlsKey})
	}

	command := fmt.Sprintf("helm upgrade %s %s%s", o.name, o.chart, additionalFlags)
	stdout, stderr, err := Capture(command)
	if err != nil || len(stderr) != 0 {
		return fmt.Errorf(string(stderr))
	}
	fmt.Println(string(stdout))

	return nil
}

type runTemplateOptions struct {
	chart       string
	name        string
	values      []string
	valuesFiles []string
	namespace   string
	kubeContext string
	timeout     int
	install     bool
	dryRun      bool
	debug       bool
	tls         bool
	tlsCert     string
	tlsKey      string
	kubeConfig  string
}

func runTemplate(o runTemplateOptions) error {
	var additionalFlags string
	additionalFlags += createFlagChain("set", o.values)
	additionalFlags += createFlagChain("f", o.valuesFiles)
	if o.namespace != "" {
		additionalFlags += createFlagChain("namespace", []string{o.namespace})
	}
	if o.kubeContext != "" {
		additionalFlags += createFlagChain("kube-context", []string{o.kubeContext})
	}
	if o.name != "" {
		additionalFlags += createFlagChain("name", []string{o.name})
	}
	if o.debug {
		additionalFlags += createFlagChain("debug", []string{""})
	}
	if o.tls {
		additionalFlags += createFlagChain("tls", []string{""})
	}
	if o.tlsCert != "" {
		additionalFlags += createFlagChain("tls-cert", []string{o.tlsCert})
	}
	if o.tlsKey != "" {
		additionalFlags += createFlagChain("tls-key", []string{o.tlsKey})
	}

	command := fmt.Sprintf("helm template %s%s", o.chart, additionalFlags)
	stdout, stderr, err := Capture(command)
	if err != nil || len(stderr) != 0 {
		return fmt.Errorf(string(stderr))
	}
	fmt.Println(string(stdout))

	return nil
}

type diffOptions struct {
	chart       string
	name        string
	values      []string
	valuesFiles []string
	namespace   string
	kubeContext string
	tls         bool
	tlsCert     string
	tlsKey      string
	kubeConfig  string
}

func diff(o diffOptions) error {
	var additionalFlags string
	additionalFlags += createFlagChain("set", o.values)
	additionalFlags += createFlagChain("f", o.valuesFiles)
	additionalFlags += createFlagChain("allow-unreleased", []string{""})
	additionalFlags += createFlagChain("detailed-exitcode", []string{""})
	additionalFlags += createFlagChain("context", []string{"3"})
	additionalFlags += createFlagChain("reset-values", []string{""})
	additionalFlags += createFlagChain("suppress-secrets", []string{""})
	if o.namespace != "" {
		additionalFlags += createFlagChain("namespace", []string{o.namespace})
	}
	if o.kubeContext != "" {
		additionalFlags += createFlagChain("kube-context", []string{o.kubeContext})
	}
	if o.tls {
		additionalFlags += createFlagChain("tls", []string{""})
	}
	if o.tlsCert != "" {
		additionalFlags += createFlagChain("tls-cert", []string{o.tlsCert})
	}
	if o.tlsKey != "" {
		additionalFlags += createFlagChain("tls-key", []string{o.tlsKey})
	}

	command := fmt.Sprintf("helm diff upgrade %s %s%s", o.name, o.chart, additionalFlags)
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
