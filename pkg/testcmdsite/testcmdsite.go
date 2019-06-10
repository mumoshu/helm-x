package testcmdsite

import (
	"bufio"
	"fmt"
	"github.com/mumoshu/helm-x/pkg/cmdsite"
	"github.com/spf13/pflag"
	"io"
	"os"
)

type Command struct {
	ID int

	Args []string

	StringFlags map[string]string
	BoolFlags   map[string]bool
	IntFlags    map[string]int

	Stdout string
	Stderr string
}

type TestCommandSite struct {
	nextId int

	*cmdsite.CommandSite
	Commands map[string][]*Command
}

type Result struct {
	Stdout string
	Stderr string
}

func New() *TestCommandSite {
	tcs := &TestCommandSite{
		Commands:    map[string][]*Command{},
		CommandSite: cmdsite.New(),
	}
	tcs.CommandSite.RunCmd = tcs.runCmd
	return tcs
}

func (s *TestCommandSite) Add(cmd string, flags map[string]interface{}, args []string, stdout, stderr string) (int, error) {
	c := &Command{
		Args:        args,
		StringFlags: map[string]string{},
		BoolFlags:   map[string]bool{},
		IntFlags:    map[string]int{},
		Stdout:      stdout,
		Stderr:      stderr,
	}

	for n, v := range flags {
		switch typed := v.(type) {
		case string:
			c.StringFlags[n] = typed
		case int:
			c.IntFlags[n] = typed
		case bool:
			c.BoolFlags[n] = typed
		default:
			return -1, fmt.Errorf("unsupported flag type: value=%v, type=%T", typed, typed)
		}
	}

	id := s.nextId

	c.ID = id
	s.Commands[cmd] = append(s.Commands[cmd], c)

	s.nextId += 1

	fmt.Fprintf(os.Stderr, "after add: %v\n", *s)

	return id, nil
}

func (s *TestCommandSite) runCmd(cmd string, args []string, stdout, stderr io.Writer, env map[string]string) error {
	cmds, ok := s.Commands[cmd]
	if !ok {
		return fmt.Errorf("unexpected call to command \"%s\"", cmd)
	}

	for i := range cmds {
		c := cmds[i]

		fs := pflag.NewFlagSet(cmd, pflag.ContinueOnError)
		for f, v := range c.BoolFlags {
			_ = fs.Bool(f, false, "")
			fmt.Fprintf(os.Stderr, "expecting bool flag %s=%v\n", f, v)
		}
		for f, v := range c.StringFlags {
			_ = fs.String(f, "", "")
			fmt.Fprintf(os.Stderr, "expecting string flag %s=%v\n", f, v)
		}
		for f, v := range c.IntFlags {
			_ = fs.Int(f, 0, "")
			fmt.Fprintf(os.Stderr, "expecting int flag %s=%v\n", f, v)
		}

		a := append([]string{}, args...)

		fmt.Fprintf(os.Stderr, "parsing flags from: %v\n", a)

		if err := fs.Parse(a); err != nil {
			if i == len(cmds)-1 {
				return err
			}
			continue
		}

		a = fs.Args()

		fmt.Fprintf(os.Stderr, "remaining: %v\n", a)

		expectedArgs := map[string]struct{}{}
		actualArgs := map[string]struct{}{}

		for _, arg := range a {
			actualArgs[arg] = struct{}{}
		}
		for _, arg := range c.Args {
			expectedArgs[arg] = struct{}{}
		}

		for arg := range actualArgs {
			if _, ok := expectedArgs[arg]; !ok {
				return fmt.Errorf("unexpected arg: got=%v, expected=any of %v", arg, expectedArgs)
			}
			delete(expectedArgs, arg)
		}
		for arg := range expectedArgs {
			if _, ok := actualArgs[arg]; !ok {
				return fmt.Errorf("missing arg: expected=%v, got none", arg)
			}
		}

		for f, expected := range c.BoolFlags {
			actual, err := fs.GetBool(f)
			if err != nil {
				return err
			}
			if actual != expected {
				return fmt.Errorf("unexpected flag value: flag=%s, expected=%v, got=%v", f, expected, actual)
			}
		}
		for f, expected := range c.StringFlags {
			actual, err := fs.GetString(f)
			if err != nil {
				return err
			}
			if actual != expected {
				return fmt.Errorf("unexpected flag value: flag=%s, expected=%v, got=%v", f, expected, actual)
			}
		}
		for f, expected := range c.IntFlags {
			actual, err := fs.GetInt(f)
			if err != nil {
				return err
			}
			if actual != expected {
				return fmt.Errorf("unexpected flag value: flag=%s, expected=%v, got=%v", f, expected, actual)
			}
		}

		stdoutbuf := bufio.NewWriter(stdout)
		n, err := stdoutbuf.WriteString(c.Stdout)
		if err != nil {
			return err
		}
		if n != len(c.Stdout) {
			return fmt.Errorf("unable to write %d bytes", len(c.Stdout)-n)
		}
		if err := stdoutbuf.Flush(); err != nil {
			return err
		}

		stderrbuf := bufio.NewWriter(stderr)
		n, err = stderrbuf.WriteString(c.Stderr)
		if err != nil {
			return err
		}
		if n != len(c.Stderr) {
			return fmt.Errorf("unable to write %d bytes", len(c.Stderr)-n)
		}
		if err := stderrbuf.Flush(); err != nil {
			return err
		}

		return nil
	}

	return fmt.Errorf("invalid state")
}
