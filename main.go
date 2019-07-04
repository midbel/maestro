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
	flag.Parse()
	p, err := ParseFile(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(123)
	}
	if err := p.Parse(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(125)
	}
}

type Action struct {
	Name  string
	Short string
	Scope string

	Dependencies []Action

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
}

func (a Action) Execute() error {
	for _, d := range a.Dependencies {
		if err := d.Execute(); err != nil {
			return err
		}
	}
	if a.Script == "" {
		return nil
	}
	return nil
}

type Parser struct {
	lex *lexer

	globals map[string][]string
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
		globals: make(map[string][]string),
		locals:  make(map[string][]string),
	}
	p.nextToken()
	p.nextToken()

	return &p, nil
}

func (p *Parser) Parse() error {
	var err error
	for p.curr.Type != eof {
		switch p.curr.Type {
		case ident:
			switch p.peek.Type {
			case equal:
				err = p.parseIdentifier()
			case colon, lparen:
				err = p.parseAction()
			default:
				err = fmt.Errorf("syntax error: invalid token %s", p.peek)
			}
		case command:
			err = p.parseCommand()
		default:
			err = fmt.Errorf("not yet supported: %s", p.curr)
		}
		if err != nil {
			return err
		}
		p.nextToken()
	}
	return nil
}

func (p *Parser) parseAction() error {
	fmt.Println("-> parseAction:", p.curr)
	a := Action{
		Name:  p.curr.Literal,
		Scope: "",
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
	fmt.Println("-> parseCommand:", p.curr)
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
			return nil
		default:
			return fmt.Errorf("syntax error: invalid token %s", p.curr)
		}
	}
}

func (p *Parser) parseIdentifier() error {
	fmt.Println("-> parseIdentifier:", p.curr)
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
	}
	return fmt.Sprintf("<%s '%s'>", str, t.Literal)
}

// map between recognized commands and their expected number of arguments
var commands = map[string]int{
	"echo":    -1,
	"export":  2,
	"include": 1,
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
	ident
	value
	variable
	command // include, export, echo
	script
	dependency
	invalid
)

const (
	lexDefault uint16 = iota << 8
	lexValue
	lexDeps
	lexScript

	lexNoop uint16 = iota
	lexProps
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
		// x.state = lexScript | lexNoop
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
