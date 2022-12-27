package maestro

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/midbel/distance"
	"github.com/midbel/maestro/internal/env"
)

const (
	CmdHelp     = "help"
	CmdVersion  = "version"
	CmdAll      = "all"
	CmdDefault  = "default"
	CmdListen   = "listen"
	CmdServe    = "serve"
	CmdGraph    = "graph"
	CmdSchedule = "schedule"
)

const (
	DefaultFile     = "maestro.mf"
	DefaultVersion  = "0.1.0"
	DefaultHttpAddr = ":9090"
)

type Maestro struct {
	MetaExec
	MetaAbout
	MetaSSH
	MetaHttp

	Locals   *env.Env
	Commands Registry

	Remote     bool
	NoDeps     bool
	WithPrefix bool
}

func New() *Maestro {
	about := MetaAbout{
		File:    DefaultFile,
		Version: DefaultVersion,
	}
	mhttp := MetaHttp{
		Addr: DefaultHttpAddr,
	}
	return &Maestro{
		Locals:    env.EmptyEnv(),
		MetaAbout: about,
		MetaHttp:  mhttp,
		Commands:  make(Registry),
	}
}

func (m *Maestro) Name() string {
	return strings.TrimSuffix(filepath.Base(m.File), filepath.Ext(m.File))
}

func (m *Maestro) Load(file string) error {
	r, err := os.Open(file)
	if err != nil {
		return err
	}
	defer r.Close()

	d, err := NewDecoderWithEnv(r, m.Locals)
	if err != nil {
		return err
	}
	if err := d.decode(m); err != nil {
		return err
	}
	m.MetaAbout.File = file
	return nil
}

func (m *Maestro) Register(cmd CommandSettings) error {
	_, ok := m.Commands[cmd.Name]
	if !ok {
		m.Commands[cmd.Name] = cmd
		return nil
	}
	return fmt.Errorf("%s command already registered", cmd.Name)
}

func (m *Maestro) ExecuteHelp(name string) error {
	return m.executeHelp(name, os.Stdout)
}

func (m *Maestro) ExecuteVersion() error {
	return m.executeVersion(os.Stdout)
}

func (m *Maestro) executeHelp(name string, w io.Writer) error {
	var (
		help string
		err  error
	)
	if name != "" {
		cmd, err := m.Commands.Lookup(name)
		if err != nil {
			return err
		}
		help, err = cmd.Help()
	} else {
		help, err = m.help()
	}
	if err == nil {
		fmt.Fprintln(w, strings.TrimSpace(help))
	}
	return err
}

func (m *Maestro) executeVersion(w io.Writer) error {
	fmt.Fprintf(w, "%s %s", m.Name(), m.Version)
	fmt.Fprintln(w)
	return nil
}

func (m *Maestro) help() (string, error) {
	return "", nil
}

type MetaExec struct {
	WorkDir   string
	Namespace string
	Dry       bool
	Ignore    bool

	Trace bool

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
	Parallel int64
	User     string
	Pass     string
}

type MetaHttp struct {
	CertFile string
	KeyFile  string
	Addr     string
	Base     string
}

type Registry map[string]CommandSettings

func (r Registry) Copy() Registry {
	x := make(Registry)
	for k, v := range r {
		x[k] = v
	}
	return x
}

func (r Registry) Prepare(name string) (Executer, error) {
	cmd, err := r.Lookup(name)
	if err != nil {
		return nil, err
	}
	return cmd.Prepare()
}

func (r Registry) LookupRemote(name string) (CommandSettings, error) {
	cmd, err := r.Lookup(name)
	if err != nil {
		return cmd, err
	}
	if !cmd.Remote() {
		return cmd, fmt.Errorf("%s: command can not be executed on remote server", name)
	}
	return cmd, nil
}

func (r Registry) Lookup(name string) (CommandSettings, error) {
	cmd, ok := r[name]
	if ok {
		return cmd, nil
	}
	for _, c := range r {
		i := sort.SearchStrings(c.Alias, name)
		if i < len(c.Alias) && c.Alias[i] == name {
			return c, nil
		}
	}
	return cmd, fmt.Errorf("%s: command not defined", name)
}

type SuggestionError struct {
	Others []string
	Err    error
}

func Suggest(err error, name string, names []string) error {
	names = distance.Levenshtein(name, names)
	if len(names) == 0 {
		return err
	}
	return SuggestionError{
		Err:    err,
		Others: names,
	}
}

func (s SuggestionError) Error() string {
	return s.Err.Error()
}

const defaultKnownHost = "~/.ssh/known_hosts"

func hasHelp(args []string) bool {
	as := make([]string, len(args))
	copy(as, args)
	sort.Strings(as)
	i := sort.Search(len(as), func(i int) bool {
		return "-h" <= as[i] || "-help" <= as[i] || "--help" <= as[i]
	})
	if i >= len(as) {
		return false
	}
	return as[i] == "-h" || as[i] == "-help" || as[i] == "--help"
}

func hasError(errs ...error) error {
	for _, e := range errs {
		if e != nil {
			return e
		}
	}
	return nil
}

func cleanFilename(str string) string {
	str = filepath.Base(str)
	for e := filepath.Ext(str); e != ""; e = filepath.Ext(str) {
		str = strings.TrimSuffix(str, e)
	}
	return str
}
