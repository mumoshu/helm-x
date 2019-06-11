package helmx

import (
	"fmt"
	"io"
	"os/exec"
)

type DiffOpts struct {
	*ChartifyOpts
	*ClientOpts

	Chart string

	kubeConfig string

	Out io.Writer
}

func (o DiffOpts) GetSetValues() []string {
	return o.SetValues
}

func (o DiffOpts) GetValuesFiles() []string {
	return o.ValuesFiles
}

func (o DiffOpts) GetNamespace() string {
	return o.Namespace
}

func (o DiffOpts) GetKubeContext() string {
	return o.KubeContext
}

func (o DiffOpts) GetTLS() bool {
	return o.TLS
}

func (o DiffOpts) GetTLSCert() string {
	return o.TLSCert
}

func (o DiffOpts) GetTLSKey() string {
	return o.TLSKey
}

type diffOptsProvider interface {
	GetSetValues() []string
	GetValuesFiles() []string
	GetNamespace() string
	GetKubeContext() string
	GetTLS() bool
	GetTLSCert() string
	GetTLSKey() string
}

type DiffOption interface {
	SetDiffOption(*DiffOpts) error
}

type diffOptsSetter struct {
	o diffOptsProvider
}

func (s *diffOptsSetter) SetDiffOption(o *DiffOpts) error {
	opts := s.o
	o.SetValues = opts.GetSetValues()
	o.ValuesFiles = opts.GetValuesFiles()
	o.Namespace = opts.GetNamespace()
	o.KubeContext = opts.GetKubeContext()
	o.TLS = opts.GetTLS()
	o.TLSCert = opts.GetTLSCert()
	o.TLSKey = opts.GetTLSKey()
	return nil
}

func (s *DiffOpts) SetDiffOption(o *DiffOpts) error {
	*o = *s
	return nil
}

func WithDiffOpts(opts diffOptsProvider) DiffOption {
	return &diffOptsSetter{o: opts}
}

// Diff returns true when the diff succeeds and changes are detected.
func (r *Runner) Diff(release, chart string, opts ...DiffOption) (bool, error) {
	o := &DiffOpts{}

	for i := range opts {
		if err := opts[i].SetDiffOption(o); err != nil {
			return false, err
		}
	}

	var additionalFlags string
	additionalFlags += createFlagChain("set", o.SetValues)
	additionalFlags += createFlagChain("f", o.ValuesFiles)
	additionalFlags += createFlagChain("allow-unreleased", []string{""})
	additionalFlags += createFlagChain("detailed-exitcode", []string{""})
	additionalFlags += createFlagChain("context", []string{"3"})
	additionalFlags += createFlagChain("reset-values", []string{""})
	additionalFlags += createFlagChain("suppress-secrets", []string{""})
	if o.GetNamespace() != "" {
		additionalFlags += createFlagChain("namespace", []string{o.Namespace})
	}
	if o.GetKubeContext() != "" {
		additionalFlags += createFlagChain("kube-context", []string{o.KubeContext})
	}
	if o.GetTLS() {
		additionalFlags += createFlagChain("tls", []string{""})
	}
	if o.GetTLSCert() != "" {
		additionalFlags += createFlagChain("tls-cert", []string{o.TLSCert})
	}
	if o.GetTLSKey() != "" {
		additionalFlags += createFlagChain("tls-key", []string{o.TLSKey})
	}

	command := fmt.Sprintf("helm diff upgrade %s %s%s", release, chart, additionalFlags)
	if err := r.DeprecatedExec(command); err != nil {
		switch e := err.(type) {
		case *exec.ExitError:
			if e.ExitCode() == 2 {
				return true, nil
			}
		}
		return false, err

	}
	return false, nil
}
