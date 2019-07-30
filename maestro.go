package maestro

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"golang.org/x/crypto/ssh"
	"golang.org/x/sync/errgroup"
)

const (
	DefaultShell      = "/bin/sh -c"
	NoParallel        = -1
	UnlimitedParallel = 0
)

var summary = `
{{usage .About}}
{{- if .Tags}}
{{range $k, $vs := .Tags}}
{{$k}} actions:

{{range $vs}}
{{- printf "- %-12s %s" .Name (usage .Help) -}}{{if .Hazard}}*{{end}}
{{end}}
{{- end}}
{{end -}}

{{- if .Usage}}
	{{- .Usage}}
{{else -}}
try maestro help <action|namespace> for more information about that topic!
{{end}}
`

type Maestro struct {
	Shell string // .SHELL
	Bin   string // .BIN: directory where scripts will be written

	// special variables for actions
	all []string // .ALL
	cmd string   // .DEFAULT

	Hosts []string // .HOSTS

	Name    string // .NAME
	Version string // .VERSION
	About   string // .ABOUT
	Usage   string // .USAGE
	Help    string // .HELP

	// special Actions around the action being executed
	// order is: BEGIN->action->(SUCCESS|FAILURE)->END
	Begin   string
	End     string
	Success string
	Failure string

	// SSH Options
	User string
	Key  string

	// actions
	Actions map[string]Action
	Nodeps  bool
	Noskip  bool

	Parallel int  // .PARALLEL
	Eta      bool // .ETA
	Echo     bool // .ECHO
}

func (m Maestro) Execute(a string, args []string) error {
	act, ok := m.Actions[a]
	if !ok {
		return fmt.Errorf("%s: action not found!", a)
	}

	set := flag.NewFlagSet(a, flag.ExitOnError)
	set.Usage = func() { act.Usage() }

	set.StringVar(&act.Shell, "shell", act.Shell, "shell")
	set.StringVar(&act.Workdir, "workdir", act.Workdir, "working directory")
	set.StringVar(&act.Stdout, "stdout", act.Stdout, "stdout")
	set.StringVar(&act.Stderr, "stderr", act.Stderr, "stderr")
	set.BoolVar(&act.Env, "env", act.Env, "environment")
	set.BoolVar(&act.Ignore, "ignore", act.Ignore, "ignore error")
	set.BoolVar(&act.Remote, "remote", act.Remote, "remote")
	set.Int64Var(&act.Retry, "retry", act.Retry, "retry on failure")
	set.DurationVar(&act.Delay, "delay", act.Delay, "delay")
	set.DurationVar(&act.Timeout, "timeout", act.Timeout, "timeout")

	if err := set.Parse(args); err != nil {
		return err
	}
	act.Args = append(act.Args, set.Args()...)

	var execute func(Action) error
	if act.Remote {
		execute = m.executeRemote
	} else {
		execute = m.executeLocal
	}
	return execute(act)
}

func (m Maestro) executeLocal(act Action) error {
	var deps [][]string
	if !m.Nodeps {
		ds, err := m.groupDependencies(act)
		if err != nil {
			return err
		}
		deps = ds
	}
	var err error
	next := m.Success
	if a, ok := m.Actions[m.Begin]; ok {
		m.executeAction(a, nil, false)
	}
	if err = m.executeAction(act, deps, m.Echo); err != nil {
		next = m.Failure
	}
	if a, ok := m.Actions[next]; ok {
		m.executeAction(a, nil, false)
	}
	if a, ok := m.Actions[m.End]; ok {
		m.executeAction(a, nil, false)
	}
	return err
}

func (m Maestro) executeRemote(act Action) error {
	if act.Local {
		return fmt.Errorf("%s: local only action", act.Name)
	}
	act.Hosts = append(act.Hosts, m.Hosts...)
	if len(act.Hosts) == 0 {
		return fmt.Errorf("%s: no remote host given", act.Name)
	}
	config := ssh.ClientConfig{
		User: "",
		Auth: []ssh.AuthMethod{},
	}
	for _, h := range act.Hosts {
		c, err := ssh.Dial("tcp", h, &config)
		if err != nil {
			return err
		}
		_ = c
	}
	return nil
}

