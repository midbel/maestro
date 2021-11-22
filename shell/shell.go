package shell

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"
)

var (
	ErrReadOnly = errors.New("read only")
	ErrEmpty    = errors.New("empty command")
)

type ShellOption func(*Shell) error

func WithEcho() ShellOption {
	return func(s *Shell) error {
		s.echo = true
		return nil
	}
}

func WithVar(ident string, values ...string) ShellOption {
	return func(s *Shell) error {
		if s.locals == nil {
			s.locals = EmptyEnv()
		}
		return s.locals.Define(ident, values)
	}
}

func WithAlias(ident, script string) ShellOption {
	return func(s *Shell) error {
		return s.Alias(ident, script)
	}
}

func WithCwd(dir string) ShellOption {
	return func(s *Shell) error {
		if dir == "" {
			return nil
		}
		err := os.Chdir(dir)
		if err == nil {
			s.cwd = dir
		}
		return err
	}
}

func WithEnv(e Environment) ShellOption {
	return func(s *Shell) error {
		if s.locals == nil {
			s.locals = e
		} else {
			s.locals = EnclosedEnv(e)
		}
		return nil
	}
}

var specials = map[string]struct{}{
	"SECONDS": {},
	"PWD":     {},
	"OLDPWD":  {},
	"PID":     {},
	"PPID":    {},
	"RANDOM":  {},
	"PATH":    {},
}

type Shell struct {
	locals   Environment
	alias    map[string][]string
	builtins map[string]string
	echo     bool
	cwd      string
	now      time.Time
}

func New(options ...ShellOption) (*Shell, error) {
	s := Shell{
		now:      time.Now(),
		cwd:      ".",
		alias:    make(map[string][]string),
		builtins: make(map[string]string),
	}
	for i := range options {
		if err := options[i](&s); err != nil {
			return nil, err
		}
	}
	if s.locals == nil {
		s.locals = EmptyEnv()
	}
	return &s, nil
}

func (s *Shell) Alias(ident, script string) error {
	return nil
}

func (s *Shell) Unalias(ident string) {

}

func (s *Shell) Subshell() *Shell {
	return nil
}

func (s *Shell) Resolve(ident string) ([]string, error) {
	switch ident {
	case "SECONDS":
		var (
			sec = time.Since(s.now).Seconds()
			str = strconv.FormatInt(int64(sec), 10)
		)
		return []string{str}, nil
	case "PWD":
		cwd, err := os.Getwd()
		if err != nil {
			cwd = s.cwd
		}
		return []string{cwd}, nil
	case "OLDPWD":
	case "PID":
		var (
			pid = os.Getpid()
			str = strconv.Itoa(pid)
		)
		return []string{str}, nil
	case "PPID":
		var (
			pid = os.Getppid()
			str = strconv.Itoa(pid)
		)
		return []string{str}, nil
	case "RANDOM":
	case "PATH":
	default:
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

func (s *Shell) Execute(str string) error {
	var (
		p = NewParser(strings.NewReader(str))
		ret error
	)
	for {
		ex, err := p.Parse()
		if err != nil {
			if errors.Is(err, io.EOF) {
				err = nil
			}
			break
		}
		ret = s.execute(ex)
	}
	return ret
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
	if s.isBuiltin(str[0]) {
		return s.executeBuiltin(str)
	}
	if _, err := exec.LookPath(str[0]); err != nil {
		return err
	}
	cmd := exec.Command(str[0], str[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (s *Shell) executePipe(ex ExecPipe) error {
	var cs []*exec.Cmd
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
		cmd := exec.Command(str[0], str[1:]...)
		if !ex.List[i].Both {
			cmd.Stderr = os.Stderr
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
	last := cs[len(cs)-1]
	last.Stdout = os.Stdout
	grp.Go(last.Run)
	return grp.Wait()
}

func (s *Shell) executeAssign(ex ExecAssign) error {
	str, err := ex.Expand(s)
	if err != nil {
		return err
	}
	return s.Define(ex.Ident, str)
}

func (s *Shell) executeBuiltin(str []string) error {
	return nil
}

func (s *Shell) isBuiltin(ident string) bool {
	_, ok := s.builtins[ident]
	return ok
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
