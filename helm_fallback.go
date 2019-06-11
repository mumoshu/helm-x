package main

import (
	"fmt"
	"k8s.io/klog"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"syscall"
)

func helmFallback(err error) {
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
		case "completion", "create", "delete", "dependency", "fetch", "get", "helm-git", "help", "history", "home", "init", "inspect", "list", "logs", "package", "plugin", "repo", "reset", "rollback", "search", "serve", "status", "test", "upgrade", "verify", "version":
			args := os.Args[1:]
			if args[0] == "x" {
				args = args[1:]
			}
			klog.Infof("helm-x: executing helm %s\n", strings.Join(args, " "))
			helmBin, err := exec.LookPath("helm")
			if err != nil {
				klog.Errorf("%v", err)
				os.Exit(1)
			}
			execErr := syscall.Exec(helmBin, args, os.Environ())
			if execErr != nil {
				panic(execErr)
			}
		}
	}
}
