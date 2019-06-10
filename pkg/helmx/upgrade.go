package helmx

import (
	"fmt"
	"io"
)

type UpgradeOpts struct {
	*ChartifyOpts
	*ClientOpts

	Chart   string
	Timeout int
	Install bool
	DryRun  bool

	kubeConfig string

	Adopt []string

	Out io.Writer
}

func (r *Runner) Upgrade(o UpgradeOpts) error {
	var additionalFlags string
	additionalFlags += createFlagChain("set", o.SetValues)
	additionalFlags += createFlagChain("f", o.ValuesFiles)
	additionalFlags += createFlagChain("timeout", []string{fmt.Sprintf("%d", o.Timeout)})
	if o.Install {
		additionalFlags += createFlagChain("install", []string{""})
	}
	if o.Namespace != "" {
		additionalFlags += createFlagChain("namespace", []string{o.Namespace})
	}
	if o.KubeContext != "" {
		additionalFlags += createFlagChain("kube-context", []string{o.KubeContext})
	}
	if o.DryRun {
		additionalFlags += createFlagChain("dry-run", []string{""})
	}
	if o.Debug {
		additionalFlags += createFlagChain("debug", []string{""})
	}
	if o.TLS {
		additionalFlags += createFlagChain("tls", []string{""})
	}
	if o.TLSCert != "" {
		additionalFlags += createFlagChain("tls-cert", []string{o.TLSCert})
	}
	if o.TLSKey != "" {
		additionalFlags += createFlagChain("tls-key", []string{o.TLSKey})
	}

	command := fmt.Sprintf("helm upgrade %s %s%s", o.ReleaseName, o.Chart, additionalFlags)
	stdout, stderr, err := r.DeprecatedCaptureBytes(command)
	if err != nil || len(stderr) != 0 {
		return fmt.Errorf(string(stderr))
	}
	fmt.Println(string(stdout))

	return nil
}
