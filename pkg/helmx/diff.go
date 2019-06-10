package helmx

import (
	"fmt"
	"io"
)

type DiffOpts struct {
	*ChartifyOpts
	*ClientOpts

	Chart string

	kubeConfig string

	Out io.Writer
}

func Diff(o DiffOpts) error {
	var additionalFlags string
	additionalFlags += createFlagChain("set", o.SetValues)
	additionalFlags += createFlagChain("f", o.ValuesFiles)
	additionalFlags += createFlagChain("allow-unreleased", []string{""})
	additionalFlags += createFlagChain("detailed-exitcode", []string{""})
	additionalFlags += createFlagChain("context", []string{"3"})
	additionalFlags += createFlagChain("reset-values", []string{""})
	additionalFlags += createFlagChain("suppress-secrets", []string{""})
	if o.Namespace != "" {
		additionalFlags += createFlagChain("namespace", []string{o.Namespace})
	}
	if o.KubeContext != "" {
		additionalFlags += createFlagChain("kube-context", []string{o.KubeContext})
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

	command := fmt.Sprintf("helm diff upgrade %s %s%s", o.ReleaseName, o.Chart, additionalFlags)
	err := Exec(command)
	if err != nil {
		return err
	}

	return nil
}
