package shell

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"os/signal"
	"os/user"
	"strconv"
	"strings"
	"time"

	"github.com/midbel/maestro/shlex"
	"golang.org/x/sync/errgroup"
)

const shell = "tish"

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

var specials = map[string]struct{}{
	"HOME":    {},
	"SECONDS": {},
	"PWD":     {},
	"OLDPWD":  {},
	"PID":     {},
	"PPID":    {},
	"RANDOM":  {},
	"SHELL":   {},
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

	env map[string]string

	cwd  string
	old  string
	now  time.Time
	rand *rand.Rand

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
		old:      "..",
		alias:    make(map[string][]string),
		commands: make(map[string]Command),
		env:      make(map[string]string),
		builtins: builtins,
	}
	s.rand = rand.New(rand.NewSource(s.now.Unix()))
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

func (s *Shell) SetOut(w io.Writer) {
	s.stdout = w
}

func (s *Shell) SetErr(w io.Writer) {
	s.stderr = w
}

func (s *Shell) Chdir(dir string) error {
	if dir == "" {
		return nil
	}
	err := os.Chdir(dir)
	if err == nil {
		s.old = s.cwd
		s.cwd = dir
	}
	return err
}

func (s *Shell) SetEcho(echo bool) {
	s.echo = echo
}

func (s *Shell) Export(ident, value string) {
	s.env[ident] = value
}

func (s *Shell) Unexport(ident string) {
	delete(s.env, ident)
}

func (s *Shell) Alias(ident, script string) error {
	alias, err := shlex.Split(strings.NewReader(script))
	if err != nil {
		return err
	}
	s.alias[ident] = alias
	return nil
}

func (s *Shell) Unalias(ident string) {
	delete(s.alias, ident)
}

func (s *Shell) Subshell() (*Shell, error) {
	options := []ShellOption{
		WithEnv(s),
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
	str, err := s.locals.Resolve(ident)
	if err == nil && len(str) > 0 {
		return str, nil
	}
	if v, ok := s.env[ident]; ok {
		return []string{v}, nil
	}
	if str = s.resolveSpecials(ident); len(str) > 0 {
		return str, nil
	}
	return nil, err
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

func (s *Shell) Expand(str string, args []string) ([]string, error) {
	var (
		p   = NewParser(strings.NewReader(str))
		ret []string
	)
	for {
		ex, err := p.Parse()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		str, err := s.expandExecuter(ex)
		if err != nil {
			continue
		}
		var lines []string
		for i := range str {
			lines = append(lines, strings.Join(str[i], " "))
		}
		ret = append(ret, strings.Join(lines, "; "))
	}
	return ret, nil
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
		err = s.executeSingle(ex.Expander, ex.Redirect)
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

func (s *Shell) executeSingle(ex Expander, redirect []ExpandRedirect) error {
	str, err := s.expand(ex)
	if err != nil {
		return err
	}
	s.trace(str)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		defer cancel()

		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Kill, os.Interrupt)
		<-sig
	}()
	cmd := s.resolveCommand(ctx, str)

	rd, err := s.setupRedirect(redirect, false)
	if err != nil {
		return err
	}
	defer rd.Close()

	cmd.SetOut(rd.out)
	cmd.SetErr(rd.err)
	cmd.SetIn(rd.in)

	err = cmd.Run()
	s.updateContext(cmd)
	return err
}

func (s *Shell) executePipe(ex ExecPipe) error {
	var (
		cs          []Command
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
		cmd := s.resolveCommand(ctx, str)
		rd, err := s.setupRedirect(sex.Redirect, true)
		if err != nil {
			return err
		}
		defer rd.Close()

		cmd.SetOut(rd.out)
		cmd.SetIn(rd.in)
		cmd.SetErr(rd.err)

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
			in   io.ReadCloser
		)
		if in, err = curr.StdoutPipe(); err != nil {
			return err
		}
		next.SetIn(in)
		grp.Go(curr.Start)
	}
	cmd := cs[len(cs)-1]
	cmd.SetOut(s.stdout)
	grp.Go(func() error {
		err := cmd.Run()
		s.updateContext(cmd)
		return err
	})
	return grp.Wait()
}

func (s *Shell) executeAssign(ex ExecAssign) error {
	str, err := ex.Expand(s, true)
	if err != nil {
		return err
	}
	return s.Define(ex.Ident, str)
}

