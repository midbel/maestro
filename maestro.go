package maestro

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

var ErrDuplicate = errors.New("command already registered")

const (
	dupError   = "error"
	dupAppend  = "append"
	dupReplace = "replace"
)

//go:embed templates/help.gotpl
var helptext string

type Maestro struct {
	MetaExec
	MetaAbout
	MetaSSH

	Duplicate string
	Commands  map[string]Command
	Alias     map[string]string
}

func New() *Maestro {
	return &Maestro{
		Duplicate: dupReplace,
		Commands:  make(map[string]Command),
		Alias:     make(map[string]string),
	}
}

func Load(file string) (*Maestro, error) {
	r, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	m, err := Decode(r)
	if err != nil {
		return nil, err
	}
	m.MetaAbout.File = file
	return m, nil
}

func (m *Maestro) Execute(name string, args []string) error {
	cmd, err := m.lookup(name)
	if err != nil {
		return err
	}
	return cmd.Execute(args)
}

func (m *Maestro) ExecuteHelp(name string) error {
	var help string
	if name != "" {
		cmd, err := m.lookup(name)
		if err != nil {
			return err
		}
		help = cmd.Help()
	} else {
		h, err := m.help()
		if err != nil {
			return err
		}
		help = h
	}
	fmt.Println(strings.TrimSpace(help))
	return nil
}

func (m *Maestro) ExecuteVersion() error {
	fmt.Printf("%s %s", m.Name(), m.Version)
	fmt.Println()
	return nil
}

func (m *Maestro) ExecuteDefault(args []string) error {
	if m.MetaExec.Default == "" {
		return fmt.Errorf("no default command defined")
	}
	return m.Execute(m.MetaExec.Default, args)
}

func (m *Maestro) ExecuteAll(_ []string) error {
	if len(m.MetaExec.All) == 0 {
		return fmt.Errorf("no all command defined")
	}
	for _, n := range m.MetaExec.All {
		if err := m.Execute(n, nil); err != nil {
			return err
		}
	}
	return nil
}

func (m *Maestro) Register(cmd *Single) error {
	curr, ok := m.Commands[cmd.Name]
	if !ok {
		m.Commands[cmd.Name] = cmd
		return nil
	}
	switch m.Duplicate {
	case dupError:
		return fmt.Errorf("%s %w", cmd.Name, ErrDuplicate)
	case dupReplace, "":
		m.Commands[cmd.Name] = cmd
	case dupAppend:
		if mul, ok := curr.(CombinedCommand); ok {
			curr = append(mul, cmd)
			break
		}
		mul := make(CombinedCommand, 0, 2)
		mul = append(mul, curr)
		mul = append(mul, cmd)
		m.Commands[cmd.Name] = mul
	default:
		return fmt.Errorf("DUPLICATE: unknown value %s", m.Duplicate)
	}
	return nil
}

func (m *Maestro) help() (string, error) {
	h := help{
		Version:  m.Version,
		File:     m.Name(),
		Usage:    m.Usage,
		Help:     m.Help,
		Commands: make(map[string][]Command),
	}
	for _, c := range m.Commands {
		for _, t := range c.Tags() {
			h.Commands[t] = append(h.Commands[t], c)
		}
	}
	t, err := template.New("help").Parse(helptext)
	if err != nil {
		return "", err
	}
	var str strings.Builder
	if err := t.Execute(&str, h); err != nil {
		return "", err
	}
	return str.String(), nil
}

func (m *Maestro) Name() string {
	return strings.TrimSuffix(filepath.Base(m.File), filepath.Ext(m.File))
}

func (m *Maestro) lookup(name string) (Command, error) {
	cmd, ok := m.Commands[name]
	if !ok {
		return nil, fmt.Errorf("%s: command not defined")
	}
	return cmd, nil
}

type MetaExec struct {
	WorkDir string

	Path     []string
	Echo     bool
	Parallel int64

	All     []string
	Default string
	Before  []string
	After   []string
	Error   []string
	Success []string
}

type MetaAbout struct {
	File    string
	Author  string
	Email   string
	Version string
	Help    string
	Usage   string
}

type MetaSSH struct {
	User       string
	Pass       string
	PublicKey  string
	PrivateKey string
}

type help struct {
	File     string
	Help     string
	Usage    string
	Version  string
	Commands map[string][]Command
}
