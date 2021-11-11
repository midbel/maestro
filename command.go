package maestro

import (
	"time"
)

type Dep struct {
	Name string
	Args []string
	Bg   bool
}

type Command struct {
	Name         string
	Help         string
	Usage        string
  Error        string
	Tags         []string
	Retry        int64
	WorkDir      string
	Timeout      time.Duration
	Hosts        []string
	Dependencies []Dep
	Scripts      []string
	Env          map[string]string
	Locals       map[string][]string
	Options      map[string]string
	Meta         map[string][]string
}

func NewCommand(name string) *Command {
	return NewCommandWithLocals(name, nil)
}

func NewCommandWithLocals(name string, locals map[string][]string) *Command {
	if locals == nil {
		locals = make(map[string][]string)
	}
	cmd := Command{
		Name:    name,
		Options: make(map[string]string),
		Meta:    make(map[string][]string),
		Locals:  make(map[string][]string),
	}
	for k := range locals {
		cmd.Locals[k] = append(cmd.Locals[k], locals[k]...)
	}
	return &cmd
}

func (c *Command) Execute(args []string) error {
	return nil
}
