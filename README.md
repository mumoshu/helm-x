# Helm X Plugin

No more "Kustomize vs Helm".

`helm-x` makes `helm` better integrate with vanilla Kubernetes manifests, [kustomize](https://kustomize.io/), and manual sidecar injections.

---

With `helm-x`, you can install and sidecar-inject helm charts, manifests, kustomize apps in the same way.

Installing your kustomize app as a helm chart is as easy as running:

```
$ helm x apply myapp examples/kustomize --version 1.2.3 \
  -f examples/kustomize/values.yaml
```

Then you can even run a [helm test](https://github.com/helm/helm/blob/master/docs/chart_tests.md):

```
$ helm test myapp
RUNNING: myapp-test
PASSED: myapp-test
```

Show diff before further upgrade:

```
$ helm x diff myapp examples/kustomize --version 1.2.4 \
  -f examples/kustomize/values.2.yaml
```

Check out the examples in the [examples](/examples) directory!

---

It keeps all the good things of `helm` as an extendable package manager.

That is, `helm x apply` is able to automatically remove resources that hsa gone from the desired state, without any additions like `--prune` and `--prune-whitelist` of `kubectl apply`.

Also, you can leverage useful `helm` commands like below, even though your app is written with a tool like `kustomize`:

- [`helm diff`](https://github.com/databus23/helm-diff) to diff your app
- [`helm test`](https://github.com/helm/helm/blob/master/docs/chart_tests.md) to test your app
- [`helm list`](https://helm.sh/docs/helm/#helm-list) to list all the apps running on your cluster, written with whatever tool not only `helm`.  
- [`helm package`](https://helm.sh/docs/helm/#helm-package) to package your app into a helm chart(even if it was originally written as a `kustomization`)
- [`helm s3`](https://github.com/hypnoglow/helm-s3) to stored packaged apps into AWS S3 as a chart registry(even if your app is written with `kustomize`)
- [`helm tiller`](https://github.com/rimusz/helm-tiller) to go tillerless!

If you're familiar with `helm`, what makes `helm-x` unique is it runs `helm upgrade --install` to install your apps described as:

1. kustomizations
2. directories containing manifests
3. local and remote helm charts.

## Usage

### helm x apply

Install or upgrade the helm release from the directory or the chart specified.

Under the hood, this generates Kubernetes manifests from (1)directory containing manifests/kustomization/local helm chart or (2)remote helm chart, then inject sidecars, and finally install the result as a Helm release

When DIR_OR_CHART is a local helm chart, this copies it into a temporary directory, renders all the templates into manifests by running "helm template", and then run injectors to update manifests, and install the temporary chart by running "helm upgrade --install".

It's better than installing it with "kubectl apply -f", as you can leverage various helm sub-commands like "helm test" if you included tests in the "templates/tests" directory of the chart.
It's also better in regard to security and reproducibility, as creating a helm release allows helm to detect Kubernetes resources removed from the desired state but still exist in the cluster, and automatically delete unnecessary resources.

When DIR_OR_CHART is a local directory containing Kubernetes manifests, this copies all the manifests into a temporary directory, and turns it into a local Helm chart by generating a Chart.yaml whose version and appVersion are set to the value of the --version flag.

When DIR_OR_CHART contains kustomization.yaml, this runs "kustomize build" to generate manifests, and then run injectors to update manifests, and install the temporary chart by running "helm upgrade --install".

```console
Usage:
  x apply [RELEASE] [DIR_OR_CHART] [flags]

Flags:
      --debug                                         enable verbose output
      --dry-run                                       simulate an upgrade
  -h, --help                                          help for apply
      --injector 'CMD SUBCMD,FLAG1=VAL1,FLAG2=VAL2'   injector to use (must be pre-installed) and flags to be passed in the syntax of 'CMD SUBCMD,FLAG1=VAL1,FLAG2=VAL2'. Flags should be without leading "--" (can specify multiple). "FILE" in values are replaced with the Kubernetes manifest file being injected. Example: "--injector 'istioctl kube-inject f=FILE,injectConfigFile=inject-config.yaml,meshConfigFile=mesh.config.yaml"
      --kubecontext string                            name of the kubeconfig context to use
      --namespace string                              namespace to install the release into (only used if --install is set). Defaults to the current kube config namespace
      --set stringArray                               set values on the command line (can specify multiple)
      --timeout int                                   time in seconds to wait for any individual Kubernetes operation (like Jobs for hooks) (default 300)
      --tls                                           enable TLS for request
      --tls-cert string                               path to TLS certificate file (default: $HELM_HOME/cert.pem)
      --tls-key string                                path to TLS key file (default: $HELM_HOME/key.pem)
  -f, --values stringArray                            specify values in a YAML file or a URL (can specify multiple)
      --version string                                specify the exact chart version to use. If this is not specified, the latest version is used
```

### helm x diff

Show a diff explaining what `helm x apply` would change.

Under the hood, this generates Kubernetes manifests from (1)directory containing manifests/kustomization/local helm chart or (2)remote helm chart, then inject sidecars, and finally print the resulting manifests

When DIR_OR_CHART is a local helm chart, this copies it into a temporary directory, renders all the templates into manifests by running "helm template", and then run injectors to update manifests, and prints the results.

When DIR_OR_CHART is a local directory containing Kubernetes manifests, this copies all the manifests into a temporary directory, and turns it into a local Helm chart by generating a Chart.yaml whose version and appVersion are set to the value of the --version flag.

When DIR_OR_CHART contains kustomization.yaml, this runs "kustomize build" to generate manifests, and then run injectors to update manifests, and prints the results.

```console
Usage:
  x diff [RELEASE] [DIR_OR_CHART] [flags]

Flags:
      --debug                                         enable verbose output
  -h, --help                                          help for diff
      --injector 'CMD SUBCMD,FLAG1=VAL1,FLAG2=VAL2'   injector to use (must be pre-installed) and flags to be passed in the syntax of 'CMD SUBCMD,FLAG1=VAL1,FLAG2=VAL2'. Flags should be without leading "--" (can specify multiple). "FILE" in values are replaced with the Kubernetes manifest file being injected. Example: "--injector 'istioctl kube-inject f=FILE,injectConfigFile=inject-config.yaml,meshConfigFile=mesh.config.yaml"
      --kubecontext string                            name of the kubeconfig context to use
      --namespace string                              namespace to install the release into (only used if --install is set). Defaults to the current kube config namespace
      --set stringArray                               set values on the command line (can specify multiple)
      --tls                                           enable TLS for request
      --tls-cert string                               path to TLS certificate file (default: $HELM_HOME/cert.pem)
      --tls-key string                                path to TLS key file (default: $HELM_HOME/key.pem)
  -f, --values stringArray                            specify values in a YAML file or a URL (can specify multiple)
      --version string                                specify the exact chart version to use. If this is not specified, the latest version is used
```

### helm x template

Print Kubernetes manifests that would be generated by `helm x apply`

Under the hood, this generates Kubernetes manifests from (1)directory containing manifests/kustomization/local helm chart or (2)remote helm chart, then inject sidecars, and finally print the resulting manifests

When DIR_OR_CHART is a local helm chart, this copies it into a temporary directory, renders all the templates into manifests by running "helm template", and then run injectors to update manifests, and prints the results.

When DIR_OR_CHART is a local directory containing Kubernetes manifests, this copies all the manifests into a temporary directory, and turns it into a local Helm chart by generating a Chart.yaml whose version and appVersion are set to the value of the --version flag.

When DIR_OR_CHART contains kustomization.yaml, this runs "kustomize build" to generate manifests, and then run injectors to update manifests, and prints the results.

```console
Usage:
  x template [DIR_OR_CHART] [flags]

Flags:
      --debug                                         enable verbose output
  -h, --help                                          help for template
      --injector 'CMD SUBCMD,FLAG1=VAL1,FLAG2=VAL2'   injector to use (must be pre-installed) and flags to be passed in the syntax of 'CMD SUBCMD,FLAG1=VAL1,FLAG2=VAL2'. Flags should be without leading "--" (can specify multiple). "FILE" in values are replaced with the Kubernetes manifest file being injected. Example: "--injector 'istioctl kube-inject f=FILE,injectConfigFile=inject-config.yaml,meshConfigFile=mesh.config.yaml"
      --kubecontext string                            name of the kubeconfig context to use
      --name string                                   release name (default "release-name") (default "release-name")
      --namespace string                              namespace to install the release into (only used if --install is set). Defaults to the current kube config namespace
      --set stringArray                               set values on the command line (can specify multiple)
  -f, --values stringArray                            specify values in a YAML file or a URL (can specify multiple)
      --version string                                specify the exact chart version to use. If this is not specified, the latest version is used
```

### helm x adopt

Adopt a set of existing K8s resources as if they are installed originally as a Helm chart.

```console
Adopt the existing kubernetes resources as a helm release

RESOURCES are represented as a whitespace-separated list of kind/name, like:

  configmap/foo.v1 secret/bar deployment/myapp

So that the full command looks like:

  helm x adopt myrelease configmap/foo.v1 secret/bar deployment/myapp

Usage:
  helm-x adopt [RELEASE] [RESOURCES]... [flags]

Flags:
  -h, --help                      help for adopt
      --kubecontext string        the kubeconfig context to use
      --namespace string          The namespace in which the resources to be adopted reside
      --tiller-namespace string   the tiller namespaceto use (default "kube-system")
      --tls                       enable TLS for request
      --tls-cert string           path to TLS certificate file (default: $HELM_HOME/cert.pem)
      --tls-key string            path to TLS key file (default: $HELM_HOME/key.pem)
```

## Install

```
$ helm plugin install https://github.com/mumoshu/helm-x
```

The above will fetch the latest binary release of `helm-x` and install it.

### Developer (From Source) Install

If you would like to handle the build yourself, instead of fetching a binary, this is how recommend doing it.

First, set up your environment:

- You need to have [Go](http://golang.org)  1.12 or greater installed.

Clone this repo anywhere OUSTSIDE of your `$GOPATH`:

```
$ git clone git@github.com:mumoshu/helm-x
$ make install
```

If you don't want to install it as a helm plugin, you can still run it stand-alone:

```
$ make build
$ ./helm-x template --name myapp examples/manifests/ --version 1.2.3
$ ./helm-x diff myapp examples/manifests/ --version 1.2.3 --debug
$ ./helm-x apply myapp examples/manifests/ --version 1.2.3 --debug
```

## Notes

* Not all flags present in the original `helm diff`, `helm template`, `helm upgrade` flags are implemented. If you need any other flags, please feel free to open issues and even submit pull requests.
* If you are using the `--kube-context` flag, you need to change it to `--kubecontext`, since helm plugins [drop this flag](https://github.com/helm/helm/blob/master/docs/plugins.md#a-note-on-flag-parsing).

## Prior Arts

1. [Customizing Upstream Helm Charts with Kustomize | Testing Clouds at 128bpm](https://testingclouds.wordpress.com/2018/07/20/844/)

  Relies on `helm template` to generate K8s manifests that `kustomize` can work on.
  This method implies that you can't use `helm` for release management, as also explained in the original article as follows:
  > First, Helm is no longer controlling releases of manifests into the cluster. This means that you cannot use helm rollback or helm list or any of the helm release related commands to manage your deployments.

 `helm-x`, on the other hand, solves this issue by treating the final output of `kustomize` as a temporary Helm chart, and actually `helm-install` it. 

## Acknowledgements

This project's implementation has been largely inspired from the awesome [helm-inject](https://github.com/maorfr/helm-inject) project maintained by @maorfr.
Thanks a lot for your work, @maorfr!
