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

type StdPipe struct {
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer

	closes []io.Closer
	copies []func() error
}

func (s *StdPipe) SetupFd() []func() (*os.File, error) {
	return []func() (*os.File, error){
		s.setStdin,
		s.setStdout,
		s.setStderr,
	}
}

func (s *StdPipe) Copies() []func() error {
	return s.copies
}

func (s *StdPipe) SetIn(r io.Reader) {
	s.stdin = r
}

func (s *StdPipe) SetOut(w io.Writer) {
	s.stdout = w
}

func (s *StdPipe) SetErr(w io.Writer) {
	s.stderr = w
}

func (s *StdPipe) StdoutPipe() (io.ReadCloser, error) {
	if s.stdout != nil {
		return nil, fmt.Errorf("stdout already set")
	}
	pr, pw, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	s.closes = append(s.closes, pr, pw)
	s.stdout = pw
	return pr, nil
}

func (s *StdPipe) StderrPipe() (io.ReadCloser, error) {
	if s.stderr != nil {
		return nil, fmt.Errorf("stderr already set")
	}
	pr, pw, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	s.closes = append(s.closes, pr, pw)
	s.stderr = pw
	return pr, nil
}

func (s *StdPipe) StdinPipe() (io.WriteCloser, error) {
	if s.stdin != nil {
		return nil, fmt.Errorf("stdin already set")
	}
	pr, pw, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	s.closes = append(s.closes, pr, pw)
	s.stdin = pr
	return pw, nil
}

func (s *StdPipe) setStdin() (*os.File, error) {
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

func (s *StdPipe) setStdout() (*os.File, error) {
	return s.openFile(s.stdout)
}

func (s *StdPipe) setStderr() (*os.File, error) {
	return s.openFile(s.stderr)
}

func (s *StdPipe) openFile(w io.Writer) (*os.File, error) {
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

func (s *StdPipe) Close() error {
	for _, c := range s.closes {
		c.Close()
	}
	s.closes = s.closes[:0]
	return nil
}
