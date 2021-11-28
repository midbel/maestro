package shell

import (
	"context"
	"io"
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
	c.Stdin = r
}

func (c *StdCmd) SetOut(w io.Writer) {
	c.Stdout = w
}

func (c *StdCmd) SetErr(w io.Writer) {
	c.Stderr = w
}

func (c *StdCmd) Exit() (int, int) {
	var (
		pid  = c.ProcessState.Pid()
		code = c.ProcessState.ExitCode()
	)
	return pid, code
}
