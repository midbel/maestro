package maestro

import (
	"fmt"
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
	Options      map[string]string
	Locals       *Env
}

func NewSingle(name string) *Single {
	return NewSingleWithLocals(name, nil)
}

func NewSingleWithLocals(name string, locals *Env) *Single {
	if locals == nil {
		locals = EmptyEnv()
	}
	cmd := Single{
		Name:    name,
		Options: make(map[string]string),
		Locals:  locals,
	}
	return &cmd
}

func (s *Single) Execute(args []string) error {
	for i := range s.Scripts {
		fmt.Println(s.Scripts[i])
	}
	return nil
}

type CombinedCommand []Command

func (c CombinedCommand) Execute(args []string) error {
	return nil
}
