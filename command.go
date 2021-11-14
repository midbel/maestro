package maestro

import (
	"fmt"
	"strings"
	"time"
)

const (
	errSilent = "silent"
	errRaise  = "raise"
)

type Command interface {
	Execute([]string) error
	Help() string
	Tags() []string
	Command() string
}

type Dep struct {
	Name string
	Args []string
	Bg   bool
}

type Single struct {
	Name         string
	Desc         string
	Usage        string
	Error        string
	Cats         []string
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

func (s *Single) Command() string {
	return s.Name
}

func (s *Single) Help() string {
	return s.Desc
}

func (s *Single) Tags() []string {
	return s.Cats
}

func (s *Single) Execute(args []string) error {
	for i := range s.Scripts {
		fmt.Println(s.Scripts[i])
	}
	return nil
}

type CombinedCommand []Command

func (c CombinedCommand) Command() string {
	return c[0].Command()
}

func (c CombinedCommand) Tags() []string {
	var (
		tags []string
		seen = make(map[string]struct{})
	)
	for i := range c {
		for _, t := range c[i].Tags() {
			if _, ok := seen[t]; ok {
				continue
			}
			seen[t] = struct{}{}
			tags = append(tags, t)
		}
	}
	return tags
}

func (c CombinedCommand) Help() string {
	var str strings.Builder
	for i := range c {
		if i > 0 {
			str.WriteRune('\n')
		}
		str.WriteString(c[i].Help())
	}
	return str.String()
}

func (c CombinedCommand) Execute(args []string) error {
	return nil
}
