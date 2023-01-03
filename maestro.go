package maestro

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
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

	Locals   *Env
	Commands Registry

	Includes   []string
	Remote     bool
	NoDeps     bool
	WithPrefix bool
}

func New() *Maestro {
	return &Maestro{
		Locals:    EmptyEnv(),
		MetaAbout: defaultAbout(),
		MetaHttp:  defaultHttp(),
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
	return m.Commands.Register(cmd)
}

func (m *Maestro) ShowGraph(name string) error {
	return nil
}

func (m *Maestro) ExecuteHelp(name string) error {
	return m.executeHelp(name, os.Stdout)
}

func (m *Maestro) ExecuteVersion() error {
	return m.executeVersion(os.Stdout)
}

func (m *Maestro) ExecuteAll(args []string) error {
	if len(m.MetaExec.All) == 0 {
		return fmt.Errorf("all command not defined")
	}
	for _, n := range m.MetaExec.All {
		if err := m.execute(n, args); err != nil {
			return err
		}
	}
	return nil
}

func (m *Maestro) ExecuteDefault(args []string) error {
	if m.MetaExec.Default == "" {
		return fmt.Errorf("default command not defined")
	}
	return m.execute(m.MetaExec.Default, args)
}

func (m *Maestro) Execute(name string, args []string) error {
	if name == "" && m.MetaExec.Default == "" {
		return m.ExecuteHelp(name)
	}
	if hasHelp(args) {
		return m.ExecuteHelp(name)
	}
	return m.execute(name, args)
}

func (m *Maestro) execute(name string, args []string) error {
	cmd, err := m.Commands.Lookup(name, m.NoDeps)
	if err != nil {
		return err
	}
	if m.Trace {
		cmd = Trace(cmd)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		sig := make(chan os.Signal, 1)
		defer close(sig)
		signal.Notify(sig, os.Kill, os.Interrupt)

		select {
		case <-ctx.Done():
		case <-sig:
			cancel()
		}
	}()

	err = cmd.Execute(ctx, args, os.Stdout, os.Stderr)
	if !errors.Is(ctx.Err(), context.Canceled) {
		cancel()
	}
	return err
}

func (m *Maestro) executeHelp(name string, w io.Writer) error {
	var (
		help string
		err  error
	)
	if name != "" {
		help, err = m.Commands.Help(name)
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
	h := struct {
		File     string
		Help     string
		Usage    string
		Version  string
		Commands map[string][]CommandSettings
	}{
		Version:  m.Version,
		File:     m.Name(),
		Usage:    m.Usage,
		Help:     m.Help,
		Commands: make(map[string][]CommandSettings),
	}
	for _, c := range m.Commands {
		if c.Blocked() {
			continue
		}
		for _, t := range c.Tags() {
			h.Commands[t] = append(h.Commands[t], c)
		}
	}
	for _, cs := range h.Commands {
		sort.Slice(cs, func(i, j int) bool {
			return cs[i].Command() < cs[j].Command()
		})
	}
	return MaestroHelp(h)
}

type MetaExec struct {
	WorkDir string
	Dry     bool
	Ignore  bool

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

func defaultAbout() MetaAbout {
	return MetaAbout{
		File:    DefaultFile,
		Version: DefaultVersion,
	}
}

type MetaSSH struct {
	Parallel int64
	User     string
	Pass     string
}

const defaultKnownHost = "~/.ssh/known_hosts"

type MetaHttp struct {
	CertFile string
	KeyFile  string
	Addr     string
	Base     string
}

func defaultHttp() MetaHttp {
	return MetaHttp{
		Addr: DefaultHttpAddr,
	}
}

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
