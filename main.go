package main

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

func main() {
	var incl includes
	flag.Var(&incl, "i", "include files")
	file := flag.String("f", "maestro.mf", "")

	debug := flag.Bool("debug", false, "debug")
	echo := flag.Bool("echo", false, "echo")
	export := flag.Bool("export", false, "export")
	bindir := flag.String("bin", "", "scripts directory")
	nodeps := flag.Bool("nodeps", false, "don't execute command dependencies")
	noskip := flag.Bool("noskip", false, "execute an action even if already executed")
	flag.Parse()

	p, err := Parse(*file, []string(incl)...)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(123)
	}
	m, err := p.Parse()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(125)
	}
	m.Debug = *debug
	m.Nodeps = *nodeps
	m.Noskip = *noskip
	m.Echo = *echo

	if *export {
		if err := m.ExportScripts(*bindir, flag.Args()); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(125)
		}
		return
	}

	switch action, args := flag.Arg(0), arguments(flag.Args()); action {
	case "help", "":
		if act := flag.Arg(1); act == "" {
			err = m.Summary()
		} else {
			err = m.ExecuteHelp(act)
		}
	case "version":
		err = m.ExecuteVersion()
	case "all":
		err = m.ExecuteAll(args)
	case "default":
		err = m.ExecuteDefault(args)
	default:
		err = m.Execute(action, args)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(122)
	}
}

func arguments(args []string) []string {
	return args[1:]
}

var summary = `
{{usage .About}}
{{if .Actions}}
Available actions:

{{range $k, $v := .Actions}}
	{{- printf "  %-12s %s" $k (usage $v.Help)}}
{{end}}
{{end}}

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

	if cmd, err := exec.LookPath(a.Shell); err == nil {
		fmt.Fprintf(w, "#! %s\n", cmd)
		fmt.Fprintln(w)
	}
	for i, d := range deps {
		a, ok := m.Actions[d]
		if !ok {
			return fmt.Errorf("%s: action not found!", a)
		}
		fmt.Fprintf(w, "# %s: %s\n", a.Name, a.Help)
		fmt.Fprintln(w, a.String())
		if i < len(deps)-1 {
			fmt.Fprintln(w)
		}
	}

	if err := w.Close(); err != nil {
		os.Remove(file)
		return err
	}
	return nil
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
	// fmt.Println(deps)
	for _, d := range deps {
		a := m.Actions[d]
		if m.Debug {
			fmt.Printf("> %s (%s)\n", a.Name, a.Help)
			fmt.Println(a.String())
		} else {
			if err := a.Execute(); err != nil {
				return fmt.Errorf("%s: %s", a, err)
			}
		}
	}

	return nil
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
	return t.Execute(os.Stdout, m)
}
