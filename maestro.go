package maestro

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
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

	Includes  Dirs
	Locals    *Env
	Duplicate string
	Commands  map[string]Command

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
		Locals:    EmptyEnv(),
		MetaAbout: about,
		MetaHttp:  mhttp,
		Duplicate: dupReplace,
		Commands:  make(map[string]Command),
	}
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

func (m *Maestro) ListenAndServe() error {
	server := http.Server{
		Addr:    m.MetaHttp.Addr,
		Handler: m,
	}
	return server.ListenAndServe()
}

func (m *Maestro) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// var (
	// 	nodeps = r.Header.Get("Maestro-NoDeps")
	// 	dry    = r.Header.Get("Maestro-Dry")
	// 	vars   = r.Header.Get("Maestro-Vars")
	// 	ignore = r.Header.Get("Maestro-Ignore")
	// 	trace  = r.Header.Get("Maestro-Trace")
	// )

	w.Header().Set("content-type", "text/plain")
	name := filepath.Base(r.URL.Path)
	switch name {
	case CmdAll:
	case CmdDefault:
	case CmdVersion:
		m.executeVersion(w)
		return
	case CmdHelp:
		m.executeHelp("", w)
		return
	case CmdListen, CmdServe:
		w.WriteHeader(http.StatusForbidden)
		return
	default:
	}
	w.Header().Set("Trailer", "Maestro-Exit")
	cmd, err := m.prepare(name)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	err = m.executeCommand(context.TODO(), cmd, nil, w, w)
	if cmd, ok := cmd.(*Single); ok {
		cmd.executed = false
	}

	exit := "ok"
	if err != nil {
		exit = err.Error()
	}
	w.Header().Set("Maestro-Exit", exit)
}

func (m *Maestro) Dry(name string, args []string) error {
	cmd, err := m.lookup(name)
	if err != nil {
		return err
	}
	m.TraceCommand(cmd, args)
	return cmd.Dry(args)
}

func (m *Maestro) Execute(name string, args []string) error {
	return m.execute(name, args, stdout, stderr)
}

func (m *Maestro) ExecuteDefault(args []string) error {
	if m.MetaExec.Default == "" {
		return fmt.Errorf("default command not defined")
	}
	return m.execute(m.MetaExec.Default, args, stdout, stderr)
}

func (m *Maestro) ExecuteAll(args []string) error {
	if len(m.MetaExec.All) == 0 {
		return fmt.Errorf("all command not defined")
	}
	for _, n := range m.MetaExec.All {
		if err := m.execute(n, args, stdout, stderr); err != nil {
			return err
		}
	}
	return nil
}

func (m *Maestro) ExecuteHelp(name string) error {
	return m.executeHelp(name, stdout)
}

func (m *Maestro) ExecuteVersion() error {
	return m.executeVersion(stdout)
}

func (m *Maestro) execute(name string, args []string, stdout, stderr io.Writer) error {
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
		return m.executeRemote(cmd, args, stdout, stderr)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Kill, os.Interrupt)
		<-sig
		cancel()
		close(sig)
	}()

	if !m.NoDeps {
		if err := m.executeDependencies(ctx, cmd); err != nil {
			return err
		}
	}
	m.executeList(ctx, m.MetaExec.Before, stdout, stderr)
	defer m.executeList(ctx, m.MetaExec.After, stdout, stderr)

	err = m.executeCommand(ctx, cmd, args, stdout, stderr)

	if errc := ctx.Done(); errc == nil {
		next := m.MetaExec.Success
		if err != nil {
			next = m.MetaExec.Error
		}
		for _, cmd := range next {
			c, err := m.lookup(cmd)
			if err == nil {
				c.Execute(ctx, nil)
			}
		}
	}
	return err
}

func (m *Maestro) executeHelp(name string, w io.Writer) error {
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
		fmt.Fprintln(w, strings.TrimSpace(help))
	}
	return err
}

func (m *Maestro) executeVersion(w io.Writer) error {
	fmt.Fprintf(w, "%s %s", m.Name(), m.Version)
	fmt.Fprintln(w)
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

func (m *Maestro) executeRemote(cmd Command, args []string, stdout, stderr io.Writer) error {
	scripts, err := cmd.Script(args)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Kill, os.Interrupt)
		<-sig
		cancel()
	}()
	if m.MetaSSH.Parallel <= 0 {
		n := len(cmd.Targets())
		m.MetaSSH.Parallel = int64(n)
	}
	var (
		grp, sub = errgroup.WithContext(ctx)
		sema     = semaphore.NewWeighted(m.MetaSSH.Parallel)
		seen     = make(map[string]struct{})
	)
	for _, h := range cmd.Targets() {
		if _, ok := seen[h]; ok {
			continue
		}
		seen[h] = struct{}{}
		if err := sema.Acquire(ctx, 1); err != nil {
			return err
		}
		host := h
		grp.Go(func() error {
			defer sema.Release(1)
			return m.executeHost(sub, cmd, host, scripts, stdout, stderr)
		})
	}
	sema.Acquire(ctx, m.MetaSSH.Parallel)
	return grp.Wait()
}

