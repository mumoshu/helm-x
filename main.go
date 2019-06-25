package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"

	"github.com/mumoshu/helm-x/pkg/helmx"
	"github.com/mumoshu/helm-x/pkg/releasetool"

	"gopkg.in/yaml.v3"
)

var Version string

var CommandName = "helm-x"

func main() {
	cmd := NewRootCmd()
	cmd.SilenceErrors = true

	// See https://flowerinthenight.com/blog/2019/02/05/golang-cobra-klog
	fs := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	// Suppress usage flag.ErrHelp
	fs.SetOutput(ioutil.Discard)
	klog.InitFlags(fs)
	args := append([]string{}, os.Args[1:]...)
	verbosityFromEnv := os.Getenv("HELM_X_VERBOSITY")
	if verbosityFromEnv != "" {
		// -v LEVEL must preceed the remaining args to be parsed by fs
		args = append([]string{"-v", verbosityFromEnv}, args...)
	}
	if err := fs.Parse(args); err != nil && err != flag.ErrHelp {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	remainings := fs.Args()
	pflag.CommandLine.AddGoFlagSet(fs)

	if err := cmd.Execute(); err != nil {
		helmFallback(remainings, err)
		klog.Fatalf("%v", err)
	}
}

func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     fmt.Sprintf("%s [apply|diff|template|dump|adopt]", CommandName),
		Short:   "Turn Kubernetes manifests, Kustomization, Helm Chart into Helm release. Sidecar injection supported.",
		Long:    ``,
		Version: Version,
	}

	out := cmd.OutOrStdout()

	cmd.AddCommand(NewApplyCommand(out, "apply", true))
	cmd.AddCommand(NewApplyCommand(out, "upgrade", false))
	cmd.AddCommand(NewDiffCommand(out))
	cmd.AddCommand(NewTemplateCommand(out))
	cmd.AddCommand(NewUtilDumpRelease(out))
	cmd.AddCommand(NewAdopt(out))

	return cmd
}

type dumpCmd struct {
	*helmx.ClientOpts

	TillerNamespace string

	Out io.Writer
}

// NewApplyCommand represents the apply command
func NewApplyCommand(out io.Writer, cmdName string, installByDefault bool) *cobra.Command {
	upOpts := &helmx.UpgradeOpts{Out: out}
	pathOptions := clientcmd.NewDefaultPathOptions()

	cmd := &cobra.Command{
		Use:   fmt.Sprintf("%s [RELEASE] [DIR_OR_CHART]", cmdName),
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

			tempLocalChartDir, err := helmx.New().Chartify(release, dir, upOpts.ChartifyOpts)
			if err != nil {
				cmd.SilenceUsage = true
				return err
			}

			if !upOpts.Debug {
				defer os.RemoveAll(tempLocalChartDir)
			} else {
				klog.Infof("helm chart has been written to %s for you to see. please remove it afterwards", tempLocalChartDir)
			}

			if len(upOpts.Adopt) > 0 {
				if err := helmx.New().Adopt(
					release,
					upOpts.Adopt,
					pathOptions,
					helmx.TillerNamespace(upOpts.TillerNamespace),
					helmx.Namespace(upOpts.Namespace),
				); err != nil {
					return err
				}
			}

			if err := helmx.New().Upgrade(release, tempLocalChartDir, *upOpts); err != nil {
				cmd.SilenceUsage = true
				return err
			}

			return nil
		},
	}
	f := cmd.Flags()

	upOpts.ChartifyOpts = chartifyOptsFromFlags(f)
	upOpts.ClientOpts = clientOptsFromFlags(f)

	//f.StringVar(&u.release, "name", "", "release name (default \"release-name\")")
	f.IntVar(&upOpts.Timeout, "timeout", 300, "time in seconds to wait for any individual Kubernetes operation (like Jobs for hooks)")

	f.BoolVar(&upOpts.DryRun, "dry-run", false, "simulate an upgrade")

	f.BoolVar(&upOpts.Install, "install", installByDefault, "install the release if missing")

	f.BoolVar(&upOpts.ResetValues, "reset-values", false, "reset the values to the ones built into the chart and merge in any new values")

	f.StringSliceVarP(&upOpts.Adopt, "adopt", "", []string{}, "adopt existing k8s resources before apply")

	f.StringVar(&pathOptions.LoadingRules.ExplicitPath, pathOptions.ExplicitFileFlag, pathOptions.LoadingRules.ExplicitPath, "use a particular kubeconfig file")

	return cmd
}

