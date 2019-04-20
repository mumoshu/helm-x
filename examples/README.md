## Examples

### kustomize

An example project defined with `kustomize`.

```
$ helm x apply mypap example/kustomize --version 1.2.3
```

If you want to override some params in `kustomization.yaml`:

```
$ helm x apply mypap example/kustomize --version 1.2.3 \
  -f example/kustomize/values.yaml
```

You can even add helm chart tests to your kustomize app:

```
$ helm test myapp
RUNNING: myapp-test
PASSED: myapp-test
```

> Note that the current kustomize example creates forever-crash-looping pods, because I had not put much effort.
> Please feel free to contribute a more serious, working example :)

### manifests

An example project defined with a set of vanilla Kubernetes manifest files.

```
$ helm x apply mypap example/manifests --version 1.2.3

```

### myinject

An example injector that just adds a YAML comment.

Use it like:

```
$ helm x apply mypap example/manifests --version 1.2.3 \
  --injector 'example/myinject,FILE'
```
