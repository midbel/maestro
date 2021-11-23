package maestro

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var ErrDuplicate = errors.New("command already registered")

const (
	dupError   = "error"
	dupAppend  = "append"
	dupReplace = "replace"
)

const (
	defaultFile    = "maestro.mf"
	defaultVersion = "0.1.0"
)

type Maestro struct {
	MetaExec
	MetaAbout
	MetaSSH

	Duplicate string
	Commands  map[string]Command
	Alias     map[string]string
}

func New() *Maestro {
	about := MetaAbout{
		File:    defaultFile,
		Version: defaultVersion,
	}
	return &Maestro{
		MetaAbout: about,
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

func (m *Maestro) Dry(name string, args []string) error {
	cmd, err := m.lookup(name)
	if err != nil {
		return err
	}
	return cmd.Dry(args)
}

func (m *Maestro) Execute(name string, args []string) error {
	cmd, err := m.lookup(name)
	if err != nil {
		return err
	}
	return cmd.Execute(args)
}

func (m *Maestro) ExecuteHelp(name string, verbose bool) error {
	var (
		help string
		err  error
	)
	if name != "" {
		cmd, err := m.lookup(name)
		if err != nil {
			return err
		}
		help, err = cmd.Help()
	} else {
		help, err = m.help()
	}
	if err == nil {
		fmt.Fprintln(os.Stdout, strings.TrimSpace(help))
	}
	return err
}

func (m *Maestro) ExecuteVersion() error {
	fmt.Fprintf(os.Stdout, "%s %s", m.Name(), m.Version)
	fmt.Fprintln(os.Stdout)
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
		if mul, ok := curr.(Combined); ok {
			curr = append(mul, cmd)
			break
		}
		mul := make(Combined, 0, 2)
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
	return renderTemplate(helptext, h)
}

func (m *Maestro) Name() string {
	return strings.TrimSuffix(filepath.Base(m.File), filepath.Ext(m.File))
}

func (m *Maestro) lookup(name string) (Command, error) {
	if name == "" {
		name = m.MetaExec.Default
	}
	cmd, ok := m.Commands[name]
	if !ok {
		return nil, fmt.Errorf("%s: command not defined", name)
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
