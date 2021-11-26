package shell

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"
)

var (
	ErrExit     = errors.New("exit")
	ErrReadOnly = errors.New("read only")
	ErrEmpty    = errors.New("empty command")
)

type ExitCode int8

const (
	Success ExitCode = iota
	Failure
)

func (e ExitCode) Success() bool {
	return e == Success
}

func (e ExitCode) Failure() bool {
	return !e.Success()
}

func (e ExitCode) Error() string {
	return fmt.Sprintf("%d", e)
}

type CommandType int8

const (
	TypeBuiltin CommandType = iota
	TypeScript
	TypeRegular
)

type Command interface {
	// Run() error
	// Start() error
	// Wait() error
	// StdoutPipe() (io.ReadCloser, error)
	// StderrPipe() (io.ReadCloser, error)
	Command() string
	Execute([]string) error
	Type() CommandType
	Status() (int, int)
}

var specials = map[string]struct{}{
	"SECONDS": {},
	"PWD":     {},
	"OLDPWD":  {},
	"PID":     {},
	"PPID":    {},
	"RANDOM":  {},
	"PATH":    {},
	"?":       {},
	"#":       {},
	"0":       {},
	"$":       {},
	"@":       {},
}

type Shell struct {
	locals   Environment
	alias    map[string][]string
	commands map[string]Command
	echo     bool
	cwd      string
	now      time.Time

	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer

	context struct {
		pid  int
		code int
		name string
		args []string
	}

	builtins map[string]Builtin
}

func New(options ...ShellOption) (*Shell, error) {
	s := Shell{
		now:      time.Now(),
		cwd:      ".",
		alias:    make(map[string][]string),
		commands: make(map[string]Command),
		builtins: builtins,
	}
	s.cwd, _ = os.Getwd()
	for i := range options {
		if err := options[i](&s); err != nil {
			return nil, err
		}
	}
	if s.stdout == nil {
		s.stdout = os.Stdout
	}
	if s.stderr == nil {
		s.stderr = os.Stderr
	}
	if s.stdin == nil {
		s.stdin = os.Stdin
	}
	if s.locals == nil {
		s.locals = EmptyEnv()
	}
	return &s, nil
}

func (s *Shell) SetStdout(w io.Writer) {
	s.stdout = w
}

func (s *Shell) SetStderr(w io.Writer) {
	s.stderr = w
}

func (s *Shell) Chdir(dir string) error {
	if dir == "" {
		return nil
	}
	err := os.Chdir(dir)
	if err == nil {
		s.cwd = dir
	}
	return err
}

func (s *Shell) SetEcho(echo bool) {
	s.echo = echo
}

func (s *Shell) Alias(ident, script string) error {
	p := NewParser(strings.NewReader(script))
	ex, err := p.Parse()
	if err != nil {
		return err
	}
	alias, err := s.expandExecuter(ex)
	if err != nil {
		return err
	}
	if len(alias) == 0 || len(alias) > 1 {
		return fmt.Errorf("invalid alias definition %s", script)
	}
	s.alias[ident] = alias[0]
	if _, err := p.Parse(); err == nil || errors.Is(err, io.EOF) {
		return nil
	}
	return fmt.Errorf("invalid alias definition (%s)", script)
}

func (s *Shell) Unalias(ident string) {
	delete(s.alias, ident)
}

func (s *Shell) Subshell() (*Shell, error) {
	options := []ShellOption{
		WithEnv(s.locals),
		WithCwd(s.cwd),
	}
	if s.echo {
		options = append(options, WithEcho())
	}
	sub, err := New(options...)
	if err != nil {
		return nil, err
	}
	for n, str := range s.alias {
		sub.alias[n] = str
	}
	return sub, nil
}

func (s *Shell) Resolve(ident string) ([]string, error) {
	str := s.resolveSpecials(ident)
	if len(str) > 0 {
		return str, nil
	}
	return s.locals.Resolve(ident)
}

func (s *Shell) Define(ident string, values []string) error {
	if _, ok := specials[ident]; ok {
		return ErrReadOnly
	}
	return s.locals.Define(ident, values)
}

