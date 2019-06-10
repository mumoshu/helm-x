package helmx

import (
	"fmt"
	"io/ioutil"
	"strings"
)

type InjectOpts struct {
	injectors []string
	injects   []string
	files     []string
}

func (r *Runner) Inject(o InjectOpts) error {
	var flagsTemplate string
	for _, inj := range o.injectors {

		tokens := strings.Split(inj, ",")
		injector := tokens[0]
		injectFlags := tokens[1:]
		for _, flag := range injectFlags {
			flagSplit := strings.Split(flag, "=")
			switch len(flagSplit) {
			case 1:
				flagsTemplate += flagSplit[0]
			case 2:
				key, val := flagSplit[0], flagSplit[1]
				flagsTemplate += createFlagChain(key, []string{val})
			default:
				return fmt.Errorf("inject-flags must be in the form of key1=value1[,key2=value2,...]: %v", flag)
			}
		}
		for _, file := range o.files {
			flags := strings.Replace(flagsTemplate, "FILE", file, 1)
			command := fmt.Sprintf("%s %s", injector, flags)
			stdout, stderr, err := r.DeprecatedCaptureBytes(command)
			if err != nil {
				return fmt.Errorf(string(stderr))
			}
			if err := ioutil.WriteFile(file, stdout, 0644); err != nil {
				return err
			}
		}
	}

	for _, tmpl := range o.injects {
		for _, file := range o.files {
			cmd := strings.Replace(tmpl, "FILE", file, 1)

			stdout, stderr, err := r.DeprecatedCaptureBytes(cmd)
			if err != nil {
				return fmt.Errorf(string(stderr))
			}
			if err := ioutil.WriteFile(file, stdout, 0644); err != nil {
				return err
			}
		}
	}

	return nil
}