// NewTemplateCommand represents the template command
func NewTemplateCommand(out io.Writer) *cobra.Command {
	templateOpts := &helmx.RenderOpts{Out: out}

	var release string

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

			tempLocalChartDir, err := helmx.New().Chartify(release, dir, templateOpts.ChartifyOpts)
			if err != nil {
				cmd.SilenceUsage = true
				return err
			}

			if !templateOpts.Debug {
				defer os.RemoveAll(tempLocalChartDir)
			} else {
				klog.Infof("helm chart has been written to %s for you to see. please remove it afterwards", tempLocalChartDir)
			}

			if err := helmx.New().Render(release, tempLocalChartDir, *templateOpts); err != nil {
				cmd.SilenceUsage = true
				return err
			}

			return nil
		},
	}
	f := cmd.Flags()

	templateOpts.ChartifyOpts = chartifyOptsFromFlags(f)

	f.StringVar(&release, "name", "release-name", "release name (default \"release-name\")")
	f.BoolVar(&templateOpts.IncludeReleaseConfigmap, "include-release-configmap", false, "turn the result into a proper helm release, by removing hooks from the manifest, and including a helm release configmap/secret that should otherwise created by \"helm [upgrade|install]\"")
	f.BoolVar(&templateOpts.IncludeReleaseSecret, "include-release-secret", false, "turn the result into a proper helm release, by removing hooks from the manifest, and including a helm release configmap/secret that should otherwise created by \"helm [upgrade|install]\"")

	return cmd
}

// NewDiffCommand represents the diff command
func NewDiffCommand(out io.Writer) *cobra.Command {
	diff := newDiffCommand("diff", out)
	upgrade := newDiffCommand("upgrade", out)
	diff.AddCommand(upgrade)
	return diff
}

func newDiffCommand(use string, out io.Writer) *cobra.Command {
	diffOpts := &helmx.DiffOpts{Out: out}

	cmd := &cobra.Command{
		Use:   fmt.Sprintf("%s [RELEASE] [DIR_OR_CHART]", use),
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

			tempDir, err := helmx.New().Chartify(release, dir, diffOpts.ChartifyOpts)
			if err != nil {
				cmd.SilenceUsage = true
				return err
			}

			if diffOpts.Debug {
				klog.Infof("helm chart has been written to %s for you to see. please remove it afterwards", tempDir)
			} else {
				defer os.RemoveAll(tempDir)
			}

			changed, err := helmx.New().Diff(release, tempDir, diffOpts)
			if err != nil {
				cmd.SilenceUsage = true
				return err
			}
			if changed {
				os.Exit(2)
			}

			return nil
		},
	}
	f := cmd.Flags()

	diffOpts.ChartifyOpts = chartifyOptsFromFlags(f)
	diffOpts.ClientOpts = clientOptsFromFlags(f)

	f.BoolVar(&diffOpts.AllowUnreleased, "allow-unreleased", false, "enables diffing of releases that are not yet deployed via Helm")
	f.BoolVar(&diffOpts.DetailedExitcode, "detailed-exitcode", false, "return a non-zero exit code when there are changes")
	f.BoolVar(&diffOpts.ResetValues, "reset-values", false, "reset the values to the ones built into the chart and merge in any new values")

	//f.StringVar(&u.release, "name", "", "release name (default \"release-name\")")

	return cmd
}

// NewAdopt represents the adopt command
func NewAdopt(out io.Writer) *cobra.Command {
	adoptOpts := &helmx.AdoptOpts{Out: out}
	pathOptions := clientcmd.NewDefaultPathOptions()

	cmd := &cobra.Command{
		Use: "adopt [RELEASE] [RESOURCES]...",
		Short: `Adopt the existing kubernetes resources as a helm release

RESOURCES are represented as a whitespace-separated list of kind/name, like:

  configmap/foo.v1 secret/bar deployment/myapp

So that the full command looks like:

  helm x adopt myrelease configmap/foo.v1 secret/bar deployment/myapp
`,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return errors.New("requires at least two argument")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			release := args[0]
			resources := args[1:]

			return helmx.New().Adopt(
				release,
				resources,
				pathOptions,
				helmx.TillerNamespace(adoptOpts.TillerNamespace),
				helmx.Namespace(adoptOpts.Namespace),
				helmx.TillerStorageBackend(adoptOpts.TillerStorageBackend),
			)
		},
	}
	f := cmd.Flags()

	adoptOpts.ClientOpts = clientOptsFromFlags(f)

	f.StringVar(&adoptOpts.Namespace, "namespace", "", "The Namespace in which the resources to be adopted reside")

	f.StringVar(&pathOptions.LoadingRules.ExplicitPath, pathOptions.ExplicitFileFlag, pathOptions.LoadingRules.ExplicitPath, "use a particular kubeconfig file")

	return cmd
}

