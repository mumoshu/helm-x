package main

import (
	"fmt"
	"github.com/mumoshu/helm-x/pkg/cmdsite"
	"github.com/mumoshu/helm-x/pkg/helmx"
	"k8s.io/klog"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"syscall"
)

func helmFallback(r *helmx.Runner, args []string, err error) {
	errMsg := err.Error()
	if strings.HasPrefix(errMsg, `unknown command "`) {
		pattern := regexp.MustCompile(fmt.Sprintf(`unknown command "(.*)" for "%s"`, CommandName))
		matches := pattern.FindSubmatch([]byte(errMsg))
		if matches == nil || len(matches) != 2 {
			panic(fmt.Sprintf("Unexpected error returned from helm-x: %v", err))
		}
		subcmdBytes := matches[1]
		subcmd := string(subcmdBytes)
		switch subcmd {
		case "completion", "create", "delete", "fetch", "get", "helm-git", "help", "history", "home", "init", "inspect", "list", "logs", "package", "plugin", "repo", "reset", "rollback", "search", "serve", "status", "test", "upgrade", "verify", "version":
			args = append([]string{r.HelmBin()}, args...)
			klog.V(1).Infof("helm-x: executing %s\n", strings.Join(args, " "))
			helmBin, err := exec.LookPath(r.HelmBin())
			if err != nil {
				klog.Errorf("%v", err)
				os.Exit(1)
			}
			execErr := syscall.Exec(helmBin, args, os.Environ())
			if execErr != nil {
				panic(execErr)
			}
		case "dependency":
			args = append([]string{r.HelmBin()}, args...)
			klog.V(1).Infof("helm-x: executing %s\n", strings.Join(args, " "))
			helmBin, err := exec.LookPath(r.HelmBin())
			if err != nil {
				klog.Errorf("%v", err)
				os.Exit(1)
			}

			cs := cmdsite.New()
			cs.RunCmd = helmx.DefaultRunCommand

			_, stderr, err := cs.CaptureBytes(helmBin, args[1:])
			if err != nil {
				if stderr != nil && strings.Contains(string(stderr), "Error: chart metadata (Chart.yaml) missing") {
					os.Exit(0)
				} else {
					fmt.Fprintln(os.Stderr, string(stderr))
					os.Exit(1)
				}
			}
		}
	}
}