func (m *Maestro) executeHost(ctx context.Context, cmd Command, addr string, scripts []string, stdout, stderr io.Writer) error {
	var (
		pout, _ = createPipe()
		perr, _ = createPipe()
	)

	defer func() {
		pout.Close()
		perr.Close()
	}()
	exec := func(sess *ssh.Session, line string) error {
		cmd.SetOut(pout.W)
		cmd.SetErr(perr.W)

		prefix := fmt.Sprintf("%s;%s;%s", m.MetaSSH.User, addr, cmd.Command())

		go toStd(prefix, stdout, createLine(pout.R))
		go toStd(prefix, stderr, createLine(perr.R))

		defer sess.Close()
		sess.Stdout = pout.W
		sess.Stderr = perr.W

		return m.TraceTime(cmd, nil, func() error {
			return sess.Run(line)
		})
	}
	config := ssh.ClientConfig{
		User:            m.MetaSSH.User,
		Auth:            m.MetaSSH.AuthMethod(),
		HostKeyCallback: m.CheckHostKey, //ssh.InsecureIgnoreHostKey(),
	}
	client, err := ssh.Dial("tcp", addr, &config)
	if err != nil {
		return err
	}
	defer client.Close()
	for i := range scripts {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		sess, err := client.NewSession()
		if err != nil {
			return err
		}
		if err := exec(sess, scripts[i]); err != nil {
			return err
		}
	}
	return nil
}

func (m *Maestro) executeList(ctx context.Context, list []string, stdout, stderr io.Writer) {
	for i := range list {
		cmd, err := m.lookup(list[i])
		if err != nil {
			continue
		}
		m.executeCommand(ctx, cmd, nil, stdout, stderr)
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

func (m *Maestro) executeCommand(ctx context.Context, cmd Command, args []string, stdout, stderr io.Writer) error {
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

	go toStd(cmd.Command(), stdout, createLine(pout.R))
	go toStd(cmd.Command(), stderr, createLine(perr.R))

	return m.TraceTime(cmd, args, func() error {
		err := cmd.Execute(ctx, args)
		if err != nil && m.MetaExec.Ignore {
			err = nil
		}
		return err
	})
}

func (m *Maestro) executeDependencies(ctx context.Context, cmd Command) error {
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
				if err := m.executeDependencies(ctx, cmd); err != nil {
					return err
				}
				m.executeCommand(ctx, cmd, d.Args, stdout, stderr)
				return nil
			})
		} else {
			err := m.executeCommand(ctx, cmd, d.Args, stdout, stderr)
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
	cmd, err := m.lookup(name)
	if err == nil {
		return cmd, err
	}
	return nil, m.suggest(err, name)
}

func (m *Maestro) suggest(err error, name string) error {
	var all []string
	for _, c := range m.Commands {
		all = append(all, c.Command())
		s, ok := c.(*Single)
		if !ok {
			continue
		}
		for _, a := range s.Alias {
			all = append(all, a)
		}
	}
	return Suggest(err, name, all)
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
	Ignore  bool

	Trace bool

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

func (m MetaExec) TraceCommand(cmd Command, args []string) {
	m.traceStart(cmd, args)
}

func (m MetaExec) traceEnd(cmd Command, err error, elapsed time.Duration) {
	if !m.Trace {
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
	if !m.Trace {
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
	Parallel int64
	User     string
	Pass     string
	Key      ssh.Signer
	Hosts    []hostEntry
}

func (m MetaSSH) AuthMethod() []ssh.AuthMethod {
	var list []ssh.AuthMethod
	if m.Pass != "" {
		list = append(list, ssh.Password(m.Pass))
	}
	if m.Key != nil {
		list = append(list, ssh.PublicKeys(m.Key))
	}
	return list
}

func (m MetaSSH) CheckHostKey(host string, addr net.Addr, key ssh.PublicKey) error {
	if len(m.Hosts) == 0 {
		return nil
	}
	i := sort.Search(len(m.Hosts), func(i int) bool {
		return host <= m.Hosts[i].Host
	})
	if i < len(m.Hosts) && m.Hosts[i].Host == host {
		ok := bytes.Equal(m.Hosts[i].Key.Marshal(), key.Marshal())
		if ok {
			return nil
		}
		return fmt.Errorf("%s: public key mismatched", host)
	}
	return fmt.Errorf("%s unknwon host (%s)", host, addr)
}

type MetaHttp struct {
	CertFile string
	KeyFile  string
	Addr     string
	Base     string

	// mapping of commands and http method
	// commands not listed won't be available for execution
	Get    []string
	Post   []string
	Delete []string
	Patch  []string
	Put    []string
	Head   []string
}

const defaultKnownHost = "~/.ssh/known_hosts"

type hostEntry struct {
	Host string
	Key  ssh.PublicKey
}

func createEntry(host string, key ssh.PublicKey) hostEntry {
	return hostEntry{
		Host: host,
		Key:  key,
	}
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

type lineReader struct {
	scan *bufio.Scanner
}

func createLine(r io.Reader) io.Reader {
	return &lineReader{
		scan: bufio.NewScanner(r),
	}
}

func (r *lineReader) Read(b []byte) (int, error) {
	if !r.scan.Scan() {
		err := r.scan.Err()
		if err == nil {
			err = io.EOF
		}
		return 0, io.EOF
	}
	x := r.scan.Bytes()
	return copy(b, append(x, '\n')), r.scan.Err()
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
