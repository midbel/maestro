package maestro

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

var ErrDuplicate = errors.New("command already registered")

const (
	dupError   = "error"
	dupAppend  = "append"
	dupReplace = "replace"
)

const (
	CmdHelp    = "help"
	CmdVersion = "version"
	CmdAll     = "all"
	CmdDefault = "default"
	CmdListen  = "listen"
	CmdServe   = "serve"
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

	Duplicate string
	Commands  map[string]Command
	Alias     map[string]string

	Remote bool
	NoDeps bool
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
		MetaAbout: about,
		MetaHttp:  mhttp,
		Duplicate: dupReplace,
		Commands:  make(map[string]Command),
		Alias:     make(map[string]string),
	}
}

func (m *Maestro) Load(file string) error {
	r, err := os.Open(file)
	if err != nil {
		return err
	}
	defer r.Close()

	d, err := NewDecoder(r)
	if err != nil {
		return err
	}
	if err := d.decode(m); err != nil {
		return err
	}
	m.MetaAbout.File = file
	return nil
}

func (m *Maestro) ListenAndServe() error {
	return nil
}

func (m *Maestro) Dry(name string, args []string) error {
	cmd, err := m.lookup(name)
	if err != nil {
		return err
	}
	m.Trace(cmd, args)
	return cmd.Dry(args)
}

func (m *Maestro) Execute(name string, args []string) error {
	if hasHelp(args) {
		return m.ExecuteHelp(name)
	}
	if m.MetaExec.Dry {
		return m.Dry(name, args)
	}
	cmd, err := m.prepare(name)
	if err != nil {
		return err
	}
	if err := m.canExecute(cmd); err != nil {
		return err
	}
	if m.Remote {
		return m.executeRemote(cmd, args)
	}

	if !m.NoDeps {
		if err := m.executeDependencies(cmd); err != nil {
			return err
		}
	}
	m.executeList(m.MetaExec.Before)
	defer m.executeList(m.MetaExec.After)

	err = m.executeCommand(cmd, args)

	next := m.MetaExec.Success
	if err != nil {
		next = m.MetaExec.Error
	}
	for _, cmd := range next {
		c, err := m.lookup(cmd)
		if err == nil {
			c.Execute(nil)
		}
	}
	return err
}

func (m *Maestro) ExecuteDefault(args []string) error {
	if m.MetaExec.Default == "" {
		return fmt.Errorf("default command not defined")
	}
	return m.Execute(m.MetaExec.Default, args)
}

func (m *Maestro) ExecuteAll(args []string) error {
	if len(m.MetaExec.All) == 0 {
		return fmt.Errorf("all command not defined")
	}
	for _, n := range m.MetaExec.All {
		if err := m.Execute(n, args); err != nil {
			return err
		}
	}
	return nil
}

