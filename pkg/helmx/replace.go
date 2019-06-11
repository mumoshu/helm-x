package helmx

import (
	"fmt"
	//"io/ioutil"
	"k8s.io/klog"
	"os"
	"path/filepath"
	"strings"
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

func (r *Runner) ReplaceWithRendered(name, chart string, files []string, o ReplaceWithRenderedOpts) ([]string, error) {
	var additionalFlags string
	additionalFlags += createFlagChain("set", o.SetValues)
	defaultValuesPath := filepath.Join(chart, "values.yaml")
	exists, err := exists(defaultValuesPath)
	if err != nil {
		return nil, err
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

	klog.Infof("options: %v", o)

	dir := filepath.Join(chart, "helmx.1.rendered")
	if err := os.Mkdir(dir, 0755); err != nil {
		return nil, err
	}

	command := fmt.Sprintf("helm template --debug=%v %s --name %s%s --output-dir %s", o.Debug, chart, name, additionalFlags, dir)
	stdout, stderr, err := r.DeprecatedCaptureBytes(command)
	if err != nil {
		if len(stderr) != 0 {
			return nil, fmt.Errorf(string(stderr))
		}
		return nil, fmt.Errorf("unexpected error with no stderr contents: %s", stdout)
	}
	results := []string{}
	lines := strings.Split(string(stdout), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "wrote ") {
			results = append(results, strings.Split(line, "wrote ")[1])
		}
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("invalid state: no files rendered")
	}

	for _, f := range files {
		klog.Infof("removing %s", f)
		if err := os.Remove(f); err != nil {
			return nil, err
		}
	}

	return results, nil
}
