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
}

type Line struct {
	Line    string
	Dry     bool
	Reverse bool
	Ignore  bool
	Echo    bool
	Empty   bool
	Env     *Env
}

func (i Line) Subshell() bool {
	return i.Env != nil
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
		if err := s.shell.Dry(cmd.Line, s.Name, args); err != nil {
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
		if !cmd.Ignore && err != nil {
			return err
		}
	}
	return nil
}

func (s *Single) parseArgs(args []string) ([]string, error) {
	set := flag.NewFlagSet(s.Name, flag.ExitOnError)
	if len(s.Options) > 0 {

	}
	if err := set.Parse(args); err != nil {
		return nil, err
	}
	if s.Args > 0 && set.NArg() != int(s.Args) {
		return nil, fmt.Errorf("%s: no enough argument supplied! expected %d, got %d", s.Name, s.Args, set.NArg())
	}
	return set.Args(), nil
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
