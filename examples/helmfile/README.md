# Integrating helm-x with helmfile

[helmfile](https://github.com/roboll/helmfile) is a kind of Infrastructure as Code tool for Kubernetes. It takes a declarative spec file called "state file"(`helmfile.yaml`) to reconcile your K8s cluster to the desired state declared in it.

Have you ever dreamed of a Terraform-like tool for K8s, but more K8s specific features built-in? `helmfile` is!

`helmfile` is only capable of managing the cluster state as the set of Helm releases, in other words a set of Helm charts coupled with the chart values.

Integrating `helm-x` with `helmfile` allows you to manage not only Helm releases but also Kustomizations and vanilla K8s manifest directories. Theoretically it can be enhanced to support any other K8s deployment tool that is capable of generating K8s manifests!

## TL;DR;

Run `helmfile` with `--helm-binary` pointed to the `helm-x` binary:

```
$ helmfile --helm-binary ~/.helm/plugins/helm-x/bin/helm-x --log-level debug apply
```

## Rationale

Comparing `kubectl`, `kustomize`, and `helm` is like comparing apples and oranges. `kubectl` is used to apply vanilla K8s manifests. `kustomize` is used to build K8s manifests.

In contrast, `helm` is used to generate K8s manifests from helm charts/templates AND for test automation(`helm test`), reviewing change sets(`helm diff`), release management(`helm list`, `helm status`, `helm rollback`, `helm get values`, ...).

From the developer's perspective, `kustomize` is ability to compose complex K8s manifests without a pile of YAML and go templates is appealing. But does it justify throwing away all the other things `helm` has already solved for us? Why we can't have both?

`helm-x` is the answer. I blurs that border by enhancing `helm` to treat K8s manifests and Kustomizations as Helm charts. For example, after `helm x install`ing one, any release-related helm commands can be run on it. You can even add `templates/tests` directory containing helm tests even under your K8s manifests directory or your project, so that you can run `helm test` on the Helm release created from a manifests directory or Kustomize project!

The integration of `helm-x` with `helmfile` advances it further. A well-known downside of `helm` is it's lack of declarativity. `helmfile` allows you to write state files called `helmfile.yaml` for your Helm releases. `helm-x` and `helmfile` together allows you to write state files for any combo of project types, including local and remote helm charts, k8s manifests directory, kustomize projects.
