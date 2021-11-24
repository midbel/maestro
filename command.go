package maestro

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/midbel/maestro/shell"
)

const (
	errSilent = "silent"
	errRaise  = "raise"
)

type Command interface {
	About() string
	Help() (string, error)
	Tags() []string
	Command() string
	Combined() bool
	Execute([]string) error
	Dry([]string) error
}

type Dep struct {
	Name string
	Args []string
	Bg   bool
}

type Option struct {
	Short    string
	Long     string
	Default  string
	Help     string
	Required bool
	Flag     bool
	Target   string
}

type Line struct {
	Line    string
	Reverse bool
	Ignore  bool
	Echo    bool
	Empty   bool
	Env     *Env
}

type Single struct {
	Name         string
	Short        string
	Desc         string
	Usage        string
	Error        string
	Categories   []string
	Retry        int64
	WorkDir      string
	Timeout      time.Duration
	Hosts        []string
	Dependencies []Dep
	Scripts      []Line
	Env          map[string]string
	Options      []Option
	Args         int64

	shell *shell.Shell
}

func NewSingle(name string) (*Single, error) {
	return NewSingleWithLocals(name, EmptyEnv())
}

func NewSingleWithLocals(name string, locals *Env) (*Single, error) {
	if locals == nil {
		locals = EmptyEnv()
	} else {
		locals = locals.Copy()
	}
	sh, err := shell.New(shell.WithEnv(locals))
	if err != nil {
		return nil, err
	}
	cmd := Single{
		Name:  name,
		Error: errSilent,
		shell: sh,
	}
	return &cmd, nil
}

func (s *Single) Command() string {
	return s.Name
}

func (s *Single) About() string {
	return s.Short
}

func (s *Single) Help() (string, error) {
	return renderTemplate(cmdhelp, s)
}

func (s *Single) Tags() []string {
	if len(s.Categories) == 0 {
		return []string{"default"}
	}
	return s.Categories
}

func (_ *Single) Combined() bool {
	return false
}

func (s *Single) Dry(args []string) error {
	args, err := s.parseArgs(args)
	if err != nil {
		return err
	}
	for _, cmd := range s.Scripts {
		if err := s.shell.Dry(cmd.Line, s.Name, args); err != nil && s.Error == errRaise {
			return err
		}
	}
	return nil
}

func (s *Single) Execute(args []string) error {
	if err := s.shell.Chdir(s.WorkDir); err != nil {
		return err
	}
	args, err := s.parseArgs(args)
	if err != nil {
		return err
	}
	for _, cmd := range s.Scripts {
		if cmd.Echo {
			fmt.Fprintln(os.Stdout, cmd.Line)
		}
		err := s.shell.Execute(cmd.Line, s.Name, args)
		if cmd.Reverse {
			if err == nil {
				err = fmt.Errorf("command succeed")
			} else {
				err = nil
			}
		}
		if !cmd.Ignore && err != nil && s.Error == errRaise {
			return err
		}
	}
	return nil
}

func (s *Single) parseArgs(args []string) ([]string, error) {
	set, err := createFlagSet(s.Name, args, s.Options)
	if err != nil {
		return nil, err
	}
	define := func(name, value string) error {
		if name == "" {
			return nil
		}
		return s.shell.Define(name, []string{value})
	}
	for _, o := range s.Options {
		if o.Required && o.Target == "" {
			return nil, fmt.Errorf("%s/%s: missing value", o.Short, o.Long)
		}
		if err := define(o.Short, o.Target); err != nil {
			return nil, err
		}
		if err := define(o.Long, o.Target); err != nil {
			return nil, err
		}
	}
	if s.Args > 0 && set.NArg() < int(s.Args) {
		return nil, fmt.Errorf("%s: no enough argument supplied! expected %d, got %d", s.Name, s.Args, set.NArg())
	}
	return set.Args(), nil
}

func createFlagSet(name string, args []string, options []Option) (*flag.FlagSet, error) {
	var (
		set  = flag.NewFlagSet(name, flag.ExitOnError)
		seen = make(map[string]struct{})
	)
	attach := func(name, value, help string, target *string) error {
		if name == "" {
			return nil
		}
		if _, ok := seen[name]; ok {
			return fmt.Errorf("%s: option already defined", name)
		}
		seen[name] = struct{}{}
		set.StringVar(target, name, value, help)
		return nil
	}
	for i, o := range options {
		if err := attach(o.Short, o.Default, o.Help, &options[i].Target); err != nil {
			return nil, err
		}
		if err := attach(o.Long, o.Default, o.Help, &options[i].Target); err != nil {
			return nil, err
		}
	}
	if err := set.Parse(args); err != nil {
		return nil, err
	}
	return set, nil
}

type Combined []Command

func (c Combined) Command() string {
	return c[0].Command()
}

func (c Combined) Tags() []string {
	var (
		tags []string
		seen = make(map[string]struct{})
	)
	for i := range c {
		for _, t := range c[i].Tags() {
			if _, ok := seen[t]; ok {
				continue
			}
			seen[t] = struct{}{}
			tags = append(tags, t)
		}
	}
	return tags
}

func (c Combined) About() string {
	return c[0].About()
}

func (c Combined) Help() (string, error) {
	return c[0].Help()
}

func (_ Combined) Combined() bool {
	return true
}

func (c Combined) Execute(args []string) error {
	for i := range c {
		if err := c[i].Execute(args); err != nil {
			return err
		}
	}
	return nil
}

func (c Combined) Dry(args []string) error {
	for i := range c {
		if err := c[i].Dry(args); err != nil {
			return err
		}
	}
	return nil
}
