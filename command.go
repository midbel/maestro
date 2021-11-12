package maestro

import (
	"time"
)

const (
	errSilent = "silent"
	errRaise  = "raise"
)

type Command interface {
	Execute([]string) error
}

type Dep struct {
	Name string
	Args []string
	Bg   bool
}

type Single struct {
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
}

func NewSingle(name string) *Single {
	return NewSingleWithLocals(name, nil)
}

func NewSingleWithLocals(name string, locals map[string][]string) *Single {
	if locals == nil {
		locals = make(map[string][]string)
	}
	cmd := Single{
		Name:    name,
		Options: make(map[string]string),
		Locals:  make(map[string][]string),
	}
	for k := range locals {
		cmd.Locals[k] = append(cmd.Locals[k], locals[k]...)
	}
	return &cmd
}

func (s *Single) Execute(args []string) error {
	return nil
}

type CombinedCommand []Command

func (c CombinedCommand) Execute(args []string) error {
	return nil
}
