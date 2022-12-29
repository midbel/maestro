package maestro

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/midbel/maestro/internal/env"
	"github.com/midbel/maestro/internal/help"
)

const DefaultSSHPort = 22

type CommandSettings struct {
	Visible bool

	Name       string
	Alias      []string
	Short      string
	Desc       string
	Categories []string

	Retry   int64
	WorkDir string
	Timeout time.Duration

	Hosts   []string
	Deps    []CommandDep
	Options []CommandOption
	Args    []CommandArg
	Lines   CommandScript

	As map[string]string
	Ev map[string]string

	locals *env.Env
}

func NewCommmandSettings(name string) (CommandSettings, error) {
	return NewCommandSettingsWithLocals(name, env.Empty())
}

func NewCommandSettingsWithLocals(name string, locals *env.Env) (CommandSettings, error) {
	cmd := CommandSettings{
		Name:   name,
		locals: locals,
		Ev:     make(map[string]string),
		As:     make(map[string]string),
	}
	if cmd.locals == nil {
		cmd.locals = env.Empty()
	}
	return cmd, nil
}

func (s CommandSettings) Command() string {
	return s.Name
}

func (s CommandSettings) About() string {
	return s.Short
}

func (s CommandSettings) Help() (string, error) {
	return help.Command(s)
}

func (s CommandSettings) Tags() []string {
	if len(s.Categories) == 0 {
		return []string{"default"}
	}
	return s.Categories
}

func (s CommandSettings) Usage() string {
	var str strings.Builder
	str.WriteString(s.Name)
	for _, o := range s.Options {
		str.WriteString(" ")
		str.WriteString("[")
		if o.Short != "" {
			str.WriteString("-")
			str.WriteString(o.Short)
		}
		if o.Short != "" && o.Long != "" {
			str.WriteString("/")
		}
		if o.Long != "" {
			str.WriteString("--")
			str.WriteString(o.Long)
		}
		str.WriteString("]")
	}
	for _, a := range s.Args {
		str.WriteString(" ")
		str.WriteString("<")
		str.WriteString(a.Name)
		str.WriteString(">")
	}
	return str.String()
}

func (s CommandSettings) Blocked() bool {
	return !s.Visible
}

func (s CommandSettings) Remote() bool {
	return len(s.Hosts) > 0
}

type CommandScript []string

func (c CommandScript) Reader() io.Reader {
	var str bytes.Buffer
	for i := range c {
		if i > 0 {
			str.WriteString("\n")
		}
		str.WriteString(c[i])
	}
	return &str
}

type CommandDep struct {
	Name string
	Args []string
	// Bg        bool
	// Optional  bool
	// Mandatory bool
}

func (c CommandDep) Key() string {
	return c.Name
}

type CommandOption struct {
	Short    string
	Long     string
	Help     string
	Required bool
	Flag     bool

	Default     string
	DefaultFlag bool
	Target      string
	TargetFlag  bool

	Valid ValidateFunc
}

func (o CommandOption) Validate() error {
	if o.Flag {
		return nil
	}
	if o.Required && o.Target == "" {
		return fmt.Errorf("%s/%s: missing value", o.Short, o.Long)
	}
	if o.Valid == nil {
		return nil
	}
	return o.Valid(o.Target)
}

type CommandArg struct {
	Name  string
	Valid ValidateFunc
}

func (a CommandArg) Validate(arg string) error {
	if a.Valid == nil {
		return nil
	}
	return a.Valid(arg)
}