func (s *Shell) expand(ex Expander) ([]string, error) {
	str, err := ex.Expand(s, true)
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
		xs, err1 := x.Expand(s, true)
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

func (s *Shell) updateContext(cmd Command) {
	pid, code := cmd.Exit()
	s.context.pid = pid
	s.context.code = code
}

func (s *Shell) clearContext() {
	s.context.name = ""
	s.context.args = nil
}

func (s *Shell) resolveCommand(ctx context.Context, str []string) Command {
	var cmd Command
	if b, ok := s.builtins[str[0]]; ok && b.IsEnabled() {
		b.shell = s
		b.args = str[1:]
		cmd = &b
	} else {
		cmd = StandardContext(ctx, str[0], str[1:])
		if e, ok := cmd.(interface{ SetEnv([]string) }); ok {
			e.SetEnv(s.environ())
		}
	}
	return cmd
}

func (s *Shell) resolveSpecials(ident string) []string {
	var ret []string
	switch ident {
	case "SHELL":
		ret = append(ret, shell)
	case "HOME":
		u, err := user.Current()
		if err == nil {
			ret = append(ret, u.HomeDir)
		}
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
		ret = append(ret, s.old)
	case "PID", "$":
		str := strconv.Itoa(os.Getpid())
		ret = append(ret, str)
	case "PPID":
		str := strconv.Itoa(os.Getppid())
		ret = append(ret, str)
	case "RANDOM":
		str := strconv.Itoa(s.rand.Int())
		ret = append(ret, str)
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

func (s *Shell) trace(str []string) {
	if !s.echo {
		return
	}
	fmt.Fprintln(s.stdout, strings.Join(str, " "))
}

func (s *Shell) environ() []string {
	var str []string
	for n, v := range s.env {
		str = append(str, fmt.Sprintf("%s=%s", n, v))
	}
	return str
}

type noopCloseReader struct {
	io.Reader
}

func noopReadCloser(r io.Reader) io.ReadCloser {
	return noopCloseReader{
		Reader: r,
	}
}

func (_ noopCloseReader) Close() error {
	return nil
}

type noopCloseWriter struct {
	io.Writer
}

func noopWriteCloser(w io.Writer) io.WriteCloser {
	return noopCloseWriter{
		Writer: w,
	}
}

func (_ noopCloseWriter) Close() error {
	return nil
}

const (
	flagRead   = os.O_CREATE | os.O_RDONLY
	flagWrite  = os.O_CREATE | os.O_WRONLY
	flagAppend = os.O_CREATE | os.O_WRONLY | os.O_APPEND
)

func replaceFile(file string, flag int, list ...*os.File) (*os.File, error) {
	fd, err := os.OpenFile(file, flag, 0644)
	if err != nil {
		return nil, err
	}
	for i := range list {
		if list[i] == nil {
			continue
		}
		list[i].Close()
	}
	return fd, nil
}

type redirect struct {
	in  io.ReadCloser
	out io.WriteCloser
	err io.WriteCloser
}

func (r redirect) Close() error {
	for _, c := range []io.Closer{r.in, r.out, r.err} {
		if c == nil {
			continue
		}
		c.Close()
	}
	return nil
}

func (s *Shell) setupRedirect(rs []ExpandRedirect, pipe bool) (redirect, error) {
	var (
		stdin  *os.File
		stdout *os.File
		stderr *os.File
		rd     redirect
	)
	for _, r := range rs {
		str, err := r.Expand(s, true)
		if err != nil {
			return rd, err
		}
		switch file := str[0]; r.Type {
		case RedirectIn:
			stdin, err = replaceFile(file, flagRead, stdin)
		case RedirectOut:
			if stdout == stderr {
				stdout = nil
			}
			stdout, err = replaceFile(file, flagWrite, stdout)
		case RedirectErr:
			if stderr == stdout {
				stderr = nil
			}
			stderr, err = replaceFile(file, flagWrite, stderr)
		case RedirectBoth:
			var fd *os.File
			if fd, err = replaceFile(file, flagWrite, stdout, stderr); err == nil {
				stdout, stderr = fd, fd
			}
		case AppendOut:
			if stdout.Fd() == stderr.Fd() {
				stdout = nil
			}
			stdout, err = replaceFile(file, flagAppend, stdout)
		case AppendErr:
			if stderr == stdout {
				stderr = nil
			}
			stderr, err = replaceFile(file, flagAppend, stderr)
		case AppendBoth:
			var fd *os.File
			if fd, err = replaceFile(file, flagAppend, stdout, stderr); err == nil {
				stdout, stderr = fd, fd
			}
		default:
			err = fmt.Errorf("unknown/unsupported redirection")
		}
		if err != nil {
			return rd, err
		}
	}
	rd.in = fileOrReader(stdin, s.stdin, pipe)
	rd.out = fileOrWriter(stdout, s.stdout, pipe)
	rd.err = fileOrWriter(stderr, s.stderr, pipe)
	return rd, nil
}

func fileOrWriter(f *os.File, w io.Writer, pipe bool) io.WriteCloser {
	if f == nil {
		if pipe {
			return nil
		}
		return noopWriteCloser(w)
	}
	return f
}

func fileOrReader(f *os.File, r io.Reader, pipe bool) io.ReadCloser {
	if f == nil {
		if pipe {
			return nil
		}
		return noopReadCloser(r)
	}
	return f
}
