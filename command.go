package maestro

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/user"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/midbel/maestro/internal/env"
	"github.com/midbel/maestro/internal/help"
	"github.com/midbel/maestro/shell"
)

const DefaultSSHPort = 22

const (
	errSilent = "silent"
	errRaise  = "raise"
)

type Executer interface {
	Command() string
	Dependencies() []CommandDep

	Script([]string) ([]string, error)
	Dry([]string) error

	Execute(context.Context, []string) error
	SetOut(w io.Writer)
	SetErr(w io.Writer)
}

type CommandDep struct {
	Name      string
	Args      []string
	Bg        bool
	Optional  bool
	Mandatory bool
}

type CommandOption struct {
	Short    string
	Long     string
	Help     string
	Required bool
	Flag     bool

	Default     string
	DefaultFlag bool
	Target      string
	TargetFlag  bool

	Valid ValidateFunc
}

func (o CommandOption) Validate() error {
	if o.Flag {
		return nil
	}
	if o.Required && o.Target == "" {
		return fmt.Errorf("%s/%s: missing value", o.Short, o.Long)
	}
	if o.Valid == nil {
		return nil
	}
	return o.Valid(o.Target)
}

type CommandArg struct {
	Name  string
	Valid ValidateFunc
}

func (a CommandArg) Validate(arg string) error {
	if a.Valid == nil {
		return nil
	}
	return a.Valid(arg)
}

type CommandLine struct {
	Line     string
	Reverse  bool
	Ignore   bool
	Echo     bool
	Subshell bool
}

type CommandSettings struct {
	Visible bool

	Name       string
	Alias      []string
	Short      string
	Desc       string
	Categories []string

	Users   []string
	Groups  []string
	Error   string
	Retry   int64
	WorkDir string
	Timeout time.Duration

	Hosts     []string
	Deps      []CommandDep
	Lines     []CommandLine
	Options   []CommandOption
	Args      []CommandArg
	Schedules []Schedule

	locals *env.Env
}

func NewCommmandSettings(name string) (CommandSettings, error) {
	return NewCommandSettingsWithLocals(name, env.EmptyEnv())
}

func NewCommandSettingsWithLocals(name string, locals *env.Env) (CommandSettings, error) {
	cmd := CommandSettings{
		Name:   name,
		Error:  errSilent,
		locals: locals,
	}
	if cmd.locals == nil {
		cmd.locals = env.EmptyEnv()
	}
	return cmd, nil
}

func (s CommandSettings) Command() string {
	return s.Name
}

func (s CommandSettings) About() string {
	return s.Short
}

func (s CommandSettings) Help() (string, error) {
	return help.Command(s)
}

func (s CommandSettings) Tags() []string {
	if len(s.Categories) == 0 {
		return []string{"default"}
	}
	return s.Categories
}

func (s CommandSettings) Usage() string {
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
		str.WriteString(a.Name)
		str.WriteString(">")
	}
	return str.String()
}

func (s CommandSettings) Blocked() bool {
	return !s.Visible
}

func (s CommandSettings) Can() bool {
	if len(s.Users) == 0 && len(s.Groups) == 0 {
		return true
	}
	if s.can(strconv.Itoa(os.Geteuid())) {
		return true
	}
	return s.can(strconv.Itoa(os.Getuid()))
}

func (s CommandSettings) can(uid string) bool {
	curr, err := user.LookupId(uid)
	if err != nil {
		return false
	}
	if len(s.Users) > 0 {
		i := sort.SearchStrings(s.Users, curr.Username)
		if i < len(s.Users) && s.Users[i] == curr.Username {
			return true
		}
	}
	if len(s.Groups) > 0 {
		groups, err := curr.GroupIds()
		if err != nil {
			return false
		}
		for _, gid := range groups {
			grp, err := user.LookupGroupId(gid)
			if err != nil {
				continue
			}
			i := sort.SearchStrings(s.Groups, grp.Name)
			if i < len(s.Groups) && s.Groups[i] == grp.Name {
				return true
			}
		}
	}
	return false
}

func (s CommandSettings) Remote() bool {
	return len(s.Hosts) > 0
}

func (s CommandSettings) Prepare() (Executer, error) {
	sh, err := shell.New(shell.WithEnv(s.locals.Copy()))
	if err != nil {
		return nil, err
	}
	cmd := command{
		name:    s.Command(),
		retry:   s.Retry,
		timeout: s.Timeout,
		shell:   sh,
	}
	cmd.help, _ = s.Help()
	cmd.lines = append(cmd.lines, s.Lines...)
	cmd.options = append(cmd.options, s.Options...)
	cmd.args = append(cmd.args, s.Args...)
	cmd.deps = append(cmd.deps, s.Deps...)

	return &cmd, nil
}

type command struct {
	name string
	help string
	deps []CommandDep

	retry   int64
	timeout time.Duration

	lines   []CommandLine
	args    []CommandArg
	options []CommandOption

	shell *shell.Shell
}

func (c *command) Command() string {
	return c.name
}

func (c *command) Dependencies() []CommandDep {
	return c.deps
}

func (c *command) SetOut(w io.Writer) {
	c.shell.SetOut(w)
}

func (c *command) SetErr(w io.Writer) {
	c.shell.SetErr(w)
}

