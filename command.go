package maestro

import (
	"flag"
	"fmt"
	"strconv"
	"strings"
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
	Usage() string
	Tags() []string
	Command() string
	Combined() bool
	Dry([]string) error
	Remote() bool

	shell.Command
}

type Dep struct {
	Name string
	Args []string
	Bg   bool
}

type Option struct {
	Short    string
	Long     string
	Help     string
	Required bool
	Flag     bool

	Default     string
	DefaultFlag bool
	Target      string
	TargetFlag  bool
}

type Line struct {
	Line    string
	Reverse bool
	Ignore  bool
	Echo    bool
}

type Single struct {
	Name       string
	Alias      []string
	Short      string
	Desc       string
	Error      string
	Categories []string
	Retry      int64
	WorkDir    string
	Timeout    time.Duration
	Hosts      []string
	Deps       []Dep
	Scripts    []Line
	Env        map[string]string
	Options    []Option
	Args       []string

	locals *Env
	shell  *shell.Shell
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
		Name:   name,
		Error:  errSilent,
		shell:  sh,
		locals: locals,
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

func (s *Single) Usage() string {
	var str strings.Builder
	str.WriteString(s.Name)
	for _, o := range s.Options {
		str.WriteString(" ")
		str.WriteString("[")
		if o.Short != "" {
			str.WriteString("-")
			str.WriteString(o.Short)
		}
		if o.Short != "" && o.Long != "" {
			str.WriteString("/")
		}
		if o.Long != "" {
			str.WriteString("--")
			str.WriteString(o.Long)
		}
		str.WriteString("]")
	}
	for _, a := range s.Args {
		str.WriteString(" ")
		str.WriteString("<")
		str.WriteString(a)
		str.WriteString(">")
	}
	return str.String()
}

func (_ *Single) Combined() bool {
	return false
}

func (s *Single) Remote() bool {
	return len(s.Hosts) > 0
}

func (s *Single) Register(list []shell.Command) {
	s.shell.Register(list...)
}

func (s *Single) Status() (int, int) {
	return s.shell.ExitStatus()
}

func (_ *Single) Type() shell.CommandType {
	return shell.TypeScript
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
	retry := s.Retry
	if retry <= 0 {
		retry = 1
	}
	for i := int64(0); i < retry; i++ {
		err = s.execute(args)
		if err == nil {
			break
		}
	}
	return err
}

func (s *Single) execute(args []string) error {
	for _, cmd := range s.Scripts {
		s.shell.SetEcho(cmd.Echo)
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
	defineFlag := func(name string, value bool) error {
		return define(name, strconv.FormatBool(value))
	}
	for _, o := range s.Options {
		if o.Required && o.Target == "" {
			return nil, fmt.Errorf("%s/%s: missing value", o.Short, o.Long)
		}
		var e1, e2 error
		if o.Flag {
			e1 = defineFlag(o.Short, o.TargetFlag)
			e2 = defineFlag(o.Long, o.TargetFlag)
		} else {
			e1 = define(o.Short, o.Target)
			e2 = define(o.Long, o.Target)
		}
		if err := hasError(e1, e2); err != nil {
			return nil, err
		}
	}
	if z := len(s.Args); z > 0 && set.NArg() < z {
		return nil, fmt.Errorf("%s: no enough argument supplied! expected %d, got %d", s.Name, z, set.NArg())
	}
	return set.Args(), nil
}

func createFlagSet(name string, args []string, options []Option) (*flag.FlagSet, error) {
	var (
		set  = flag.NewFlagSet(name, flag.ExitOnError)
		seen = make(map[string]struct{})
	)
	check := func(name string) error {
		if name == "" {
			return nil
		}
		if _, ok := seen[name]; ok {
			return fmt.Errorf("%s: option already defined", name)
		}
		seen[name] = struct{}{}
		return nil
	}
	attach := func(name, help, value string, target *string) error {
		err := check(name)
		if err == nil {
			set.StringVar(target, name, value, help)
		}
		return err
	}
	attachFlag := func(name, help string, value bool, target *bool) error {
		err := check(name)
		if err == nil {
			set.BoolVar(target, name, value, help)
		}
		return err
	}
	for i, o := range options {
		var e1, e2 error
		if o.Flag {
			e1 = attachFlag(o.Short, o.Help, o.DefaultFlag, &options[i].TargetFlag)
			e2 = attachFlag(o.Long, o.Help, o.DefaultFlag, &options[i].TargetFlag)
		} else {
			e1 = attach(o.Short, o.Help, o.Default, &options[i].Target)
			e2 = attach(o.Long, o.Help, o.Default, &options[i].Target)
		}
		if err := hasError(e1, e2); err != nil {
			return nil, err
		}
	}
	if err := set.Parse(args); err != nil {
		return nil, err
	}
	return set, nil
}

func hasError(errs ...error) error {
	for _, e := range errs {
		if e != nil {
			return e
		}
	}
	return nil
}

type Combined []Command

func (c Combined) Command() string {
	return c[0].Command()
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

func (c Combined) Type() shell.CommandType {
	return c[0].Type()
}

func (c Combined) Status() (int, int) {
	z := len(c) - 1
	return c[z].Status()
}

func (c Combined) Remote() bool {
	for i := range c {
		if !c[i].Remote() {
			return false
		}
	}
	return true
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

func (c Combined) Usage() string {
	return ""
}