func (m *Maestro) ExecuteHelp(name string) error {
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

func (m *Maestro) executeRemote(cmd Command, args []string) error {
	scripts, err := cmd.Script(args)
	if err != nil {
		return err
	}
	for _, str := range scripts {
		fmt.Println(str)
	}
	return nil
}

func (m *Maestro) executeList(list []string) {
	for i := range list {
		cmd, err := m.lookup(list[i])
		if err != nil {
			continue
		}
		m.executeCommand(cmd, nil)
	}
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
		if c.Blocked() || !c.Can() {
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
	return renderTemplate(helptext, h)
}

func (m *Maestro) Name() string {
	return strings.TrimSuffix(filepath.Base(m.File), filepath.Ext(m.File))
}

func (m *Maestro) canExecute(cmd Command) error {
	if cmd.Blocked() {
		return fmt.Errorf("%s: command can not be called", cmd.Command())
	}
	if !cmd.Can() {
		return fmt.Errorf("current user is not allowed to executed %s", cmd.Command())
	}
	if m.Remote && !cmd.Remote() {
		return fmt.Errorf("%s can not be executly on remote system", cmd.Command())
	}
	return nil
}

func (m *Maestro) executeCommand(cmd Command, args []string) error {
	var (
		pout, _ = createPipe()
		perr, _ = createPipe()
	)

	defer func() {
		pout.Close()
		perr.Close()
	}()

	cmd.SetOut(pout.W)
	cmd.SetErr(perr.W)

	go toStd(cmd.Command(), stdout, pout.R)
	go toStd(cmd.Command(), stderr, perr.R)

	return m.TraceTime(cmd, args, func() error {
		return cmd.Execute(args)
	})
}

func (m *Maestro) executeDependencies(cmd Command) error {
	deps, err := m.resolveDependencies(cmd)
	if err != nil {
		return err
	}
	var (
		grp  errgroup.Group
		seen = make(map[string]struct{})
	)
	for i := range deps {
		if _, ok := seen[deps[i].Name]; ok {
			continue
		}
		seen[deps[i].Name] = struct{}{}

		cmd, err := m.prepare(deps[i].Name)
		if err != nil {
			if deps[i].Optional {
				continue
			}
			return err
		}
		if d := deps[i]; d.Bg {
			grp.Go(func() error {
				if err := m.executeDependencies(cmd); err != nil {
					return err
				}
				m.executeCommand(cmd, d.Args)
				return nil
			})
		} else {
			err := m.executeCommand(cmd, d.Args)
			if err != nil && !deps[i].Optional {
				return err
			}
		}
	}
	grp.Wait()
	return grp.Wait()
}

func (m *Maestro) resolveDependencies(cmd Command) ([]Dep, error) {
	var traverse func(Command) ([]Dep, error)

	traverse = func(cmd Command) ([]Dep, error) {
		s, ok := cmd.(*Single)
		if !ok {
			return nil, nil
		}
		var all []Dep
		for _, d := range s.Deps {
			c, err := m.lookup(d.Name)
			if err != nil {
				return nil, err
			}
			set, err := traverse(c)
			if err != nil {
				return nil, err
			}
			all = append(all, set...)
			all = append(all, d)
		}
		return all, nil
	}
	return traverse(cmd)
}

func (m *Maestro) prepare(name string) (Command, error) {
	return m.lookup(name)
}

func (m *Maestro) lookup(name string) (Command, error) {
	if name == "" {
		name = m.MetaExec.Default
	}
	cmd, ok := m.Commands[name]
	if ok {
		return cmd, nil
	}
	for _, c := range m.Commands {
		s, ok := c.(*Single)
		if !ok {
			continue
		}
		i := sort.SearchStrings(s.Alias, name)
		if i < len(s.Alias) && s.Alias[i] == name {
			return c, nil
		}
	}
	return nil, fmt.Errorf("%s: command not defined", name)
}

type MetaExec struct {
	WorkDir string
	Dry     bool

	Echo bool

	All     []string
	Default string
	Before  []string
	After   []string
	Error   []string
	Success []string
}

func (m MetaExec) TraceTime(cmd Command, args []string, run func() error) error {
	if cmd.HasRun() {
		return nil
	}
	m.traceStart(cmd, args)
	var (
		now = time.Now()
		err = run()
	)
	m.traceEnd(cmd, err, time.Since(now))
	return err
}

func (m MetaExec) Trace(cmd Command, args []string) {
	m.traceStart(cmd, args)
}

func (m MetaExec) traceEnd(cmd Command, err error, elapsed time.Duration) {
	if !m.Echo {
		return
	}
	if err != nil {
		fmt.Print("[maestro] fail")
		fmt.Println()
	}
	fmt.Printf("[maestro] time: %.3fs", elapsed.Seconds())
	fmt.Println()
}

func (m MetaExec) traceStart(cmd Command, args []string) {
	if !m.Echo {
		return
	}
	fmt.Printf("[maestro] %s", cmd.Command())
	if len(args) > 0 {
		fmt.Printf(": %s", strings.Join(args, " "))
	}
	fmt.Println()
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

type MetaHttp struct {
	CertFile string
	KeyFile  string
	Addr     string
}

type help struct {
	File     string
	Help     string
	Usage    string
	Version  string
	Commands map[string][]Command
}

type pipe struct {
	R *os.File
	W *os.File
}

func createPipe() (*pipe, error) {
	var (
		p   pipe
		err error
	)
	p.R, p.W, err = os.Pipe()
	return &p, err
}

func (p *pipe) Close() error {
	p.R.Close()
	return p.W.Close()
}

type prefixWriter struct {
	prefix string
	inner  io.Writer
}

func createPrefix(prefix string, w io.Writer) io.Writer {
	return &prefixWriter{
		prefix: fmt.Sprintf("[%s] ", prefix),
		inner:  w,
	}
}

func (w *prefixWriter) Write(b []byte) (int, error) {
	io.WriteString(w.inner, w.prefix)
	return w.inner.Write(b)
}

type lockedWriter struct {
	mu sync.Mutex
	io.Writer
}

func createLock(w io.Writer) io.Writer {
	return &lockedWriter{
		Writer: w,
	}
}

func (w *lockedWriter) Write(b []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.Writer.Write(b)
}

var (
	stdout = createLock(os.Stdout)
	stderr = createLock(os.Stderr)
)

func toStd(prefix string, w io.Writer, r io.Reader) {
	io.Copy(createPrefix(prefix, w), r)
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
