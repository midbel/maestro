package maestro

type Maestro struct {
	MetaExec
	MetaAbout
	MetaSSH

	Commands map[string]*Command
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

type MetaExec struct {
  WorkDir  string
  
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
