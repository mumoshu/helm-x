package helmx

import (
	"fmt"
	"github.com/variantdev/chartify"
	"github.com/mumoshu/helm-x/pkg/util"
	"io"
)

type UpgradeOpts struct {
	*chartify.ChartifyOpts
	*ClientOpts

	Timeout string
	Install bool
	DryRun  bool

	ResetValues bool

	kubeConfig string

	Adopt []string

	Out io.Writer
}

func (r *Runner) Upgrade(release, chart string, o UpgradeOpts) error {
	var additionalFlags string
	additionalFlags += util.CreateFlagChain("set", o.SetValues)
	additionalFlags += util.CreateFlagChain("f", o.ValuesFiles)
	timeout := o.Timeout
	if r.IsHelm3() {
		timeout = timeout + "s"
	}
	additionalFlags += util.CreateFlagChain("timeout", []string{fmt.Sprintf("%s", timeout)})
	if o.Install {
		additionalFlags += util.CreateFlagChain("install", []string{""})
	}
	if o.ResetValues {
		additionalFlags += util.CreateFlagChain("reset-values", []string{""})
	}
	if o.Namespace != "" {
		additionalFlags += util.CreateFlagChain("namespace", []string{o.Namespace})
	}
	if o.KubeContext != "" {
		additionalFlags += util.CreateFlagChain("kube-context", []string{o.KubeContext})
	}
	if o.DryRun {
		additionalFlags += util.CreateFlagChain("dry-run", []string{""})
	}
	if o.Debug {
		additionalFlags += util.CreateFlagChain("debug", []string{""})
	}
	if o.TLS {
		additionalFlags += util.CreateFlagChain("tls", []string{""})
	}
	if o.TLSCert != "" {
		additionalFlags += util.CreateFlagChain("tls-cert", []string{o.TLSCert})
	}
	if o.TLSKey != "" {
		additionalFlags += util.CreateFlagChain("tls-key", []string{o.TLSKey})
	}

	command := fmt.Sprintf("%s upgrade %s %s%s", r.HelmBin(), release, chart, additionalFlags)
	stdout, stderr, err := r.DeprecatedCaptureBytes(command)
	if err != nil || len(stderr) != 0 {
		return fmt.Errorf(string(stderr))
	}
	fmt.Println(string(stdout))

	return nil
}
