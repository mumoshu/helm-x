package helmx

import (
	"io"
	"os"
	"os/exec"
	"strings"
)

type Runner struct {
	commander *commander
}

func (r *Runner) Run(name string, args ...string) (string, error) {
	bytes, _, err := r.Capture(name, args)
	if err != nil {
		var out string
		if bytes != nil {
			out = string(bytes)
		}
		return out, err
	}
	return string(bytes), nil
}

func New() *Runner {
	return &Runner{
		commander: &commander{
			runCmd: DefaultRunCommand,
			env:    map[string]string{},
		},
	}
}

func DefaultRunCommand(cmd string, args []string, stdout, stderr io.Writer, env map[string]string) error {
	command := exec.Command(cmd, args...)
	command.Stdout = stdout
	command.Stderr = stderr
	command.Env = mergeEnv(os.Environ(), env)
	return command.Run()
}

type RunCommand func(name string, args []string, stdout, stderr io.Writer, env map[string]string) error

type commander struct {
	runCmd RunCommand

	env map[string]string
}

func (s *commander) run(cmd string, args []string, stdout, stderr io.Writer) error {
	return s.runCmd(cmd, args, stdout, stderr, s.env)
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
