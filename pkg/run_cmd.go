package apply

import (
	"os/exec"
	"strings"

	"github.com/pkg/errors"
	"k8s.io/klog"
)

func RunCommandExt(cmd *exec.Cmd) (string, error) {
	cmdStr := strings.Join(cmd.Args, " ")
	klog.Info(cmdStr)
	outBytes, err := cmd.Output()
	if err != nil {
		exErr, ok := err.(*exec.ExitError)
		if !ok {
			return "", errors.WithStack(err)
		}
		errOutput := string(exErr.Stderr)
		klog.Errorf("`%s` failed: %s", cmdStr, errOutput)
		return "", errors.New(strings.TrimSpace(errOutput))
	}
	// Trims off a single newline for user convenience
	output := string(outBytes)
	outputLen := len(output)
	if outputLen > 0 && output[outputLen-1] == '\n' {
		output = output[0 : outputLen-1]
	}
	klog.V(1).Info(output)
	return output, nil
}

func RunCommand(name string, arg ...string) (string, error) {
	return RunCommandExt(exec.Command(name, arg...))
}
