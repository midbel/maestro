package shell

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
)

type Command interface {
	Command() string
	Type() CommandType

	StdinPipe() (io.WriteCloser, error)
	StdoutPipe() (io.ReadCloser, error)
	StderrPipe() (io.ReadCloser, error)

	SetIn(r io.Reader)
	SetOut(w io.Writer)
	SetErr(w io.Writer)

	Run() error
	Start() error
	Wait() error
	Exit() (int, int)
}

type StdCmd struct {
	*exec.Cmd
	name string
}

func StandardContext(ctx context.Context, name string, args []string) Command {
	c := exec.CommandContext(ctx, name, args...)
	return &StdCmd{
		Cmd:  c,
		name: name,
	}
}

func Standard(name string, args []string) Command {
	c := exec.Command(name, args...)
	return &StdCmd{
		Cmd:  c,
		name: name,
	}
}

func (c *StdCmd) Command() string {
	return c.name
}

func (_ *StdCmd) Type() CommandType {
	return TypeRegular
}

func (c *StdCmd) SetIn(r io.Reader) {
	if r, ok := r.(noopCloseReader); ok {
		if f, ok := r.Reader.(*os.File); ok {
			c.Stdin = f
			return
		}
	}
	c.Stdin = r
}

func (c *StdCmd) SetOut(w io.Writer) {
	if w, ok := w.(noopCloseWriter); ok {
		if f, ok := w.Writer.(*os.File); ok {
			c.Stdout = f
			return
		}
	}
	c.Stdout = w
}

func (c *StdCmd) SetErr(w io.Writer) {
	if w, ok := w.(noopCloseWriter); ok {
		if f, ok := w.Writer.(*os.File); ok {
			c.Stderr = f
			return
		}
	}
	c.Stderr = w
}

func (c *StdCmd) Exit() (int, int) {
	if c == nil || c.Cmd == nil || c.Cmd.ProcessState == nil {
		return 0, 255
	}
	var (
		pid  = c.ProcessState.Pid()
		code = c.ProcessState.ExitCode()
	)
	return pid, code
}

func (c *StdCmd) SetEnv(env []string) {
	c.Cmd.Env = append(c.Cmd.Env[:0], env...)
}

type scriptCmd struct {
	name  string
	lines []string
	args  []string

	shell    *Shell
	finished bool
	code     int
	done     chan error

	stdout io.Writer
	stderr io.Writer
	stdin  io.Reader

	closes []io.Closer
	copies []func() error
	errch  chan error
}

func Script(name string, lines, args []string) Command {
	return &scriptCmd{
		name:  name,
		lines: lines,
		args:  args,
	}
}

func (s *scriptCmd) Command() string {
	return s.name
}

func (_ *scriptCmd) Type() CommandType {
	return TypeScript
}

func (_ *scriptCmd) Exit() (int, int) {
	return 0, 0
}

func (s *scriptCmd) Start() error {
	if s.finished {
		return fmt.Errorf("builtin already executed")
	}
	setupfd := []func() (*os.File, error){
		s.setStdin,
		s.setStdout,
		s.setStderr,
	}
	for _, set := range setupfd {
		_, err := set()
		if err != nil {
			s.closeDescriptors()
			return err
		}
	}
	if len(s.copies) > 0 {
		s.errch = make(chan error, 3)
		for _, fn := range s.copies {
			go func(fn func() error) {
				s.errch <- fn()
			}(fn)
		}
	}
	s.done = make(chan error, 1)
	go func() {
		s.done <- s.execute()
	}()
	return nil
}

func (s *scriptCmd) Wait() error {
	if s.finished {
		return fmt.Errorf("script already finished")
	}
	s.finished = true

	var (
		errex = <-s.done
		errcp error
	)
	defer close(s.done)
	s.closeDescriptors()
	for range s.copies {
		e := <-s.errch
		if errcp == nil && e != nil {
			s.code = 2
			errcp = e
		}
	}
	if errex != nil {
		s.code = 1
		return errex
	}
	return errcp
}

func (s *scriptCmd) Run() error {
	if err := s.Start(); err != nil {
		return err
	}
	return s.Wait()
}

func (s *scriptCmd) SetIn(r io.Reader) {
	s.stdin = r
}

func (s *scriptCmd) SetOut(w io.Writer) {
	s.stdout = w
}

func (s *scriptCmd) SetErr(w io.Writer) {
	s.stderr = w
}

func (s *scriptCmd) StdoutPipe() (io.ReadCloser, error) {
	if s.stdout != nil {
		return nil, fmt.Errorf("stdout already set")
	}
	if s.finished {
		return nil, fmt.Errorf("stdout after builtin started")
	}
	pr, pw, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	s.closes = append(s.closes, pr, pw)
	s.stdout = pw
	return pr, nil
}

func (s *scriptCmd) StderrPipe() (io.ReadCloser, error) {
	if s.stderr != nil {
		return nil, fmt.Errorf("stderr already set")
	}
	if s.finished {
		return nil, fmt.Errorf("stderr after builtin started")
	}
	pr, pw, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	s.closes = append(s.closes, pr, pw)
	s.stderr = pw
	return pr, nil
}

func (s *scriptCmd) StdinPipe() (io.WriteCloser, error) {
	if s.stdin != nil {
		return nil, fmt.Errorf("stdin already set")
	}
	if s.shell != nil {
		return nil, fmt.Errorf("stdin after builtin started")
	}
	pr, pw, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	s.closes = append(s.closes, pr, pw)
	s.stdin = pr
	return pw, nil
}

func (s *scriptCmd) execute() error {
	// for _, i := range s.lines {
	// 	if err := s.shell.Execute(i, s.name, s.args); err != nil {
	// 		return err
	// 	}
	// }
	return nil
}

func (s *scriptCmd) setStdin() (*os.File, error) {
	if s.stdin == nil {
		f, err := os.Open(os.DevNull)
		if err != nil {
			return nil, err
		}
		s.closes = append(s.closes, f)
		return f, nil
	}
	switch r := s.stdin.(type) {
	case *os.File:
		return r, nil
	case noopCloseReader:
		f, ok := r.Reader.(*os.File)
		if ok {
			return f, nil
		}
	}
	pr, pw, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	s.closes = append(s.closes, pw)
	s.copies = append(s.copies, func() error {
		defer pw.Close()
		_, err := io.Copy(pw, s.stdin)
		return err
	})
	return pr, nil
}

func (s *scriptCmd) setStdout() (*os.File, error) {
	return s.openFile(s.stdout)
}

func (s *scriptCmd) setStderr() (*os.File, error) {
	return s.openFile(s.stderr)
}

func (s *scriptCmd) openFile(w io.Writer) (*os.File, error) {
	if w == nil {
		f, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
		if err != nil {
			return nil, err
		}
		s.closes = append(s.closes, f)
		return f, nil
	}
	switch w := w.(type) {
	case *os.File:
		return w, nil
	case noopCloseWriter:
		f, ok := w.Writer.(*os.File)
		if ok {
			return f, nil
		}
	default:
	}

	pr, pw, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	s.closes = append(s.closes, pw)
	s.copies = append(s.copies, func() error {
		defer pr.Close()
		_, err := io.Copy(w, pr)
		return err
	})
	return pw, nil
}

func (s *scriptCmd) closeDescriptors() {
	for _, c := range s.closes {
		c.Close()
	}
	s.closes = s.closes[:0]
}