func (s *Shell) Delete(ident string) error {
	if _, ok := specials[ident]; ok {
		return ErrReadOnly
	}
	return s.locals.Delete(ident)
}

func (s *Shell) Dry(str, cmd string, args []string) error {
	s.setContext(cmd, args)
	defer s.clearContext()

	p := NewParser(strings.NewReader(str))
	for {
		ex, err := p.Parse()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}
		str, err := s.expandExecuter(ex)
		if err != nil {
			continue
		}
		for i := range str {
			io.WriteString(s.stdout, strings.Join(str[i], " "))
			io.WriteString(s.stdout, "\n")
		}
	}
	return nil
}

func (s *Shell) Register(list ...Command) {
	for i := range list {
		s.commands[list[i].Command()] = list[i]
	}
}

func (s *Shell) ExitStatus() (int, int) {
	return s.context.pid, s.context.code
}

func (s *Shell) Execute(str, cmd string, args []string) error {
	s.setContext(cmd, args)
	defer s.clearContext()
	var (
		p   = NewParser(strings.NewReader(str))
		ret error
	)
	for {
		ex, err := p.Parse()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return ret
			}
			return err
		}
		ret = s.execute(ex)
	}
}

func (s *Shell) execute(ex Executer) error {
	var err error
	switch ex := ex.(type) {
	case ExecSimple:
		err = s.executeSingle(ex.Expander)
	case ExecAssign:
		err = s.executeAssign(ex)
	case ExecAnd:
		if err = s.execute(ex.Left); err != nil {
			break
		}
		err = s.execute(ex.Right)
	case ExecOr:
		if err = s.execute(ex.Left); err == nil {
			break
		}
		err = s.execute(ex.Right)
	case ExecPipe:
		err = s.executePipe(ex)
	default:
		err = fmt.Errorf("unsupported executer type %s", ex)
	}
	return err
}

func (s *Shell) executeSingle(ex Expander) error {
	str, err := s.expand(ex)
	if err != nil {
		return err
	}
	if cmd, ok := s.builtins[str[0]]; ok && cmd.IsEnabled() {
		cmd.shell = s
		cmd.args = str[1:]
		cmd.stdout = s.stdout
		cmd.stderr = s.stderr
		cmd.stdin = s.stdin
		err := cmd.Run()
		s.context.pid, s.context.code = os.Getpid(), 0
		if err != nil {
			s.context.code = 1
		}
		return err
	}
	if cmd, ok := s.commands[str[0]]; ok {
		err := cmd.Execute(str[1:])
		s.context.pid, s.context.code = cmd.Status()
		return err
	}
	if _, err := exec.LookPath(str[0]); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		defer cancel()

		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Kill, os.Interrupt)
		<-sig
	}()

	cmd := exec.CommandContext(ctx, str[0], str[1:]...)
	cmd.Stdout = s.stdout
	cmd.Stderr = s.stderr
	err = cmd.Run()
	s.updateContext(cmd)
	return err
}

func (s *Shell) executePipe(ex ExecPipe) error {
	var (
		cs          []*exec.Cmd
		ctx, cancel = context.WithCancel(context.Background())
	)
	go func() {
		defer cancel()

		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Kill, os.Interrupt)
		<-sig
	}()
	for i := range ex.List {
		sex, ok := ex.List[i].Executer.(ExecSimple)
		if !ok {
			return fmt.Errorf("single command expected")
		}
		str, err := s.expand(sex.Expander)
		if err != nil {
			return err
		}
		if _, err = exec.LookPath(str[0]); err != nil {
			return err
		}
		cmd := exec.CommandContext(ctx, str[0], str[1:]...)
		if !ex.List[i].Both {
			cmd.Stderr = s.stderr
		}
		cs = append(cs, cmd)
	}
	var (
		err error
		grp errgroup.Group
	)
	for i := 0; i < len(cs)-1; i++ {
		var (
			curr = cs[i]
			next = cs[i+1]
		)
		if next.Stdin, err = curr.StdoutPipe(); err != nil {
			return err
		}
		grp.Go(curr.Start)
		defer curr.Wait()
	}
	cmd := cs[len(cs)-1]
	cmd.Stdout = s.stdout
	grp.Go(func() error {
		err := cmd.Run()
		s.updateContext(cmd)
		return err
	})
	return grp.Wait()
}

