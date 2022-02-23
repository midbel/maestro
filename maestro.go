package maestro

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/midbel/maestro/internal/env"
	"github.com/midbel/maestro/internal/help"
	"github.com/midbel/maestro/internal/stdio"
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

	Includes  Dirs
	Locals    *env.Env
	Duplicate string
	Commands  map[commandKey]Command

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
		Duplicate: dupReplace,
		Commands:  make(map[commandKey]Command),
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

func (m *Maestro) Register(ns string, cmd *Single) error {
	var (
		key      = makeKey(ns, cmd.Name)
		curr, ok = m.Commands[key]
	)
	if !ok {
		m.Commands[key] = cmd
		return nil
	}
	switch m.Duplicate {
	case dupError:
		return fmt.Errorf("%s %w", cmd.Name, ErrDuplicate)
	case dupReplace, "":
		m.Commands[key] = cmd
	case dupAppend:
		if mul, ok := curr.(Combined); ok {
			curr = append(mul, cmd)
			break
		}
		mul := make(Combined, 0, 2)
		mul = append(mul, curr)
		mul = append(mul, cmd)
		m.Commands[key] = mul
	default:
		return fmt.Errorf("DUPLICATE: unknown value %s", m.Duplicate)
	}
	return nil
}

func (m *Maestro) ListenAndServe() error {
	setupRoutes(m)
	server := http.Server{
		Addr: m.MetaHttp.Addr,
	}
	return server.ListenAndServe()
}

func (m *Maestro) Graph(name string) error {
	all, err := m.traverseGraph(name, 0)
	var (
		seen = make(map[string]struct{})
		deps = make([]string, 0, len(all))
		zero = struct{}{}
	)
	for _, n := range all {
		if _, ok := seen[n]; ok {
			continue
		}
		seen[n] = zero
		deps = append(deps, n)
	}
	fmt.Fprintf(stdio.Stdout, "order %s -> %s", strings.Join(deps, " -> "), name)
	fmt.Fprintln(stdio.Stdout)
	return err
}

func (m *Maestro) Schedule(args []string) error {
	var (
		set   = flag.NewFlagSet(CmdSchedule, flag.ExitOnError)
		list  = set.Bool("l", false, "show list of schedule command")
		limit = set.Int("n", 0, "show next schedule time")
	)
	if err := set.Parse(args); err != nil {
		return err
	}
	if *list {
		return m.scheduleList(args, *limit)
	}
	return m.schedule(stdio.Stdout, stdio.Stderr)
}

func (m *Maestro) schedule(stdout, stderr io.Writer) error {
	var (
		parent   = interruptContext()
		grp, ctx = errgroup.WithContext(parent)
	)
	_ = ctx
	for _, c := range m.Commands {
		s, ok := c.(*Single)
		if !ok {
			continue
		}
		for i := range s.Schedules {
			e := s.Schedules[i]
			grp.Go(func() error {
				return e.Run(ctx, stdout, stderr)
			})
		}
	}
	return grp.Wait()
}

func (m *Maestro) scheduleList(args []string, limit int) error {
	if limit == 0 {
		m.showScheduleShort(args)
	} else {
		m.showScheduleLong(args, limit)
	}
	return nil
}

func (m *Maestro) showScheduleShort(args []string) {
	now := time.Now()
	for _, c := range m.getCommandByNames(args) {
		s, ok := c.(*Single)
		if !ok || len(s.Schedules) == 0 {
			continue
		}
		for _, s := range s.Schedules {
			var wait time.Duration
			for wait <= 0 {
				next := s.Sched.Next()
				wait = next.Sub(now)
			}
			fmt.Fprintf(stdio.Stdout, "- %s in %s", c.Command(), wait)
			fmt.Fprintln(stdio.Stdout)
		}
	}
}

func (m *Maestro) showScheduleLong(args []string, limit int) {
	for _, c := range m.getCommandByNames(args) {
		s, ok := c.(*Single)
		if !ok || len(s.Schedules) == 0 {
			continue
		}
		for _, s := range s.Schedules {
			fmt.Fprintln(stdio.Stdout, "*", c.Command())
			prefix := "next"
			for i := 0; i < limit; i++ {
				w := s.Sched.Next()
				fmt.Fprintf(stdio.Stdout, "  %s at %s", prefix, w.Format("2006-01-02 15:04:05"))
				fmt.Fprintln(stdio.Stdout)
				prefix = "then"
			}
		}
	}
}