func (m Maestro) executeAction(a Action, deps [][]string, echo bool) error {
	if m.Parallel == NoParallel {
		fs := flatten(deps)
		return m.executeChain(append(fs, a.Name))
	}
	if m.Parallel == UnlimitedParallel {
		m.Parallel = (1 << 16) - 1
	}
	for i := 0; i < len(deps); i++ {
		var (
			group errgroup.Group
			sema  = make(chan struct{}, m.Parallel)
		)
		for _, d := range deps[i] {
			sema <- struct{}{}

			fn := wrapAction(m.Actions[d], m.Echo)
			group.Go(func() error {
				defer func() { <-sema }()
				return fn()
			})
		}
		if err := group.Wait(); err != nil {
			return err
		}
	}
	return wrapAction(a, m.Echo && echo)()
}

func wrapAction(a Action, echo bool) func() error {
	if !echo {
		return a.Execute
	}
	return func() error {
		fmt.Printf("%s: started\n", a.Name)
		err := a.Execute()
		if err == nil {
			fmt.Printf("%s: done\n", a.Name)
		} else {
			fmt.Printf("%s: fail (%s)\n", a.Name, err)
		}
		return err
	}
}

func (m Maestro) executeChain(deps []string) error {
	var err error
	for _, d := range deps {
		a := m.Actions[d]
		if err = a.Execute(); err != nil {
			err = fmt.Errorf("%s: %s", a, err)
			break
		}
	}
	return err
}

func (m Maestro) groupDependencies(a Action) ([][]string, error) {
	if m.Nodeps {
		return nil, nil
	}
	reverse := func(xs [][]string) [][]string {
		for i, j := 0, len(xs)-1; i < len(xs)/2; i, j = i+1, j-1 {
			xs[i], xs[j] = xs[j], xs[i]
		}
		return xs
	}
	var (
		deps [][]string
		walk func([]string) error
		seen = make(map[string]struct{})
	)

	walk = func(ds []string) error {
		if len(ds) == 0 {
			return nil
		}

		all := make([]string, 0, len(ds)*4)
		for _, d := range ds {
			if d == a.Name {
				return fmt.Errorf("%s: cyclic dependency for action!", a.Name)
			}
			c, ok := m.Actions[d]
			if !ok {
				return fmt.Errorf("%s: dependency not resolved!", d)
			}
			for _, d := range c.Dependencies {
				if _, ok := seen[d]; !m.Noskip && ok {
					continue
				}
				seen[d] = struct{}{}
				all = append(all, d)
			}
		}
		var err error
		if len(all) > 0 {
			deps = append(deps, all)
			err = walk(all)
		}
		return err
	}

	deps = append(deps, a.Dependencies)
	err := walk(deps[0])
	if err == nil {
		deps = reverse(deps)
	}
	return deps, err
}