// NewDiffCommand represents the diff command
func NewUtilDumpRelease(out io.Writer) *cobra.Command {
	dumpOpts := &dumpCmd{Out: out}

	cmd := &cobra.Command{
		Use:   "dump [RELEASE]",
		Short: "Dump the release object for developing purpose",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("requires one argument")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			release := args[0]
			storage, err := releasetool.New(dumpOpts.TillerNamespace, releasetool.Opts{StorageBackend: dumpOpts.TillerStorageBackend})
			if err != nil {
				return err
			}

			r, err := storage.GetDeployedRelease(release)
			if err != nil {
				return err
			}

			jsonBytes, err := json.Marshal(r)

			jsonObj := map[string]interface{}{}
			if err := json.Unmarshal(jsonBytes, &jsonObj); err != nil {
				return err
			}

			yamlBytes, err := yaml.Marshal(jsonObj)
			if err != nil {
				return err
			}

			fmt.Printf("%s\n", string(yamlBytes))

			fmt.Printf("manifest:\n%s", jsonObj["manifest"])

			return nil
		},
	}
	f := cmd.Flags()

	dumpOpts.ClientOpts = clientOptsFromFlags(f)

	return cmd
}

func chartifyOptsFromFlags(f *pflag.FlagSet) *helmx.ChartifyOpts {
	chartifyOpts := &helmx.ChartifyOpts{}

	f.StringArrayVar(&chartifyOpts.Injectors, "injector", []string{}, "DEPRECATED: Use `--inject \"CMD ARG1 ARG2\"` instead. injector to use (must be pre-installed) and flags to be passed in the syntax of `'CMD SUBCMD,FLAG1=VAL1,FLAG2=VAL2'`. Flags should be without leading \"--\" (can specify multiple). \"FILE\" in values are replaced with the Kubernetes manifest file being injected. Example: \"--injector 'istioctl kube-inject f=FILE,injectConfigFile=inject-config.yaml,meshConfigFile=mesh.config.yaml\"")
	f.StringArrayVar(&chartifyOpts.Injects, "inject", []string{}, "injector to use (must be pre-installed) and flags to be passed in the syntax of `'istioctl kube-inject -f FILE'`. \"FILE\" is replaced with the Kubernetes manifest file being injected")
	f.StringArrayVar(&chartifyOpts.AdhocChartDependencies, "dependency", []string{}, "Adhoc dependencies to be added to the temporary local helm chart being installed. Syntax: ALIAS=REPO/CHART:VERSION e.g. mydb=stable/mysql:1.2.3")
	f.StringArrayVar(&chartifyOpts.JsonPatches, "json-patch", []string{}, "Kustomize JSON Patch file to be applied to the rendered K8s manifests. Allows customizing your chart without forking or updating")
	f.StringArrayVar(&chartifyOpts.StrategicMergePatches, "strategic-merge-patch", []string{}, "Kustomize Strategic Merge Patch file to be applied to the rendered K8s manifests. Allows customizing your chart without forking or updating")

	f.StringArrayVarP(&chartifyOpts.ValuesFiles, "values", "f", []string{}, "specify values in a YAML file or a URL (can specify multiple)")
	f.StringArrayVar(&chartifyOpts.SetValues, "set", []string{}, "set values on the command line (can specify multiple)")
	f.StringVar(&chartifyOpts.Namespace, "namespace", "", "Namespace to install the release into (only used if --install is set). Defaults to the current kube config Namespace")
	f.StringVar(&chartifyOpts.TillerNamespace, "tiller-namespace", "kube-system", "Namespace to in which release configmap/secret objects reside")
	f.StringVar(&chartifyOpts.ChartVersion, "version", "", "specify the exact chart version to use. If this is not specified, the latest version is used")

	f.BoolVar(&chartifyOpts.Debug, "debug", os.Getenv("HELM_X_DEBUG") == "on", "enable verbose output")
	f.BoolVar(&chartifyOpts.EnableKustomizeAlphaPlugins, "enable_alpha_plugins", false, "Enable the use of kustomize plugins")

	return chartifyOpts
}

func clientOptsFromFlags(f *pflag.FlagSet) *helmx.ClientOpts {
	clientOpts := &helmx.ClientOpts{}
	f.BoolVar(&clientOpts.TLS, "tls", false, "enable TLS for request")
	f.StringVar(&clientOpts.TLSCert, "tls-cert", "", "path to TLS certificate file (default: $HELM_HOME/cert.pem)")
	f.StringVar(&clientOpts.TLSKey, "tls-key", "", "path to TLS key file (default: $HELM_HOME/key.pem)")
	f.StringVar(&clientOpts.KubeContext, "kubecontext", "", "the kubeconfig context to use")
	f.StringVar(&clientOpts.TillerStorageBackend, "tiller-storage-backend", "configmaps", "the tiller storage backend to use. either `configmaps` or `secrets` are supported. See the upstream doc for more context: https://helm.sh/docs/install/#storage-backends")
	return clientOpts
}