func (c *command) Register(ctx context.Context, other Executer) {
	cmd := makeShellCommand(ctx, other)
	c.shell.Register(cmd)
}

func (c *command) Dry(args []string) error {
	args, err := c.parseArgs(args)
	if err != nil {
		return err
	}
	for _, cmd := range c.lines {
		err = c.shell.Dry(cmd.Line, c.name, args)
		if err != nil {
			break
		}
	}
	return err
}

func (c *command) Script(args []string) ([]string, error) {
	args, err := c.parseArgs(args)
	if err != nil {
		return nil, err
	}
	var list []string
	for _, i := range c.lines {
		rs, err := shell.Expand(i.Line, args, c.shell)
		if err != nil {
			return nil, err
		}
		list = append(list, rs...)
	}
	return list, nil
}

func (c *command) Execute(ctx context.Context, args []string) error {
	args, err := c.parseArgs(args)
	if err != nil {
		return err
	}
	if c.retry <= 0 {
		c.retry = 1
	}
	if c.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}
	for i := int64(0); i < c.retry; i++ {
		err = c.execute(ctx, args)
		if err == nil {
			break
		}
	}
	if err := ctx.Err(); errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return err
}

func (c *command) execute(ctx context.Context, args []string) error {
	for _, cmd := range c.lines {
		if err := ctx.Err(); err != nil {
			break
		}
		sh := c.shell
		if cmd.Subshell {
			sh, _ = sh.Subshell()
		}
		sh.SetEcho(cmd.Echo)
		err := sh.Execute(ctx, cmd.Line, c.name, args)
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

func (c *command) parseArgs(args []string) ([]string, error) {
	set, err := c.prepareArgs(args)
	if err != nil {
		return nil, err
	}
	define := func(name, value string) error {
		if name == "" {
			return nil
		}
		return c.shell.Define(name, []string{value})
	}
	defineFlag := func(name string, value bool) error {
		return define(name, strconv.FormatBool(value))
	}
	for _, o := range c.options {
		if err := o.Validate(); err != nil {
			return nil, err
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
	if z := len(c.args); z > 0 && set.NArg() < z {
		return nil, fmt.Errorf("%s: no enough argument supplied! expected %d, got %d", c.name, z, set.NArg())
	}
	return set.Args(), nil
}

func (c *command) prepareArgs(args []string) (*flag.FlagSet, error) {
	var (
		set  = flag.NewFlagSet(c.name, flag.ExitOnError)
		seen = make(map[string]struct{})
	)
	set.Usage = func() {
		fmt.Fprintln(os.Stdout, strings.TrimSpace(c.help))
		os.Exit(1)
	}
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
	for i, o := range c.options {
		var e1, e2 error
		if o.Flag {
			e1 = attachFlag(o.Short, o.Help, o.DefaultFlag, &c.options[i].TargetFlag)
			e2 = attachFlag(o.Long, o.Help, o.DefaultFlag, &c.options[i].TargetFlag)
		} else {
			e1 = attach(o.Short, o.Help, o.Default, &c.options[i].Target)
			e2 = attach(o.Long, o.Help, o.Default, &c.options[i].Target)
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

type shellCommand struct {
	cmd  Executer
	args []string
	ctx  context.Context

	shell.StdPipe

	done    chan error
	errch   chan error
	running bool
	code    int
}

func makeShellCommand(ctx context.Context, cmd Executer) shell.Command {
	return &shellCommand{
		cmd: cmd,
		ctx: ctx,
	}
}

func (s *shellCommand) SetArgs(args []string) {
	s.args = append(s.args[:0], args...)
}

func (s *shellCommand) Command() string {
	return s.cmd.Command()
}

func (s *shellCommand) Type() shell.CommandType {
	return shell.TypeExternal
}

func (s *shellCommand) Run() error {
	if err := s.Start(); err != nil {
		return err
	}
	return s.Wait()
}

func (s *shellCommand) Start() error {
	if s.running {
		return fmt.Errorf("%s is already running", s.Command())
	}
	s.running = true

	for i, set := range s.StdPipe.SetupFd() {
		rw, err := set()
		if err != nil {
			s.Close()
			return err
		}
		switch i {
		case 0:
			// s.cmd.SetIn(rw)
		case 1:
			s.cmd.SetOut(rw)
		case 2:
			s.cmd.SetErr(rw)
		default:
		}
	}
	if copies := s.Copies(); len(copies) > 0 {
		s.errch = make(chan error, 3)
		for _, fn := range copies {
			go func(fn func() error) {
				s.errch <- fn()
			}(fn)
		}
	}
	s.done = make(chan error, 1)
	go func() {
		s.done <- s.cmd.Execute(s.ctx, s.args)
	}()
	return nil
}

func (s *shellCommand) Wait() error {
	if !s.running {
		return fmt.Errorf("%s is not running", s.Command())
	}
	s.running = false
	var (
		errex = <-s.done
		errcp error
	)
	defer close(s.done)
	s.Close()
	for range s.Copies() {
		e := <-s.errch
		if errcp == nil && e != nil {
			s.code = 2
			errcp = e
		}
	}
	s.Clear()
	if errex != nil {
		s.code = 1
		return errex
	}
	return errcp
}

func (s *shellCommand) Exit() (int, int) {
	return 0, s.code
}
