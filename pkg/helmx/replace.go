package helmx

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
)

type ReplaceWithRenderedOpts struct {
	// Debug when set to true passes `--debug` flag to `helm` in order to enable debug logging
	Debug bool

	// ValuesFiles are a list of Helm chart values files
	ValuesFiles []string

	// SetValues is a list of adhoc Helm chart values being passed via helm's `--set` flags
	SetValues []string

	// Namespace is the default namespace in which the K8s manifests rendered by the chart are associated
	Namespace string

	// ChartVersion is the semver of the Helm chart being used to render the original K8s manifests before various tweaks applied by helm-x
	ChartVersion string
}

func (r *Runner) ReplaceWithRendered(name, chart string, files []string, o ReplaceWithRenderedOpts) error {
	var additionalFlags string
	additionalFlags += createFlagChain("set", o.SetValues)
	defaultValuesPath := filepath.Join(chart, "values.yaml")
	exists, err := exists(defaultValuesPath)
	if err != nil {
		return err
	}
	if exists {
		additionalFlags += createFlagChain("f", []string{defaultValuesPath})
	}
	additionalFlags += createFlagChain("f", o.ValuesFiles)
	if o.Namespace != "" {
		additionalFlags += createFlagChain("namespace", []string{o.Namespace})
	}
	if o.ChartVersion != "" {
		additionalFlags += createFlagChain("version", []string{o.ChartVersion})
	}

	for _, file := range files {
		command := fmt.Sprintf("helm template --debug=%v %s --name %s -x %s%s", o.Debug, chart, name, file, additionalFlags)
		stdout, stderr, err := r.DeprecatedCaptureBytes(command)
		if err != nil || len(stderr) != 0 {
			return fmt.Errorf(string(stderr))
		}
		if err := ioutil.WriteFile(file, stdout, 0644); err != nil {
			return err
		}
	}

	return nil
}