func (m *Maestro) getCommandByNames(names []string) []Command {
	var (
		cs  []Command
		all []Command
	)
	sort.Strings(names)
	for n, c := range m.Commands {
		all = append(all, c)
		x := sort.SearchStrings(names, n.Name)
		if x < len(names) && names[x] == n.Name {
			cs = append(cs, c)
		}
	}
	if len(cs) == 0 {
		return all
	}
	return cs
}

func (m *Maestro) Dry(name string, args []string) error {
	cmd, err := m.prepare(name)
	if err != nil {
		return err
	}
	return cmd.Dry(args)
}

func (m *Maestro) Execute(name string, args []string) error {
	return m.execute(name, args, stdio.Stdout, stdio.Stderr)
}

func (m *Maestro) ExecuteDefault(args []string) error {
	if m.MetaExec.Default == "" {
		return fmt.Errorf("default command not defined")
	}
	return m.execute(m.MetaExec.Default, args, stdio.Stdout, stdio.Stderr)
}

func (m *Maestro) ExecuteAll(args []string) error {
	if len(m.MetaExec.All) == 0 {
		return fmt.Errorf("all command not defined")
	}
	for _, n := range m.MetaExec.All {
		if err := m.execute(n, args, stdio.Stdout, stdio.Stderr); err != nil {
			return err
		}
	}
	return nil
}

func (m *Maestro) ExecuteHelp(name string) error {
	return m.executeHelp(name, stdio.Stdout)
}

