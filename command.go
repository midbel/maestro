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
	About() string
	Help() (string, error)
	Tags() []string
	Command() string
	Combined() bool
}

type Dep struct {
	Name string
	Args []string
	Bg   bool
}

type Single struct {
	Name         string
	Short        string
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

func (s *Single) About() string {
	return s.Short
}

func (s *Single) Help() (string, error) {
	return renderTemplate(cmdhelp, s)
}

func (s *Single) Tags() []string {
	return s.Cats
}

func (_ *Single) Combined() bool {
	return false
}

func (s *Single) Execute(args []string) error {
	for i := range s.Scripts {
		fmt.Println(s.Scripts[i])
	}
	return nil
}

type Combined []Command

func (c Combined) Command() string {
	return c[0].Command()
}

func (c Combined) Tags() []string {
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

func (c Combined) About() string {
	return c[0].About()
}

func (c Combined) Help() (string, error) {
	return c[0].Help()
}

func (_ Combined) Combined() bool {
	return true
}

func (c Combined) Execute(args []string) error {
	return nil
}
