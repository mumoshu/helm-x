package helmx

import (
	"fmt"
	"github.com/variantdev/chartify"
	"github.com/mumoshu/helm-x/pkg/util"
	"io"
	"os/exec"
)

type DiffOpts struct {
	*chartify.ChartifyOpts
	*ClientOpts

	Chart string

	kubeConfig string

	AllowUnreleased  bool
	DetailedExitcode bool
	ResetValues      bool

	Out io.Writer
}

/*func (o DiffOpts) GetSetValues() []string {
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

func WithDiffOpts(opts diffOptsProvider) DiffOption {
	return &diffOptsSetter{o: opts}
}*/

type DiffOption interface {
	SetDiffOption(*DiffOpts) error
}

func (s *DiffOpts) SetDiffOption(o *DiffOpts) error {
	*o = *s
	return nil
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
	additionalFlags += util.CreateFlagChain("context", []string{"3"})
	if len(o.SetValues) > 0 {
		additionalFlags += util.CreateFlagChain("set", o.SetValues)
	}
	if len(o.ValuesFiles) > 0 {
		additionalFlags += util.CreateFlagChain("f", o.ValuesFiles)
	}
	if o.ResetValues {
		additionalFlags += util.CreateFlagChain("reset-values", []string{""})
	}
	additionalFlags += util.CreateFlagChain("suppress-secrets", []string{""})
	if o.Namespace != "" {
		additionalFlags += util.CreateFlagChain("namespace", []string{o.Namespace})
	}
	if o.KubeContext != "" {
		additionalFlags += util.CreateFlagChain("kube-context", []string{o.KubeContext})
	}
	if o.ChartVersion != "" {
		additionalFlags += util.CreateFlagChain("version", []string{o.ChartVersion})
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
	if o.AllowUnreleased {
		additionalFlags += util.CreateFlagChain("allow-unreleased", []string{""})
	}
	if o.DetailedExitcode {
		additionalFlags += util.CreateFlagChain("detailed-exitcode", []string{""})
	}

	command := fmt.Sprintf("%s diff upgrade %s %s%s", r.HelmBin(), release, chart, additionalFlags)
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