func (s *Shell) executeAssign(ex ExecAssign) error {
	str, err := ex.Expand(s)
	if err != nil {
		return err
	}
	return s.Define(ex.Ident, str)
}

func (s *Shell) expand(ex Expander) ([]string, error) {
	str, err := ex.Expand(s)
	if err != nil {
		return nil, err
	}
	if len(str) == 0 {
		return nil, ErrEmpty
	}
	alias, ok := s.alias[str[0]]
	if ok {
		as := make([]string, len(alias))
		copy(as, alias)
		str = append(as, str[1:]...)
	}
	return str, nil
}

func (s *Shell) expandExecuter(ex Executer) ([][]string, error) {
	var (
		str [][]string
		err error
	)
	switch x := ex.(type) {
	case ExecSimple:
		xs, err1 := x.Expand(s)
		if err1 != nil {
			err = err1
			break
		}
		str = append(str, xs)
	case ExecAnd:
		left, err1 := s.expandExecuter(x.Left)
		if err1 != nil {
			err = err1
			break
		}
		right, err1 := s.expandExecuter(x.Right)
		if err1 != nil {
			err = err1
			break
		}
		str = append(str, left...)
		str = append(str, right...)
	case ExecOr:
		left, err1 := s.expandExecuter(x.Left)
		if err1 != nil {
			err = err1
			break
		}
		right, err1 := s.expandExecuter(x.Right)
		if err1 != nil {
			err = err1
			break
		}
		str = append(str, left...)
		str = append(str, right...)
	case ExecPipe:
		for i := range x.List {
			xs, err1 := s.expandExecuter(x.List[i].Executer)
			if err1 != nil {
				err = err1
				break
			}
			str = append(str, xs...)
		}
	default:
		err = fmt.Errorf("unknown/unsupported executer type %T", ex)
	}
	return str, err
}

func (s *Shell) setContext(name string, args []string) {
	s.context.name = name
	s.context.args = append(s.context.args[:0], args...)
}

func (s *Shell) updateContext(cmd *exec.Cmd) {
	s.context.pid = cmd.ProcessState.Pid()
	s.context.code = cmd.ProcessState.ExitCode()
}

func (s *Shell) clearContext() {
	s.context.name = ""
	s.context.args = nil
}

func (s *Shell) resolveSpecials(ident string) []string {
	var ret []string
	switch ident {
	case "SECONDS":
		sec := time.Since(s.now).Seconds()
		ret = append(ret, strconv.FormatInt(int64(sec), 10))
	case "PWD":
		cwd, err := os.Getwd()
		if err != nil {
			cwd = s.cwd
		}
		ret = append(ret, cwd)
	case "OLDPWD":
		// TODO
		ret = append(ret, "")
	case "PID", "$":
		str := strconv.Itoa(os.Getpid())
		ret = append(ret, str)
	case "PPID":
		str := strconv.Itoa(os.Getppid())
		ret = append(ret, str)
	case "RANDOM":
		// TODO
		ret = append(ret, "")
	case "PATH":
		// TODO
		ret = append(ret, "")
	case "0":
		ret = append(ret, s.context.name)
	case "#":
		ret = append(ret, strconv.Itoa(len(s.context.args)))
	case "?":
		ret = append(ret, strconv.Itoa(s.context.code))
	case "!":
		ret = append(ret, strconv.Itoa(s.context.pid))
	case "*":
		ret = append(ret, strings.Join(s.context.args, " "))
	case "@":
		ret = s.context.args
	default:
		n, err := strconv.Atoi(ident)
		if err != nil {
			break
		}
		var arg string
		if n >= 1 && n <= len(s.context.args) {
			arg = s.context.args[n-1]
		}
		ret = append(ret, arg)
	}
	return ret
}
