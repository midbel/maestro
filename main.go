package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const DefaultShell = "/bin/sh"

func main() {
	debug := flag.Bool("debug", false, "debug")
	file := flag.String("file", "maestro.mf", "")
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
	switch flag.Arg(0) {
	case "help":
		if act := flag.Arg(1); act == "" {
			m.Summary()
		} else {
			m.Help(act)
		}
	case "version":
		err = m.Version()
	case "all":
		err = m.All()
	case "":
		err = m.Default()
	default:
		err = m.Execute(flag.Args())
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(122)
	}
}

type Maestro struct {
	Shell string // .SHELL
	Echo  bool   // .ECHO

	// special variables for actions
	all     []string // .ALL
	cmd     string   // .DEFAULT
	version string   // .VERSION

	name  string // .NAME
	about string // .ABOUT
	usage string // .USAGE
	help  string // .HELP

	// actions
	Actions map[string]Action
	Debug   bool
}

func (m Maestro) Execute(actions []string) error {
	for _, a := range actions {
		act, ok := m.Actions[a]
		if !ok {
			return fmt.Errorf("%s: action not found!", a)
		}
		if m.Debug {
			fmt.Printf("> %s\n", a)
			fmt.Println(act.String())
		} else {
			if err := act.Execute(); err != nil {
				return fmt.Errorf("%s: %s", a, err)
			}
		}
	}
	return nil
}

func (m Maestro) All() error {
	return m.Execute(m.all)
}

func (m Maestro) Default() error {
	switch m.cmd {
	case "all":
		return m.All()
	case "help":
		return m.Summary()
	case "version":
		return m.Version()
	default:
		return m.Execute([]string{m.cmd})
	}
}

func (m Maestro) Version() error {
	fmt.Fprintln(os.Stdout, m.version)
	return nil
}

func (m Maestro) Help(action string) error {
	a, ok := m.Actions[action]
	if !ok {
		return fmt.Errorf("no help available for %s", action)
	}
	a.Usage()
	return nil
}

func (m Maestro) Summary() error {
	if m.about != "" {
		fmt.Println(m.about)
		fmt.Println()
	}
	if len(m.Actions) > 0 {
		fmt.Println("available actions:")
		fmt.Println()
		as := make([]string, 0, len(m.Actions))
		for a := range m.Actions {
			as = append(as, a)
		}
		sort.Strings(as)
		for _, a := range as {
			a := m.Actions[a]
			help := a.Help
			if help == "" {
				help = "no description available"
			}
			fmt.Printf("  %-12s %s\n", a.Name, help)
		}
		fmt.Println()
	}
	if m.usage != "" {
		fmt.Println(m.usage)
	}
	return nil
}

type Action struct {
	Name string
	Help string
	Desc string

	Dependencies []string
	// Dependencies []Action

	Script string
	Shell  string // bash, sh, ksh, python,...
	Args   []string

	Env     bool
	Ignore  bool
	Retry   int64
	Delay   time.Duration
	Timeout time.Duration
	Workdir string
	Stdout  string
	Stderr  string

	// environment variables + locals variables
	locals  map[string][]string
	globals map[string]string
	data    map[string][]string

	echo []string
}

