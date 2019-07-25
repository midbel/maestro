package maestro

import (
	"io"
)

type Command interface {
	Run(io.Writer, io.Writer) error
	Start(io.Writer, io.Writer) error
	Wait() error
}

// Commmand for local execution (via exec.Cmd)
type local struct {}

// Command for remote execution (via ssh.Session)
type remote struct {}

// Chain executes the final chain of Command
func Chain(cs ...Command) error {
	return nil
}
