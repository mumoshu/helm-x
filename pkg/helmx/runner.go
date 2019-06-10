package helmx

import (
	"github.com/mumoshu/helm-x/pkg/cmdsite"
	"io"
	"os"
	"os/exec"
	"strings"
)

type Runner struct {
	commander *cmdsite.CommandSite
}

type Option func(*Runner) error

func Commander(c cmdsite.RunCommand) Option {
	return func(r *Runner) error {
		r.commander.RunCmd = c
		return nil
	}
}

func New(opts ...Option) *Runner {
	cs := cmdsite.New()
	cs.RunCmd = DefaultRunCommand
	r := &Runner{
		commander: cs,
	}
	for i := range opts {
		if err := opts[i](r); err != nil {
			panic(err)
		}
	}
	return r
}

func (r *Runner) Run(name string, args ...string) (string, error) {
	bytes, _, err := r.CaptureBytes(name, args)
	if err != nil {
		var out string
		if bytes != nil {
			out = string(bytes)
		}
		return out, err
	}
	return string(bytes), nil
}

func DefaultRunCommand(cmd string, args []string, stdout, stderr io.Writer, env map[string]string) error {
	command := exec.Command(cmd, args...)
	command.Stdout = stdout
	command.Stderr = stderr
	command.Env = mergeEnv(os.Environ(), env)
	return command.Run()
}

func mergeEnv(orig []string, new map[string]string) []string {
	wanted := env2map(orig)
	for k, v := range new {
		wanted[k] = v
	}
	return map2env(wanted)
}

func map2env(wanted map[string]string) []string {
	result := []string{}
	for k, v := range wanted {
		result = append(result, k+"="+v)
	}
	return result
}

func env2map(env []string) map[string]string {
	wanted := map[string]string{}
	for _, cur := range env {
		pair := strings.SplitN(cur, "=", 2)
		wanted[pair[0]] = pair[1]
	}
	return wanted
}