func (m *Maestro) ExecuteVersion() error {
	return m.executeVersion(stdio.Stdout)
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

	var (
		ctx    = interruptContext()
		option = ctreeOption{
			Trace:  m.Trace,
			NoDeps: m.NoDeps,
			Prefix: m.WithPrefix,
			Ignore: m.Ignore,
		}
	)
	ex, err := m.resolve(cmd, args, option)
	if err != nil {
		return err
	}
	if c, ok := ex.(io.Closer); ok {
		defer c.Close()
	}
	return ex.Execute(ctx, stdout, stderr)
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

func (m *Maestro) executeRemote(cmd Command, args []string, stdout, stderr io.Writer) error {
	scripts, err := cmd.Script(args)
	if err != nil {
		return err
	}
	if m.MetaSSH.Parallel <= 0 {
		n := len(cmd.Targets())
		m.MetaSSH.Parallel = int64(n)
	}
	var (
		parent   = interruptContext()
		grp, ctx = errgroup.WithContext(parent)
		sema     = semaphore.NewWeighted(m.MetaSSH.Parallel)
		seen     = make(map[string]struct{})
		pout, _  = createPipe()
		perr, _  = createPipe()
		sshout   = stdio.Lock(pout)
		ssherr   = stdio.Lock(perr)
	)

	go io.Copy(stdout, pout)
	go io.Copy(stderr, perr)

	for _, h := range cmd.Targets() {
		if _, ok := seen[h]; ok {
			continue
		}
		seen[h] = struct{}{}
		if err := sema.Acquire(parent, 1); err != nil {
			return err
		}
		host := h
		grp.Go(func() error {
			defer sema.Release(1)
			return m.executeHost(ctx, cmd, host, scripts, sshout, ssherr)
		})
	}
	sema.Acquire(parent, m.MetaSSH.Parallel)
	return grp.Wait()
}

func (m *Maestro) executeHost(ctx context.Context, cmd Command, addr string, scripts []string, stdout, stderr io.Writer) error {
	var (
		prefix = fmt.Sprintf("%s;%s;%s", m.MetaSSH.User, addr, cmd.Command())
		exec   = func(sess *ssh.Session, line string) error {
			setPrefix(stdout, prefix)
			setPrefix(stderr, prefix)

			defer sess.Close()
			sess.Stdout = stdout
			sess.Stderr = stderr

			return sess.Run(line)
		}
	)
	config := ssh.ClientConfig{
		User:            m.MetaSSH.User,
		Auth:            m.MetaSSH.AuthMethod(),
		HostKeyCallback: m.CheckHostKey,
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

func (m *Maestro) help() (string, error) {
	h := struct {
		File     string
		Help     string
		Usage    string
		Version  string
		Commands map[string][]Command
	}{
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
	return help.Maestro(h)
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

func (m *Maestro) resolve(cmd Command, args []string, option ctreeOption) (executer, error) {
	var list deplist
	if !option.NoDeps {
		deps, err := m.resolveDependencies(cmd, option)
		if err != nil {
			return nil, err
		}
		list = deps
	}
	var (
		root = createMain(cmd, args, list)
		err  error
	)
	root.ignore = option.Ignore
	root.pre, err = m.resolveList(m.Before)
	root.post, err = m.resolveList(m.After)
	root.errors, err = m.resolveList(m.Error)
	root.success, err = m.resolveList(m.Success)

	var ex executer = root
	if option.Trace {
		ex = trace(ex)
	}

	tree, err := createTree(ex)
	if err != nil {
		return nil, err
	}
	tree.prefix = option.Prefix
	return &tree, nil
}

func (m *Maestro) resolveList(names []string) ([]Command, error) {
	var list []Command
	for _, n := range names {
		c, err := m.lookup(n)
		if err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	return list, nil
}

func (m *Maestro) resolveDependencies(cmd Command, option ctreeOption) (deplist, error) {
	var (
		traverse func(Command) (deplist, error)
		seen     = make(map[string]struct{})
		empty    = struct{}{}
	)

	traverse = func(cmd Command) (deplist, error) {
		s, ok := cmd.(*Single)
		if !ok {
			return nil, nil
		}
		var set []executer
		for _, d := range s.Deps {
			if _, ok := seen[d.Name]; ok && !d.Mandatory {
				continue
			}
			seen[d.Name] = empty
			c, err := m.prepare(d.Name)
			if err != nil {
				if d.Optional && !d.Mandatory {
					continue
				}
				return nil, err
			}
			list, err := traverse(c)
			if err != nil {
				return nil, err
			}
			ed := createDep(c, d.Args, list)
			ed.background = d.Bg

			var ex executer = ed
			if option.Trace {
				ex = trace(ex)
			}

			set = append(set, ex)
		}
		return deplist(set), nil
	}
	return traverse(cmd)
}

func (m *Maestro) prepare(name string) (Command, error) {
	cmd, err := m.lookup(name)
	if err != nil {
		return nil, m.suggest(err, name)
	}
	if curr, ok := cmd.(*Single); ok {
		for _, c := range m.Commands {
			// if n == name {
			// 	continue
			// }
			s := makeShellCommand(context.TODO(), c)
			curr.shell.Register(s)
		}
	}
	return cmd, nil
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
	all = append(all, CmdHelp, CmdVersion, CmdAll, CmdDefault, CmdServe, CmdGraph, CmdSchedule)
	return Suggest(err, name, all)
}

func (m *Maestro) lookup(name string) (Command, error) {
	if name == "" {
		name = m.MetaExec.Default
	}
	var (
		key     = defaultKey(name)
		cmd, ok = m.Commands[key]
	)
	if ok {
		return cmd, nil
	}
	for k, c := range m.Commands {
		if k.Space != key.Space {
			continue
		}
		s, ok := c.(*Single)
		if !ok {
			continue
		}
		i := sort.SearchStrings(s.Alias, key.Name)
		if i < len(s.Alias) && s.Alias[i] == key.Name {
			return c, nil
		}
	}
	return nil, fmt.Errorf("%s: command not defined", name)
}

func (m *Maestro) traverseGraph(name string, level int) ([]string, error) {
	cmd, err := m.lookup(name)
	if err != nil {
		return nil, err
	}
	s, ok := cmd.(*Single)
	if !ok {
		return nil, nil
	}
	fmt.Fprintf(stdio.Stdout, "%s- %s", strings.Repeat(" ", level*2), name)
	fmt.Fprintln(stdio.Stdout)
	var list []string
	for _, d := range s.Deps {
		others, err := m.traverseGraph(d.Name, level+1)
		if err != nil {
			return nil, err
		}
		list = append(list, others...)
		list = append(list, d.Name)
	}
	return list, nil
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
	return fmt.Errorf("%s unknown host (%s)", host, addr)
}

type MetaHttp struct {
	CertFile string
	KeyFile  string
	Addr     string
	Base     string
}

type commandKey struct {
	Space string
	Name  string
}

func defaultKey(name string) commandKey {
	ns, rest, ok := strings.Cut(name, "::")
	if ok {
		name = rest
	} else {
		ns = ""
	}
	return makeKey(ns, name)
}

func makeKey(ns, name string) commandKey {
	return commandKey{
		Space: ns,
		Name:  name,
	}
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

func interruptContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		sig := make(chan os.Signal, 1)
		defer close(sig)
		signal.Notify(sig, os.Kill, os.Interrupt)
		<-sig
		cancel()
	}()
	return ctx
}
