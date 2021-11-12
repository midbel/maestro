package maestro

import (
	"errors"
	"fmt"
)

var ErrDuplicate = errors.New("command already registered")

const (
	dupError   = "error"
	dupAppend  = "append"
	dupReplace = "replace"
)

type Maestro struct {
	MetaExec
	MetaAbout
	MetaSSH

	Duplicate string
	Commands  map[string]Command
}

func Load(file string) (*Maestro, error) {
	return nil, nil
}

func (m *Maestro) ExecuteDefault() error {
	return nil
}

func (m *Maestro) ExecuteAll() error {
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
