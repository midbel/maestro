package maestro

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
)

const DefaultShell = "/bin/sh -c"

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
{{else}}
	try maestro help <action|namespace> for more information about that topic!
{{end}}
`

func strUsage(u string) string {
	if u == "" {
		u = "no description available"
	}
	return u
}

type Maestro struct {
	Shell string // .SHELL
	Bin   string // .BIN: directory where scripts will be written

	Parallel int  // .PARALLEL
	Echo     bool // .ECHO

	// special variables for actions
	all []string // .ALL
	cmd string   // .DEFAULT

	Name    string // .NAME
	Version string // .VERSION
	About   string // .ABOUT
	Usage   string // .USAGE
	Help    string // .HELP

	// actions
	Actions map[string]Action
	Debug   bool
	Nodeps  bool
	Noskip  bool
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
	set.BoolVar(&act.Inline, "inline", act.Inline, "inline")
	set.BoolVar(&act.Env, "env", act.Env, "environment")
	set.BoolVar(&act.Ignore, "ignore", act.Ignore, "ignore error")
	set.Int64Var(&act.Retry, "retry", act.Retry, "retry on failure")
	set.DurationVar(&act.Delay, "delay", act.Delay, "delay")
	set.DurationVar(&act.Timeout, "timeout", act.Timeout, "timeout")

	if err := set.Parse(args); err != nil {
		return err
	}

	if flag.NArg() > 0 {
		act.Args = append(act.Args, flag.Args()...)
	}

	deps, err := m.dependencies(act)
	if err != nil {
		return err
	}
	if m.Debug {
		m.executeDry(deps)
	} else {
		err = m.executeChain(deps)
	}

	return err
}

func (m Maestro) executeDry(deps []string) {
	fmt.Println(deps)
	for i, d := range deps {
		a := m.Actions[d]

		fmt.Printf("> %s (%s)\n", a.Name, strUsage(a.Help))
		fmt.Println(a.String())
		if i < len(deps)-1 {
			fmt.Println()
		}
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

func (m Maestro) dependencies(act Action) ([]string, error) {
	if m.Nodeps {
		return []string{act.Name}, nil
	}
	reverse := func(vs []string) []string {
		for i, j := 0, len(vs)-1; i < len(vs)/2; i, j = i+1, j-1 {
			vs[i], vs[j] = vs[j], vs[i]
		}
		return vs
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
	return t.Execute(os.Stdout, d)
}

func (m *Maestro) ExportScripts(bin string, actions []string) error {
	if bin != "" {
		if i, err := os.Stat(bin); err != nil || !i.IsDir() {
			return fmt.Errorf("%s: not a directory", bin)
		}
	} else {
		bin = m.Bin
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
	deps, err := m.dependencies(a)
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