func (a Action) Usage() {
	if a.Desc != "" {
		fmt.Println(a.Desc)
	} else {
		fmt.Println(a.Help)
	}
	fmt.Println()
	shell := a.Shell
	if len(a.Args) > 0 {
		shell = fmt.Sprintf("%s %s", a.Shell, strings.Join(a.Args, " "))
	}
	fmt.Println("properties:")
	fmt.Printf("- shell  : %s\n", shell)
	fmt.Printf("- workdir: %s\n", a.Workdir)
	fmt.Printf("- stdout : %s\n", a.Stdout)
	fmt.Printf("- stderr : %s\n", a.Stderr)
	fmt.Printf("- env    : %t\n", a.Env)
	fmt.Printf("- ignore : %t\n", a.Ignore)
	fmt.Printf("- retry  : %d\n", a.Retry)
	fmt.Printf("- delay  : %s\n", a.Delay)
	fmt.Printf("- timeout: %s\n", a.Timeout)
	if len(a.data) > 0 {
		fmt.Println()
		for k, vs := range a.data {
			fmt.Printf("%s: %s\n", k, strings.Join(vs, " "))
		}
	}
	if len(a.locals) > 0 {
		fmt.Println()
		fmt.Println("local variables:")
		for k, vs := range a.locals {
			fmt.Printf("- %s: %s\n", k, strings.Join(vs, " "))
		}
		fmt.Println()
	}
	if len(a.globals) > 0 {
		fmt.Println("environment variables:")
		for k, v := range a.globals {
			fmt.Printf("- %s: %s\n", k, v)
		}
		fmt.Println()
	}
	if len(a.Dependencies) > 0 {
		fmt.Println("dependencies:")
		for _, d := range a.Dependencies {
			fmt.Printf("- %s\n", d)
		}
	}
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
	var args []string
	if len(a.Args) == 0 {
		// args = append(args, "-c")
	} else {
		args = append(args, a.Args...)
	}
	cmd := exec.Command(a.Shell, append(args, script)...)
	if i, err := os.Stat(a.Workdir); err == nil && i.IsDir() {
		cmd.Dir = a.Workdir
	}
	cmd.Stdin = strings.NewReader(script)
	if a.Stdout == "" {
		cmd.Stdout = os.Stdout
	}
	if a.Stderr == "" {
		cmd.Stderr = os.Stderr
	}
	if a.Env {
		var es []string
		es = append(es, os.Environ()...)
		for k, v := range a.globals {
			es = append(es, fmt.Sprintf("%s=%s", k, v))
		}
	}
	if a.Delay > 0 {
		time.Sleep(a.Delay)
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
			} else if str == "PROPS" {
				// to be written
			} else if s, ok := a.locals[str]; ok {
				str = strings.Join(s, " ")
			} else if s, ok := a.data[str]; ok {
				str = strings.Join(s, " ")
			} else {
				return "", fmt.Errorf("%s: variable not defined", str)
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

	includes []string // list of files already includes; usefull for cyclic include
	globals  map[string]string
	locals   map[string][]string

	curr Token
	peek Token
}

func ParseFile(file string) (*Parser, error) {
	r, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	lex, err := Lex(r)
	if err != nil {
		return nil, err
	}
	p := Parser{
		lex:     lex,
		globals: make(map[string]string),
		locals:  make(map[string][]string),
	}
	p.includes = append(p.includes, file)
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
	// fmt.Println("-> parseAction:", p.curr)
	a := Action{
		Name:    p.curr.Literal,
		locals:  make(map[string][]string),
		globals: make(map[string]string),
		data:    make(map[string][]string),
	}
	p.nextToken()
	if p.curr.Type == lparen {
		// parsing action properties
		if err := p.parseProperties(&a); err != nil {
			return err
		}
	}
	if p.peek.Type == ident {
		return nil
	}
	p.nextToken()
	for p.curr.Type == dependency {
		a.Dependencies = append(a.Dependencies, p.curr.Literal)
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
	m.Actions[a.Name] = a
	return nil
}

func (p *Parser) parseProperties(a *Action) error {
	p.nextToken() // consuming '(' token

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
			// lit = p.curr.Literal
			// a.data[lit] = append(a.data[lit], p.valueOf())
		case "shell":
			a.Shell = p.valueOf()
		case "help":
			a.Help = p.valueOf()
		case "desc":
			a.Desc = p.valueOf()
		case "env":
			a.Env, err = strconv.ParseBool(p.valueOf())
		case "ignore":
			a.Ignore, err = strconv.ParseBool(p.valueOf())
		case "retry":
			a.Retry, err = strconv.ParseInt(p.valueOf(), 0, 64)
		case "timeout":
			a.Timeout, err = time.ParseDuration(p.valueOf())
		case "delay":
			a.Delay, err = time.ParseDuration(p.valueOf())
		case "workdir":
			a.Workdir = p.valueOf()
		case "stdout":
			a.Stdout = p.valueOf()
		case "stderr":
			a.Stderr = p.valueOf()
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

func (p *Parser) valueOf() string {
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

func (p *Parser) parseCommand(m *Maestro) error {
	// fmt.Println("-> parseCommand:", p.curr)
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
	// fmt.Println("-> parseMeta:", p.curr)
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
		m.name = lit
	case "VERSION":
		m.version = lit
	case "HELP":
		m.help = lit
	case "USAGE":
		m.usage = lit
	case "ABOUT":
		m.about = lit
	case "DEFAULT":
		m.cmd = lit
	case "ECHO":
	}
	if p.peek.Type != nl {
		return fmt.Errorf("syntax error: invalid token %s", p.peek)
	}
	p.nextToken()
	return nil
}

func (p *Parser) parseIdentifier() error {
	// fmt.Println("-> parseIdentifier:", p.curr)
	ident := p.curr.Literal

	p.nextToken() // consuming '=' token
	for {
		p.nextToken()
		switch p.curr.Type {
		case value:
			p.locals[ident] = append(p.locals[ident], p.curr.Literal)
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

func (x *lexer) nextValue(t *Token) {
	if x.char == space {
		x.skipSpace()
	}
	switch {
	case x.char == nl || x.char == comma || x.char == rparen:
		t.Type = x.char
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
	} else if x.char == nl {
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
