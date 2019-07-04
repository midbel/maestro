package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

func main() {
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
	switch flag.Arg(0) {
	case "help":
		err = m.Help()
	case "version":
		err = m.Version()
	case "debug":
	case "all":
		err = m.All()
	case "":
		err = m.Default()
	default:
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
}

func (m Maestro) All() error {
	return nil
}

func (m Maestro) Default() error {
	return nil
}

func (m Maestro) Version() error {
	fmt.Fprintln(os.Stdout, m.version)
	return nil
}

func (m Maestro) Help() error {
	if m.about != "" {
		fmt.Println(m.about)
		fmt.Println()
	}
	if len(m.Actions) > 0 {
		fmt.Println("available actions:")
		fmt.Println()
		for _, a := range m.Actions {
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

func (m Maestro) executeAction(a Action) error {
	return nil
}

type Action struct {
	Name string
	Help string

	Dependencies []string
	// Dependencies []Action

	Script string

	Env     bool
	Ignore  bool
	Retry   int64
	Delay   time.Duration
	Timeout time.Duration
	Workdir string
	Shell   string // bash, sh, ksh, python,...
	Stdout  string
	Stderr  string

	// environment variables + locals variables
	locals  map[string][]string
	globals map[string]string

	echo []string
}

func (a Action) Execute() error {
	if a.Script == "" {
		return nil
	}
	return nil
}

type Parser struct {
	lex *lexer

	globals map[string]string
	locals  map[string][]string

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
	p.nextToken()
	p.nextToken()

	return &p, nil
}

func (p *Parser) Parse() (*Maestro, error) {
	var (
		err error
		mst Maestro
	)
	mst.Actions = make(map[string]Action)
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
			err = p.parseCommand()
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
		p.nextToken()
	}
	a.Script = p.curr.Literal
	for k, vs := range p.locals {
		a.locals[k] = append(a.locals[k], vs...)
	}
	for k, v := range p.globals {
		a.globals[k] = v
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
		case "shell":
			a.Shell = p.valueOf()
		case "help":
			a.Help = p.valueOf()
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

func (p *Parser) parseCommand() error {
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
				err = p.executeInclude(values)
			default:
				err = fmt.Errorf("%s: unrecognized command", ident)
			}
			return err
		default:
			return fmt.Errorf("syntax error: invalid token %s", p.curr)
		}
	}
}

func (p *Parser) executeInclude(files []string) error {
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
			p.locals[ident] = val
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
	space   = ' '
	tab     = '\t'
	period  = '.'
	colon   = ':'
	percent = '%'
	lparen  = '('
	rparen  = ')'
	comment = '#'
	quote   = '"'
	equal   = '='
	comma   = ','
	nl      = '\n'
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
	x.readRune()
	pos := x.pos
	for !isQuote(x.char) {
		x.readRune()
	}
	t.Literal, t.Type = string(x.inner[pos:x.pos]), value
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
	return x == quote
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
