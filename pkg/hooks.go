package x

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"k8s.io/helm/pkg/hooks"
	"k8s.io/helm/pkg/proto/hapi/release"
	"strings"
)

// See https://github.com/helm/helm/blob/29ab7a0a775ec7182be88a1b6daa9e65a472b46b/pkg/tiller/hooks.go#L35
var events = map[string]release.Hook_Event{
	hooks.PreInstall:         release.Hook_PRE_INSTALL,
	hooks.PostInstall:        release.Hook_POST_INSTALL,
	hooks.PreDelete:          release.Hook_PRE_DELETE,
	hooks.PostDelete:         release.Hook_POST_DELETE,
	hooks.PreUpgrade:         release.Hook_PRE_UPGRADE,
	hooks.PostUpgrade:        release.Hook_POST_UPGRADE,
	hooks.PreRollback:        release.Hook_PRE_ROLLBACK,
	hooks.PostRollback:       release.Hook_POST_ROLLBACK,
	hooks.ReleaseTestSuccess: release.Hook_RELEASE_TEST_SUCCESS,
	hooks.ReleaseTestFailure: release.Hook_RELEASE_TEST_FAILURE,
	hooks.CRDInstall:         release.Hook_CRD_INSTALL,
}

type metadata struct {
	Name        string            `yaml:"name"`
	Annotations map[string]string `yaml:"annotations"`
}

type resource struct {
	Kind     string   `yaml:"kind"`
	Metadata metadata `yaml:"metadata"`
}

func SplitManifestAndHooks(manifest string) (string, []*release.Hook, error) {
	manifests := strings.Split(manifest, "\n---\n")

	resources := ""

	result := []*release.Hook{}

	for _, m := range manifests {
		m = strings.TrimPrefix(m, "---\n")

		lines := strings.Split(m, "\n")
		header := lines[0]

		items := strings.Split(header, "Source: ")

		if len(items) != 2 {
			return "", nil, fmt.Errorf("unexpected format of manifest: missing Source line:\n%s", m)
		}

		source := items[1]

		r := resource{Metadata: metadata{}}
		if err := yaml.Unmarshal([]byte(m), &r); err != nil {
			return "", nil, err
		}
		// See https://github.com/helm/helm/blob/2b36b1ad46278380aa70b8c190c346ce50e8dc96/pkg/hooks/hooks.go#L27
		hook := r.Metadata.Annotations[hooks.HookAnno]

		if hook == "" {
			resources += "\n---\n" + m
			continue
		}

		hookEvent, ok := events[hook]
		if !ok {
			return "", nil, fmt.Errorf("unexpected hook: %s", hook)
		}

		if r.Metadata.Name == "" {
			return "", nil, fmt.Errorf("assertion failed: expected metadata.name to be non-nil, but was nil: %+v", r)
		}

		rh := &release.Hook{
			Name:     r.Metadata.Name,
			Kind:     r.Kind,
			Path:     source,
			Manifest: strings.Join(lines[1:], "\n"),
			Events:   []release.Hook_Event{hookEvent},
		}

		result = append(result, rh)
	}

	return resources, result, nil
}

func ExtractHookManifests(manifest, target string) ([]string, error) {
	if target != "" {
		_, ok := events[target]

		if !ok {
			defined := []string{}
			for h, _ := range events {
				defined = append(defined, h)
			}
			return nil, fmt.Errorf("unknown target hook \"%s\": must be one of %s", target, strings.Join(defined, ", "))
		}
	}

	manifests := strings.Split(manifest, "\n---\n")

	result := []string{}

	for _, m := range manifests {
		r := resource{Metadata: metadata{}}
		if err := yaml.Unmarshal([]byte(m), &r); err != nil {
			return nil, err
		}
		// See https://github.com/helm/helm/blob/2b36b1ad46278380aa70b8c190c346ce50e8dc96/pkg/hooks/hooks.go#L27
		hook := r.Metadata.Annotations[hooks.HookAnno]

		if hook == "" {
			continue
		}

		if target != "" && hook != target {
			continue
		}

		if _, exists := events[hook]; !exists {
			return nil, fmt.Errorf("unknown hook \"%s\" found. maybe a bug in helm-x?", hook)
		}

		result = append(result, m)
	}

	return result, nil
}
