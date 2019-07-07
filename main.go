package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"time"
	"unicode/utf8"

	"golang.org/x/sync/errgroup"
)

const DefaultShell = "/bin/sh -c"

func main() {
	debug := flag.Bool("debug", false, "debug")
	echo := flag.Bool("echo", false, "echo")
	export := flag.Bool("export", false, "export")
	bindir := flag.String("bin", "", "scripts directory")
	// incl := flag.String("include", "", "")
	file := flag.String("file", "maestro.mf", "")
	nodeps := flag.Bool("nodeps", false, "don't execute command dependencies")
	noskip := flag.Bool("noskip", false, "execute an action even if already executed")
	flag.Parse()

	p, err := ParseFile(*file)
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

var usage = `
{{- if .Desc}}
	{{- .Desc}}
{{else}}
	{{- .Help}}

{{ end -}}
Properties:

- shell  : {{.Shell}}
- workdir: {{.Workdir}}
- stdout : {{.Stdout}}
- stderr : {{.Stderr}}
- env    : {{.Env}}
- ignore : {{.Ignore}}
- retry  : {{.Retry}}
- delay  : {{.Delay}}
- timeout: {{.Timeout}}

{{if .Locals -}}
Local variables:

{{range $k, $vs := .Locals}}
	{{- printf "- %-12s: %s" $k (join $vs " ")}}
{{end}}
{{- end}}
{{- if .Globals}}
Environment Variables:

{{range $k, $v := .Globals}}
	{{- printf "- %-12s: %s" $k $v}}
{{end}}
{{- end}}
{{if .Dependencies}}
Dependencies:
{{range .Dependencies}}
- {{ . -}}
{{end}}
{{end}}
`

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

	deps, err := m.dependencies(act)
	if err != nil {
		return err
	}
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

type MultiAction struct {
	actions  []Action
	parallel int
}

func (m MultiAction) Execute() error {
	if len(m.actions) == 1 {
		return m.executeSingle(0)
	}
	if m.parallel <= 0 {
		return m.executeSequential()
	}
	var (
		group errgroup.Group
		sema  = make(chan struct{}, m.parallel)
	)
	for i := range m.actions {
		sema <- struct{}{}
		j := i
		group.Go(func() error {
			err := m.executeSingle(j)
			<-sema
			return err
		})
	}
	return group.Wait()
}

func (m MultiAction) executeSequential() error {
	var err error
	for i := range m.actions {
		if err = m.executeSingle(i); err != nil {
			break
		}
	}
	return err
}

func (m MultiAction) executeSingle(i int) error {
	return m.actions[i].Execute()
}

type Action struct {
	Name string
	Help string
	Desc string
	Tags []string

	Dependencies []string
	// Dependencies []Action

	Script string
	Shell  string // bash, sh, ksh, python,...

	Inline  bool
	Env     bool
	Ignore  bool
	Retry   int64
	Delay   time.Duration
	Timeout time.Duration
	Workdir string
	Stdout  string
	Stderr  string

	// command could be repeated X times (could be in parallel)
	// Repeat   int
	// Parallel bool

	// environment variables + locals variables
	locals  map[string][]string
	globals map[string]string
}

func (a Action) Usage() error {
	fs := template.FuncMap{
		"join": strings.Join,
	}
	t, err := template.New("usage").Funcs(fs).Parse(strings.TrimSpace(usage))
	if err != nil {
		return err
	}
	d := struct {
		Action
		Locals  map[string][]string
		Globals map[string]string
	}{
		Action:  a,
		Locals:  a.locals,
		Globals: a.globals,
	}
	return t.Execute(os.Stdout, d)
}

func (a Action) String() string {
	s, err := a.prepareScript()
	if err != nil {
		s = err.Error()
	}
	return s
}

func (a Action) Execute() error {
	script, err := a.prepareScript()
	if err != nil {
		return err
	}
	args := ParseShell(a.Shell)
	if len(args) == 0 {
		return fmt.Errorf("%s: fail to parse shell", a.Shell)
	}

	for i := int64(0); i < a.Retry; i++ {
		if err = a.executeScript(args, script); err == nil {
			break
		}
	}
	if a.Ignore && err != nil {
		err = nil
	}
	return err
}

func (a Action) executeScript(args []string, script string) error {

	if a.Delay > 0 {
		time.Sleep(a.Delay)
	}
	if a.Inline {
		args = append(args, script)
	}
	cmd := exec.Command(args[0], args[1:]...)
	// cmd := exec.Command(args[0], append(args[1:], script)...)
	if i, err := os.Stat(a.Workdir); err == nil && i.IsDir() {
		cmd.Dir = a.Workdir
	} else {
		if a.Workdir != "" {
			return fmt.Errorf("%s: not a directory", a.Workdir)
		}
	}

	if !a.Inline {
		cmd.Stdin = strings.NewReader(script)
	}
	openFD := func(n string, w io.Writer) (io.Writer, error) {
		if n == "" {
			return w, nil
		} else if n == "discard" || n == "-" {
			return ioutil.Discard, nil
		} else {
			return os.OpenFile(n+".err", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		}
	}
	var err error
	if cmd.Stdout, err = openFD(a.Stdout, os.Stdout); err != nil {
		return err
	}
	if cmd.Stderr, err = openFD(a.Stderr, os.Stderr); err != nil {
		return err
	}

	if a.Env {
		cmd.Env = append(cmd.Env, os.Environ()...)
		for k, v := range a.globals {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}
	return cmd.Run()
}

func (a Action) prepareScript() (string, error) {
	var (
		b strings.Builder
		n int
	)

	script := []byte(a.Script)
	for {
		k, nn := utf8.DecodeRune(script[n:])
		if k == utf8.RuneError {
			if nn == 0 {
				break
			} else {
				return "", fmt.Errorf("invalid character found in script!!!")
			}
		}
		n += nn
		if k == percent {
			x := n
			for k != rparen {
				k, nn = utf8.DecodeRune(script[x:])
				x += nn
			}
			str := strings.Trim(string(script[n:x]), "()")
			if len(str) == 0 {
				return "", fmt.Errorf("script: invalid syntax")
			}
			if str == "TARGET" {
				str = a.Name
			} else if s, ok := a.locals[str]; ok {
				str = strings.Join(s, " ")
			} else {
				switch str {
				case "shell":
					str = a.Shell
				case "workdir":
					str = a.Workdir
				case "stdout":
					str = a.Stdout
				case "stderr":
					str = a.Stderr
				case "env":
					str = strconv.FormatBool(a.Env)
				case "ignore":
					str = strconv.FormatBool(a.Ignore)
				case "retry":
					str = strconv.FormatInt(a.Retry, 10)
				case "delay":
					str = a.Delay.String()
				case "timeout":
					str = a.Timeout.String()
				default:
					return "", fmt.Errorf("%s: variable not defined", str)
				}
			}
			b.WriteString(str)
			n = x
		} else {
			b.WriteRune(k)
		}
	}
	return b.String(), nil
}

type Parser struct {
	lex *lexer

	includes []string // list of files already includes; usefull to detect cyclic include
	globals  map[string]string
	locals   map[string][]string

	curr Token
	peek Token

	frames []*frame
}

func ParseFile(file string) (*Parser, error) {
	r, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	p, err := parseReader(r)
	if err == nil {
		p.includes = append(p.includes, file)
	}
	return p, err
}

// func ParseFileWithIncludes(file string, inc ...string) (*Parser, error) {
// 	// 1: parse all files to be included
// 	// 2: parse main file
// 	return nil, nil
// }

func parseReader(r io.Reader) (*Parser, error) {
	lex, err := Lex(r)
	if err != nil {
		return nil, err
	}
	p := Parser{
		lex:     lex,
		globals: make(map[string]string),
		locals:  make(map[string][]string),
	}
	p.nextToken()
	p.nextToken()

	return &p, nil
}

func (p *Parser) parseFile(file string, mst *Maestro) error {
	sort.Strings(p.includes)
	ix := sort.SearchStrings(p.includes, file)
	if ix < len(p.includes) && p.includes[ix] == file {
		return fmt.Errorf("%s: cyclic include detected!", file)
	}
	p.includes = append(p.includes, file)

	r, err := os.Open(file)
	if err != nil {
		return err
	}
	defer r.Close()

	x, err := Lex(r)
	if err != nil {
		return err
	}
	// save current state of parser
	curr, peek := p.curr, p.peek
	x, p.lex = p.lex, x

	// init curr and peek token from  new lexer
	p.nextToken()
	p.nextToken()

	if m, err := p.Parse(); err != nil {
		return err
	} else {
		for k, a := range m.Actions {
			mst.Actions[k] = a
		}
	}

	// restore state of parser (TODO: using kind of frame could make code cleaner)
	p.lex = x
	p.curr, p.peek = curr, peek

	return nil
}

func (p *Parser) Parse() (*Maestro, error) {
	mst := Maestro{
		Actions: make(map[string]Action),
		Shell:   DefaultShell,
	}

	var err error
	for p.curr.Type != eof {
		switch p.curr.Type {
		case meta:
			err = p.parseMeta(&mst)
		case ident:
			switch p.peek.Type {
			case equal:
				err = p.parseIdentifier()
			case colon, lparen:
				err = p.parseAction(&mst)
			default:
				err = fmt.Errorf("syntax error: invalid token %s", p.peek)
			}
		case command:
			err = p.parseCommand(&mst)
		default:
			err = fmt.Errorf("not yet supported: %s", p.curr)
		}
		if err != nil {
			return nil, err
		}
		p.nextToken()
	}
	return &mst, nil
}

func (p *Parser) parseAction(m *Maestro) error {
	a := Action{
		Name:    p.curr.Literal,
		locals:  make(map[string][]string),
		globals: make(map[string]string),
	}
	p.nextToken()
	if p.curr.Type == lparen {
		// parsing action properties
		if err := p.parseProperties(&a); err != nil {
			return err
		}
	}
	if p.peek.Type == ident {
		m.Actions[a.Name] = a
		return nil
	}
	p.nextToken()
	for p.curr.Type == dependency {
		a.Dependencies = append(a.Dependencies, p.curr.Literal)
		if p.peek.Type == plus {
			p.nextToken()
		}
		p.nextToken()
	}
	a.Script = p.curr.Literal
	for k, vs := range p.locals {
		switch k {
		case "help", "desc":
		default:
			a.locals[k] = append(a.locals[k], vs...)
		}
	}
	for k, v := range p.globals {
		a.globals[k] = v
	}
	if a.Shell == "" {
		a.Shell = m.Shell
	}
	if a.Retry <= 0 {
		a.Retry = 1
	}
	m.Actions[a.Name] = a
	return nil
}

func (p *Parser) parseProperties(a *Action) error {
	p.nextToken() // consuming '(' token

	valueOf := func() string {
		var str string
		switch p.curr.Type {
		case value:
			str = p.curr.Literal
		case variable:
			vs, ok := p.locals[p.curr.Literal]
			if ok && len(vs) >= 1 {
				str = vs[0]
			}
		}
		return str
	}

	var err error
	for p.curr.Type != rparen {
		lit := p.curr.Literal
		p.nextToken()
		if p.curr.Type != equal {
			return fmt.Errorf("syntax error: invalid token %s", p.curr)
		}
		p.nextToken()
		switch strings.ToLower(lit) {
		default:
			err = fmt.Errorf("%s: unknown option %s", a.Name, p.curr.Literal)
		case "tag":
			a.Tags = append(a.Tags, valueOf())
		case "shell":
			a.Shell = valueOf()
		case "help":
			a.Help = valueOf()
		case "desc":
			a.Desc = valueOf()
		case "env":
			a.Env, err = strconv.ParseBool(valueOf())
		case "ignore":
			a.Ignore, err = strconv.ParseBool(valueOf())
		case "retry":
			a.Retry, err = strconv.ParseInt(valueOf(), 0, 64)
			if err == nil && a.Retry <= 0 {
				a.Retry = 1
			}
		case "timeout":
			a.Timeout, err = time.ParseDuration(valueOf())
		case "delay":
			a.Delay, err = time.ParseDuration(valueOf())
		case "workdir":
			a.Workdir = valueOf()
		case "stdout":
			a.Stdout = valueOf()
		case "stderr":
			a.Stderr = valueOf()
		case "inline":
			a.Inline, err = strconv.ParseBool(valueOf())
		}
		if err != nil {
			return err
		}
		if p.peek.Type == comma {
			p.nextToken()
		}
		p.nextToken()
	}
	if p.peek.Type != colon {
		return fmt.Errorf("syntax error: invalid token %s", p.peek)
	} else {
		p.nextToken()
	}
	return nil
}

func (p *Parser) parseCommand(m *Maestro) error {
	ident := p.curr.Literal
	n, ok := commands[ident]
	if !ok {
		return fmt.Errorf("%s: unknown command", ident)
	}
	x := n
	if x < 0 {
		x = 0
	}
	values := make([]string, 0, x)
	for {
		p.nextToken()
		switch p.curr.Type {
		case value:
			values = append(values, p.curr.Literal)
		case variable:
			val, ok := p.locals[p.curr.Literal]
			if !ok {
				return fmt.Errorf("%s: not defined", p.curr.Literal)
			}
			values = append(values, val...)
		case nl:
			if n >= 0 && len(values) != n {
				return fmt.Errorf("%s: wrong number of arguments (want: %d, got %d)", ident, n, len(values))
			}
			var err error
			switch ident {
			case "clear":
				err = p.executeClear()
			case "export":
				err = p.executeExport(values)
			case "declare":
				err = p.executeDeclare(values)
			case "include":
				err = p.executeInclude(m, values)
			default:
				err = fmt.Errorf("%s: unrecognized command", ident)
			}
			return err
		default:
			return fmt.Errorf("syntax error: invalid token %s", p.curr)
		}
	}
}

func (p *Parser) executeInclude(m *Maestro, files []string) error {
	for _, f := range files {
		if err := p.parseFile(f, m); err != nil {
			return err
		}
	}
	return nil
}

func (p *Parser) executeDeclare(values []string) error {
	for _, v := range values {
		vs, ok := p.locals[v]
		if ok {
			return fmt.Errorf("declare: %s already declared", v)
		}
		p.locals[v] = vs
	}
	return nil
}

func (p *Parser) executeExport(values []string) error {
	if len(values) != 2 {
		return fmt.Errorf("export: wrong number of arguments")
	}
	p.globals[values[0]] = values[1]
	return nil
}

func (p *Parser) executeClear() error {
	p.locals = make(map[string][]string)
	p.globals = make(map[string]string)
	return nil
}

func (p *Parser) parseMeta(m *Maestro) error {
	ident := p.curr.Literal

	p.nextToken()
	if p.curr.Type != equal {
		return fmt.Errorf("syntax error: invalid token %s", p.curr)
	}
	p.nextToken()
	if p.curr.Type != value {
		return fmt.Errorf("syntax error: invalid token %s", p.curr)
	}
	switch lit := p.curr.Literal; ident {
	case "ALL":
		for p.peek.Type != nl {
			if p.curr.Type != value {
				return fmt.Errorf("syntax error: invalid token %s", p.curr)
			}
			m.all = append(m.all, p.curr.Literal)
			p.nextToken()
		}
	case "NAME":
		m.Name = lit
	case "VERSION":
		m.Version = lit
	case "HELP":
		m.Help = lit
	case "USAGE":
		m.Usage = lit
	case "ABOUT":
		m.About = lit
	case "DEFAULT":
		m.cmd = lit
	case "ECHO":
	case "PARALLEL":
		n, err := strconv.ParseInt(lit, 10, 64)
		if err != nil && lit != "-" {
			return err
		}
		if lit == "-" {
			m.Parallel = -1
		} else {
			m.Parallel = int(n)
		}
	}
	if p.peek.Type != nl {
		return fmt.Errorf("syntax error: invalid token %s", p.peek)
	}
	p.nextToken()
	return nil
}

func (p *Parser) parseIdentifier() error {
	ident := p.curr.Literal

	p.nextToken() // consuming '=' token
	for {
		p.nextToken()
		switch p.curr.Type {
		case value:
			switch lit := p.curr.Literal; lit {
			case "-":
				p.locals[ident] = p.locals[ident][:0]
			case "":
				delete(p.locals, ident)
			default:
				p.locals[ident] = append(p.locals[ident], lit)
			}
		case variable:
			val, ok := p.locals[p.curr.Literal]
			if !ok {
				return fmt.Errorf("%s: not defined", p.curr.Literal)
			}
			p.locals[ident] = append(p.locals[ident], val...)
		case nl:
			return nil
		default:
			return fmt.Errorf("syntax error: invalid token %s", p.curr)
		}
	}
}

func (p *Parser) nextToken() {
	p.curr = p.peek
	p.peek = p.lex.Next()
}

func (p *Parser) pushFrame(file string) error {
	r, err := os.Open(file)
	if err != nil {
		return err
	}
	x, err := Lex(r)
	if err == nil {
		f := frame{
			lex:     x,
			locals:  make(map[string][]string),
			globals: make(map[string]string),
		}
		f.nextToken()
		f.nextToken()

		p.frames = append(p.frames, &f)
	}
	return err
}

func (p *Parser) popFrame() {
	if len(p.frames) == 0 {
		return
	}
	n := len(p.frames) - 1
	if n < 0 {
		return
	}
	p.frames = p.frames[:n]
}

type frame struct {
	lex *lexer

	locals  map[string][]string
	globals map[string]string

	curr Token
	peek Token
}

func (f *frame) nextToken() {
	f.curr = f.peek
	f.peek = f.lex.Next()
}

func ParseShell(str string) []string {
	const (
		single byte = '\''
		double      = '"'
		space       = ' '
		equal       = '='
	)

	skipN := func(b byte) int {
		var i int
		if b == space || b == single || b == double {
			i++
		}
		return i
	}

	var (
		ps  []string
		j   int
		sep byte = space
	)
	for i := 0; i < len(str); i++ {
		if str[i] == sep || str[i] == equal {
			if i > j {
				j += skipN(str[j])
				ps, j = append(ps, str[j:i]), i+1
				if sep == single || sep == double {
					sep = space
				}
			}
			continue
		}
		if sep == space && (str[i] == single || str[i] == double) {
			sep, j = str[i], i+1
		}
	}
	if str := str[j:]; len(str) > 0 {
		i := skipN(str[0])
		ps = append(ps, str[i:])
	}
	return ps
}

type Token struct {
	Literal string
	Type    rune
}

func (t Token) String() string {
	var str string
	switch t.Type {
	default:
		return fmt.Sprintf("<punct '%c'>", t.Type)
	case nl:
		return "<NL>"
	case command:
		str = "command"
	case ident:
		str = "ident"
	case variable:
		str = "variable"
	case value:
		str = "value"
	case script:
		str = "script"
	case dependency:
		str = "dependency"
	case meta:
		str = "meta"
	}
	return fmt.Sprintf("<%s '%s'>", str, t.Literal)
}

// map between recognized commands and their expected number of arguments
var commands = map[string]int{
	"echo":    -1,
	"declare": -1,
	"export":  2,
	"include": 1,
	"clear":   0,
}

const (
	space     = ' '
	tab       = '\t'
	period    = '.'
	colon     = ':'
	percent   = '%'
	lparen    = '('
	rparen    = ')'
	comment   = '#'
	quote     = '"'
	tick      = '`'
	equal     = '='
	comma     = ','
	nl        = '\n'
	backslash = '\\'
	plus      = '+'
	minus     = '-'
)

const (
	eof rune = -(iota + 1)
	meta
	ident
	value
	variable
	command // include, export, echo, declare
	script
	dependency
	invalid
)

const (
	lexDefault uint16 = iota << 8
	lexValue
	lexDeps
	lexScript
)

const (
	lexNoop uint16 = iota
	lexProps
	lexMeta
)

type lexer struct {
	inner []byte

	state uint16

	char rune
	pos  int
	next int
}

func Lex(r io.Reader) (*lexer, error) {
	xs, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	x := lexer{
		inner: xs,
		state: lexDefault,
	}
	x.readRune()
	return &x, nil
}

func (x *lexer) Next() Token {
	var t Token
	if x.char == eof || x.char == invalid {
		t.Type = x.char
		return t
	}
	switch state := x.state & 0xFF00; state {
	case lexValue:
		x.nextValue(&t)
		if state, peek := x.state&0xFF, x.peekRune(); state == lexProps && isSpace(peek) {
			x.readRune()
			x.skipSpace()
			x.unreadRune()
		}
	case lexScript:
		x.nextScript(&t)
	case lexDeps:
		x.nextDependency(&t)
	default:
		x.nextDefault(&t)
	}
	switch t.Type {
	case colon:
		x.state = lexDeps | lexNoop
	case equal, command:
		x.state |= lexValue
	case lparen, comma:
		x.state = lexDefault | lexProps
	case nl:
		if state := x.state & 0xFF00; state == lexDeps {
			x.state |= lexScript
			return x.Next()
		} else {
			x.state = lexDefault | lexNoop
		}
	case rparen:
		x.state = lexDefault | lexNoop
	case script:
		x.state = lexDefault | lexNoop
		if t.Literal == "" {
			return x.Next()
		}
	}
	x.readRune()
	return t
}

func (x *lexer) nextScript(t *Token) {
	done := func() bool {
		if x.char == eof {
			return true
		}
		peek := x.peekRune()
		return x.char == nl && (!isSpace(peek) || peek == eof || peek == comment)
	}

	var str strings.Builder
	for !done() {
		if peek := x.peekRune(); x.char == nl && peek != nl {
			str.WriteRune(x.char)
			x.readRune()
			x.skipSpace()
		}
		str.WriteRune(x.char)
		x.readRune()
	}
	t.Literal, t.Type = strings.TrimSpace(str.String()), script
}

func (x *lexer) nextValue(t *Token) {
	if x.char == space {
		x.skipSpace()
	}
	switch {
	case x.char == nl || x.char == comma || x.char == rparen:
		t.Type = x.char
	case x.char == minus:
		t.Literal, t.Type = "-", value
	case isQuote(x.char):
		x.readString(t)
	case x.char == percent:
		x.readVariable(t)
	default:
		x.readValue(t)
	}
}

func (x *lexer) nextDependency(t *Token) {
	if x.char == space {
		x.skipSpace()
	}
	if isIdent(x.char) {
		x.readIdent(t)
		t.Type = dependency
	} else if x.char == nl || x.char == plus {
		t.Type = x.char
	} else {
		t.Type = invalid
	}
}

func (x *lexer) nextDefault(t *Token) {
	x.skipSpace()
	switch {
	case isIdent(x.char):
		x.readIdent(t)
	case isQuote(x.char):
		x.readString(t)
	case isComment(x.char):
		x.skipComment()
		x.nextDefault(t)
	case x.char == period:
		x.readRune()
		x.readIdent(t)
		t.Type = meta
	default:
		t.Type = x.char
	}
}

func (x *lexer) countRuneUntil(fn func(rune) bool) int {
	var (
		i int
		n = x.pos
	)
	for {
		k, nn := utf8.DecodeRune(x.inner[n:])
		if fn(k) || k == utf8.RuneError {
			break
		}
		n += nn
		i++
	}
	return i
}

func (x *lexer) readVariable(t *Token) {
	x.readRune()
	if x.char != lparen {
		t.Type = invalid
		return
	}
	x.readRune()

	pos := x.pos
	for x.char != rparen {
		if x.char == space || x.char == nl {
			t.Type = invalid
			return
		}
		x.readRune()
	}
	t.Literal, t.Type = string(x.inner[pos:x.pos]), variable
}

func (x *lexer) readIdent(t *Token) {
	pos := x.pos
	for isIdent(x.char) || isDigit(x.char) {
		x.readRune()
	}
	t.Literal, t.Type = string(x.inner[pos:x.pos]), ident
	if _, ok := commands[t.Literal]; ok {
		t.Type = command
	}
	x.unreadRune()
}

func (x *lexer) readValue(t *Token) {
	pos := x.pos
	for {
		switch x.char {
		case space, nl, comma, rparen:
			t.Literal, t.Type = string(x.inner[pos:x.pos]), value
			x.unreadRune()

			return
		default:
			x.readRune()
		}
	}
}

func (x *lexer) readString(t *Token) {
	ticky := x.char == tick
	var eos rune
	if ticky {
		eos = tick
	} else {
		eos = quote
	}
	x.readRune()

	var b strings.Builder
	for x.char != eos {
		if !ticky && x.char == backslash {
			if peek := x.peekRune(); peek == quote {
				x.readRune()
			}
		}
		b.WriteRune(x.char)
		x.readRune()
	}
	t.Literal, t.Type = b.String(), value
	if ticky {
		t.Literal = strings.TrimLeft(t.Literal, "\n\t ")
	}
	// t.Literal, t.Type = string(x.inner[pos:x.pos]), value
}

func (x *lexer) readRune() {
	if x.pos > 0 {
		if x.char == eof || x.char == invalid {
			return
		}
	}
	k, n := utf8.DecodeRune(x.inner[x.next:])
	if k == utf8.RuneError {
		if n == 0 {
			x.char = eof
		} else {
			x.char = invalid
		}
	} else {
		x.char = k
	}
	x.pos = x.next
	x.next += n
}

func (x *lexer) unreadRune() {
	x.next = x.pos
	x.pos -= utf8.RuneLen(x.char)
}

func (x *lexer) peekRune() rune {
	k, _ := utf8.DecodeRune(x.inner[x.next:])
	return k
}

func (x *lexer) skipComment() {
	for x.char != nl {
		x.readRune()
	}
}

func (x *lexer) skipSpace() {
	for isSpace(x.char) {
		x.readRune()
	}
}

func isQuote(x rune) bool {
	return x == quote || x == tick
}

func isSpace(x rune) bool {
	return x == space || x == tab || x == nl
}

func isComment(x rune) bool {
	return x == comment
}

func isIdent(x rune) bool {
	return (x >= 'a' && x <= 'z') || (x >= 'A' && x <= 'Z')
}

func isDigit(x rune) bool {
	return x >= '0' && x <= '9'
}
