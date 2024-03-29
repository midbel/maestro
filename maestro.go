package maestro

import (
	"bytes"
	"context"
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

	"github.com/midbel/distance"
	"github.com/midbel/maestro/internal/env"
	"github.com/midbel/maestro/internal/help"
	"github.com/midbel/maestro/internal/stdio"
	"github.com/midbel/tish"
	"golang.org/x/crypto/ssh"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
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

	Includes Dirs
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

func (m *Maestro) ListenAndServe(args []string) error {
	var (
		set  = flag.NewFlagSet(CmdServe, flag.ExitOnError)
		addr = set.String("a", m.MetaHttp.Addr, "listening address")
	)
	if err := set.Parse(args); err != nil {
		return err
	}
	setupRoutes(m)
	server := http.Server{
		Addr: *addr,
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
	return m.schedule(args, stdio.Stdout, stdio.Stderr)
}

func (m *Maestro) schedule(args []string, stdout, stderr io.Writer) error {
	sort.Strings(args)
	grp, ctx := errgroup.WithContext(interruptContext())
	for _, c := range m.Commands {
		var (
			x = sort.SearchStrings(args, c.Name)
			k = x < len(args) && args[x] == c.Name
		)
		if len(args) > 0 && !k {
			continue
		}
		for i := range c.Schedules {
			var (
				c = scheduleContext(c, m.WithPrefix, m.Trace)
				e = c.Schedules[i]
			)
			grp.Go(func() error {
				return e.Run(ctx, m.Commands.Copy(), c, stdout, stderr)
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
		for _, s := range c.Schedules {
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
		for _, s := range c.Schedules {
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

func (m *Maestro) getCommandByNames(names []string) []CommandSettings {
	var (
		cs  []CommandSettings
		all []CommandSettings
	)
	sort.Strings(names)
	for n, c := range m.Commands {
		all = append(all, c)
		x := sort.SearchStrings(names, n)
		if x < len(names) && names[x] == n {
			cs = append(cs, c)
		}
	}
	if len(cs) == 0 {
		return all
	}
	return cs
}

func (m *Maestro) Dry(name string, args []string) error {
	cmd, err := m.setup(interruptContext(), name, true)
	if err != nil {
		return err
	}
	cmd.SetOut(stdio.Stdout)
	cmd.SetErr(stdio.Stderr)
	return cmd.Dry(args)
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

func (m *Maestro) Execute(name string, args []string) error {
	if name == "" && m.MetaExec.Default == "" {
		return m.ExecuteHelp(name)
	}
	if hasHelp(args) {
		return m.ExecuteHelp(name)
	}
	if m.MetaExec.Dry {
		return m.Dry(name, args)
	}
	if m.Remote {
		return m.executeRemote(name, args, stdio.Stdout, stdio.Stderr)
	}
	return m.execute(name, args, stdio.Stdout, stdio.Stderr)
}

func (m *Maestro) execute(name string, args []string, stdout, stderr io.Writer) error {
	ctx := interruptContext()
	cmd, err := m.setup(ctx, name, true)
	if err != nil {
		return err
	}
	option := ctreeOption{
		Trace:  m.Trace,
		NoDeps: m.NoDeps,
		Prefix: m.WithPrefix,
		Ignore: m.Ignore,
	}
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

func (m *Maestro) executeRemote(name string, args []string, stdout, stderr io.Writer) error {
	cmd, err := m.Commands.LookupRemote(name)
	if err != nil {
		return err
	}
	ex, err := cmd.Prepare()
	if err != nil {
		return err
	}
	scripts, err := ex.Script(args)
	if err != nil {
		return err
	}
	if m.MetaSSH.Parallel <= 0 {
		n := len(cmd.Hosts)
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

	for _, h := range cmd.Hosts {
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
			return m.executeHost(ctx, ex, host, scripts, sshout, ssherr)
		})
	}
	sema.Acquire(parent, m.MetaSSH.Parallel)
	return grp.Wait()
}

func (m *Maestro) executeHost(ctx context.Context, cmd Executer, addr string, scripts []string, stdout, stderr io.Writer) error {
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
	return help.Maestro(h)
}

func (m *Maestro) canExecute(cmd CommandSettings) error {
	if cmd.Blocked() {
		return fmt.Errorf("%s: command can not be called", cmd.Command())
	}
	if m.Remote && !cmd.Remote() {
		return fmt.Errorf("%s can not be executly on remote system", cmd.Command())
	}
	return nil
}

func (m *Maestro) resolve(cmd Executer, args []string, option ctreeOption) (executer, error) {
	var (
		list deplist
		err  error
	)
	if !option.NoDeps {
		list, err = m.resolveDependencies(cmd, option)
		if err != nil {
			return nil, err
		}
	}

	root := createMain(cmd, args, list)
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

func (m *Maestro) resolveList(names []string) ([]Executer, error) {
	var list []Executer
	for _, n := range names {
		x, err := m.Commands.Prepare(n)
		if err != nil {
			return nil, err
		}
		list = append(list, x)
	}
	return list, nil
}

func (m *Maestro) resolveDependencies(cmd Executer, option ctreeOption) (deplist, error) {
	var (
		traverse func(Executer) (deplist, error)
		seen     = make(map[string]struct{})
		empty    = struct{}{}
	)

	traverse = func(cmd Executer) (deplist, error) {
		var set []executer
		for _, d := range cmd.Dependencies() {
			if _, ok := seen[d.Key()]; ok && !d.Mandatory {
				continue
			}
			seen[d.Key()] = empty
			c, err := m.setup(context.Background(), d.Key(), false)
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

func (m *Maestro) setup(ctx context.Context, name string, can bool) (Executer, error) {
	cmd, err := m.Commands.Lookup(name)
	if err != nil {
		return nil, m.suggest(err, name)
	}
	if err := m.canExecute(cmd); can && err != nil {
		return nil, err
	}
	ex, err := cmd.Prepare(tish.WithFinder(makeFinder(m.Namespace, m.Commands)))
	if err != nil {
		return nil, err
	}
	return ex, nil
}

func (m *Maestro) suggest(err error, name string) error {
	var all []string
	for _, c := range m.Commands {
		all = append(all, c.Command())
		all = append(all, c.Alias...)
	}
	all = append(all, CmdHelp, CmdVersion, CmdAll, CmdDefault, CmdServe, CmdGraph, CmdSchedule)
	return Suggest(err, name, all)
}

func (m *Maestro) traverseGraph(name string, level int) ([]string, error) {
	cmd, err := m.Commands.Lookup(name)
	if err != nil {
		return nil, err
	}

	fmt.Fprintf(stdio.Stdout, "%s- %s", strings.Repeat(" ", level*2), name)
	fmt.Fprintln(stdio.Stdout)
	var list []string
	for _, d := range cmd.Deps {
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

type commandFinder struct {
	Space    string
	Commands Registry
}

func makeFinder(ns string, set Registry) tish.CommandFinder {
	return &commandFinder{
		Space:    ns,
		Commands: set,
	}
}

func (c *commandFinder) Find(ctx context.Context, name string) (tish.Command, error) {
	cmd, ok := c.Commands[name]
	if !ok {
		cmd, ok = c.findByName(name)
		if !ok {
			return nil, fmt.Errorf("%s: command not found", name)
		}
	}
	x, err := cmd.Prepare(tish.WithFinder(c))
	if err != nil {
		return nil, err
	}
	return makeShellCommand(ctx, x), nil
}

func (c *commandFinder) findByName(name string) (CommandSettings, bool) {
	var list []CommandSettings
	for _, cmd := range c.Commands {
		for _, a := range cmd.Alias {
			if a == name {
				return cmd, true
			}
		}
	}
	if len(list) > 0 {
		return list[0], true
	}
	return CommandSettings{}, false
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