func (m Maestro) listDependencies(act Action) ([]string, error) {
	if m.Nodeps {
		return []string{act.Name}, nil
	}
	reverse := func(vs []string) []string {
		xs := make([]string, len(vs))
		copy(xs, vs)
		for i, j := 0, len(xs)-1; i < len(xs)/2; i, j = i+1, j-1 {
			xs[i], xs[j] = xs[j], xs[i]
		}
		return xs
	}
	var (
		walk func(Action, int) error
		deps []string
		seen = make(map[string]struct{})
	)

	walk = func(a Action, lvl int) error {
		if lvl > 0 && a.Name == act.Name {
			return fmt.Errorf("%s: cyclic dependency for %s action!", act.Name, a.Name)
		}
		lvl++
		deps = append(deps, a.Name)
		for _, d := range reverse(a.Dependencies) {
			if lvl > 0 && d == act.Name {
				return fmt.Errorf("%s: cyclic dependency for %s action!", act.Name, a.Name)
			}
			if _, ok := seen[d]; !m.Noskip && ok {
				continue
			}
			a, ok := m.Actions[d]
			if !ok {
				return fmt.Errorf("%s: dependency not resolved", d)
			}
			seen[d] = struct{}{}

			if err := walk(a, lvl); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(act, 0); err != nil {
		return nil, err
	}
	return reverse(deps), nil
}

func (m Maestro) ExecuteAll(args []string) error {
	var err error
	for _, a := range m.all {
		if err = m.Execute(a, args); err != nil {
			break
		}
	}
	return err
}

func (m Maestro) ExecuteDefault(args []string) error {
	switch m.cmd {
	case "all":
		return m.ExecuteAll(args)
	case "help":
		return m.Summary()
	case "version":
		return m.ExecuteVersion()
	default:
		return m.Execute(m.cmd, args)
	}
}

func (m Maestro) ExecuteExport(bin string, actions []string) error {
	if bin == "" {
		bin = m.Bin
	}
	return m.exportScripts(bin, actions)
}

func (m Maestro) ExecuteCat(actions []string) error {
	var (
		deps [][]string
		err  error
	)
	for _, a := range actions {
		act, ok := m.Actions[a]
		if !ok {
			return fmt.Errorf("%s: action not found", a)
		}
		if !m.Nodeps {
			if deps, err = m.groupDependencies(act); err != nil {
				break
			}
		}
		m.executeDry(a, deps)
	}
	return err
}

func (m Maestro) executeDry(action string, deps [][]string) {
	actions := append(flatten(deps), action)
	for i, d := range actions {
		a := m.Actions[d]

		fmt.Printf("> %s (%s)\n", a.Name, strUsage(a.Help))
		fmt.Println(a.String())
		if i < len(deps)-1 {
			fmt.Println()
		}
	}
}

func (m Maestro) ExecuteFormat() error {
	return nil
}

func (m Maestro) ExecuteVersion() error {
	fmt.Fprintln(os.Stdout, m.Version)
	return nil
}

func (m Maestro) ExecuteHelp(action string) error {
	a, ok := m.Actions[action]
	if !ok {
		return fmt.Errorf("no help available for %s", action)
	}
	return a.Usage()
}

func (m Maestro) Summary() error {
	fs := template.FuncMap{
		"usage": strUsage,
	}
	t, err := template.New("summary").Funcs(fs).Parse(strings.TrimSpace(summary))
	if err != nil {
		return err
	}
	d := struct {
		Usage string
		About string
		Tags  map[string][]Action
	}{
		Usage: m.Usage,
		About: m.About,
		Tags:  make(map[string][]Action),
	}
	for _, a := range m.Actions {
		if len(a.Tags) == 0 {
			d.Tags["miscellaneous"] = append(d.Tags["miscellaneous"], a)
			continue
		}
		for _, t := range a.Tags {
			d.Tags[t] = append(d.Tags[t], a)
		}
	}
	for t := range d.Tags {
		ts := d.Tags[t]
		sort.Slice(ts, func(i, j int) bool {
			return ts[i].Name < ts[j].Name
		})
		d.Tags[t] = ts
	}
	return t.Execute(os.Stdout, d)
}

func (m *Maestro) exportScripts(bin string, actions []string) error {
	if i, err := os.Stat(bin); err != nil || !i.IsDir() {
		return fmt.Errorf("%s: not a directory", bin)
	}
	var as []Action
	for _, a := range actions {
		act, ok := m.Actions[a]
		if !ok {
			return fmt.Errorf("%s: action not found!", a)
		}
		as = append(as, act)
	}
	if len(as) == 0 {
		for _, a := range m.Actions {
			as = append(as, a)
		}
	}
	for _, a := range as {
		if err := m.exportAction(bin, a); err != nil {
			return err
		}
	}
	return nil
}

var scriptfile = `
{{- if .Shell}}#! {{.Shell}}{{end}}

{{range .Actions}}
# {{.Name}}: {{.Help}}
{{.String}}
{{end}}
`

func (m *Maestro) exportAction(bin string, a Action) error {
	deps, err := m.listDependencies(a)
	if err != nil {
		return err
	}
	file := filepath.Join(bin, a.Name+".sh")
	w, err := os.OpenFile(file, os.O_CREATE|os.O_WRONLY, 0755)
	if err != nil {
		return err
	}
	defer w.Close()

	c := struct {
		Shell   string
		Actions []Action
	}{}
	if cmd, err := exec.LookPath(a.Shell); err == nil {
		c.Shell = cmd
	}
	for _, d := range deps {
		a, ok := m.Actions[d]
		if !ok {
			return fmt.Errorf("%s: action not found!", a)
		}
		c.Actions = append(c.Actions, a)
	}
	t, err := template.New("file").Parse(scriptfile)
	if err != nil {
		return err
	}
	return t.Execute(w, c)
}
