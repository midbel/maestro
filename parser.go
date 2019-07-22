package maestro

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/midbel/xxh"
)

type Parser struct {
	// hashes of files already includes; usefull to detect cyclic include
	includes map[uint64]struct{}

	globals map[string]string
	locals  map[string][]string

	frames []*frame
}

func Parse(file string, is ...string) (*Maestro, error) {
	p := Parser{
		includes: make(map[uint64]struct{}),
		globals:  make(map[string]string),
		locals:   make(map[string][]string),
	}
	if err := p.pushFrame(file); err != nil {
		return nil, err
	}
	for i := len(is) - 1; i >= 0; i-- {
		if err := p.pushFrame(is[i]); err != nil {
			return nil, err
		}
	}
	// p.nextToken()
	p.nextToken()

	return p.Parse()
}

func (p *Parser) Parse() (*Maestro, error) {
	mst := Maestro{
		Actions: make(map[string]Action),
		Shell:   DefaultShell,
	}

	var err error
	for p.currType() != eof {
		switch p.currType() {
		case meta:
			err = p.parseMeta(&mst)
		case ident:
			switch p.peekType() {
			case equal:
				err = p.parseIdentifier()
			case colon, lparen:
				err = p.parseAction(&mst)
			default:
				err = p.peekError()
			}
		case comment:
			// ignore by the parser
		case command:
			err = p.parseCommand(&mst)
		default:
			err = p.currError()
		}
		if err != nil {
			return nil, err
		}
		p.nextToken()
	}
	return &mst, nil
}

func (p *Parser) debugTokens() (curr Token, peek Token) {
	if n := len(p.frames) - 1; n >= 0 {
		f := p.frames[n]
		curr, peek = f.curr, f.peek
	}
	return
}

func (p *Parser) parseFile(file string, mst *Maestro) error {
	p.nextToken()
	return p.pushFrame(file)
}

func (p *Parser) parseAction(m *Maestro) error {
	a := Action{
		Name:    p.currLiteral(),
		Shell:   m.Shell,
		locals:  make(map[string][]string),
		globals: make(map[string]string),
	}

	if err := p.nextExpect(lparen); err == nil {
		if err = p.parseProperties(&a); err != nil {
			return err
		}
	}
	if p.peekIs(ident) {
		m.Actions[a.Name] = a
		return nil
	}
	p.nextToken()
	for p.currIs(dependency) {
		a.Dependencies = append(a.Dependencies, p.currLiteral())
		if !p.peekIs(dependency) {
			break
		}
		p.nextToken()
	}
	if p.peekIs(script) {
		p.nextToken()
		a.Script = p.currLiteral()
	}
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
	if a.Retry <= 0 {
		a.Retry = 1
	}
	m.Actions[a.Name] = a
	return nil
}

func (p *Parser) parseProperties(a *Action) error {
	p.nextToken() // consuming '(' token

	valueOf := func() string {
		p.nextToken()
		var str string
		switch lit := p.currLiteral(); p.currType() {
		default:
		case value:
			str = lit
		case variable:
			vs, ok := p.locals[lit]
			if ok && len(vs) >= 1 {
				str = vs[0]
			}
		}
		return str
	}

	var err error
	for !p.currIs(rparen) {
		lit := p.currLiteral()
		if err := p.nextExpect(equal); err != nil {
			return err
		}

		switch strings.ToLower(lit) {
		default:
			err = fmt.Errorf("%s: unknown option %s", a.Name, lit)
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
		case "hazardous":
			a.Hazard, err = strconv.ParseBool(valueOf())
		}
		if err != nil {
			return err
		}
		if err := p.peekExpect(comma); err == nil {
			// p.nextToken()
		}
		p.nextToken()
	}
	return p.peekExpect(colon)
}

func (p *Parser) parseCommand(m *Maestro) error {
	ident := p.currLiteral()
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
		switch p.currType() {
		case value:
			values = append(values, p.currLiteral())
		case variable:
			val, ok := p.locals[p.currLiteral()]
			if !ok {
				return fmt.Errorf("%s: not defined", p.currLiteral())
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
			return p.currError()
		}
	}
}

func (p *Parser) parseMeta(m *Maestro) error {
	ident := p.currLiteral()

	if err := p.nextExpect(equal, value); err != nil {
		return err
	}
	switch lit := p.currLiteral(); ident {
	case "ALL":
		for !p.peekIs(nl) {
			if !p.currIs(value) {
				return p.currError()
			}
			m.all = append(m.all, p.currLiteral())
			p.nextToken()
		}
	case "SHELL":
		m.Shell = lit
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
	case "BEGIN":
		m.Begin = lit
	case "END":
		m.End = lit
	case "SUCCESS":
		m.Success = lit
	case "FAILURE":
		m.Failure = lit
	case "ECHO":
		echo, err := strconv.ParseBool(lit)
		if err != nil {
			return err
		}
		m.Echo = echo
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
	return p.nextExpect(nl)
}

func (p *Parser) parseIdentifier() error {
	ident := p.currLiteral()

	p.nextToken() // consuming '=' token
	var values []string
	for {
		p.nextToken()
		switch p.currType() {
		case value:
			switch lit := p.currLiteral(); lit {
			case "-":
				p.locals[ident] = p.locals[ident][:0]
				p.nextUntil(nl)
				return nil
			case "":
				delete(p.locals, ident)
				p.nextUntil(nl)
				return nil
			default:
				values = append(values, lit)
				// p.locals[ident] = append(p.locals[ident][:0], lit)
			}
		case variable:
			val, ok := p.locals[p.currLiteral()]
			if !ok {
				return fmt.Errorf("%s: not defined", p.currLiteral())
			}
			values = append(values, val...)
			// p.locals[ident] = append(p.locals[ident], val...)
		case nl:
			p.locals[ident] = append(p.locals[ident][:0], values...)
			return nil
		default:
			return p.currError()
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

func (p *Parser) nextExpect(ks ...rune) error {
	for _, k := range ks {
		p.nextToken()
		if p.currType() != k {
			return p.currError()
		}
	}
	return nil
}

func (p *Parser) peekExpect(k rune) error {
	var err error
	if !p.peekIs(k) {
		err = p.peekError()
	} else {
		p.nextToken()
	}
	return err
}

func (p *Parser) currIs(k rune) bool {
	return p.currType() == k
}

func (p *Parser) peekIs(k rune) bool {
	return p.peekType() == k
}

func (p *Parser) nextUntil(k rune) {
	for !p.currIs(k) {
		p.nextToken()
	}
}

func (p *Parser) nextToken() {
	n := len(p.frames) - 1
	if !p.frames[n].Advance() {
		p.popFrame()
	}
}

func (p *Parser) pushFrame(file string) error {
	r, err := os.Open(file)
	if err != nil {
		return err
	}
	defer r.Close()

	digest := xxh.New64(0)
	x, err := Lex(io.TeeReader(r, digest))
	if err == nil {
		sum := digest.Sum64()
		if _, ok := p.includes[sum]; ok {
			return fmt.Errorf("%s: cyclic include detected!", file)
		}
		p.includes[sum] = struct{}{}

		f := frame{
			lex:  x,
			file: file,
		}
		f.Advance()

		p.frames = append(p.frames, &f)
	}
	return err
}

func (p *Parser) popFrame() {
	if len(p.frames) <= 1 {
		return
	}
	n := len(p.frames) - 1
	if n < 0 {
		return
	}
	p.frames = p.frames[:n]
}

func (p *Parser) currLiteral() string {
	if len(p.frames) == 0 {
		return ""
	}
	n := len(p.frames) - 1
	return p.frames[n].String()
}

func (p *Parser) currType() rune {
	if len(p.frames) == 0 {
		return eof
	}
	n := len(p.frames) - 1
	return p.frames[n].currType()
}

func (p *Parser) peekType() rune {
	if len(p.frames) == 0 {
		return eof
	}
	n := len(p.frames) - 1
	return p.frames[n].peekType()
}

func (p *Parser) peekError() error {
	if len(p.frames) == 0 {
		return fmt.Errorf("no tokens available")
	}
	n := len(p.frames) - 1
	return p.frames[n].peekError()
}

func (p *Parser) currError() error {
	if len(p.frames) == 0 {
		return fmt.Errorf("no tokens available")
	}
	n := len(p.frames) - 1
	return p.frames[n].currError()
}

func (p *Parser) currToken() Token {
	var t Token
	if n := len(p.frames); n > 0 {
		t = p.frames[n-1].curr
	}
	return t
}

func (p *Parser) peekToken() Token {
	var t Token
	if n := len(p.frames); n > 0 {
		t = p.frames[n-1].peek
	}
	return t
}

type frame struct {
	file string

	lex *lexer

	curr Token
	peek Token
}

func (f *frame) Advance() bool {
	f.curr = f.peek
	f.peek = f.lex.Next()

	return f.curr.Type != eof
}

func (f *frame) String() string {
	return f.curr.Literal
}

func (f *frame) peekType() rune {
	return f.peek.Type
}

func (f *frame) currType() rune {
	return f.curr.Type
}

func (f *frame) peekError() error {
	file := f.file
	if file == "" {
		file = "<input>"
	} else {
		file = filepath.Base(file)
	}
	return fmt.Errorf("syntax error (%s): invalid token %s", file, f.peek)
}

func (f *frame) currError() error {
	file := f.file
	if file == "" {
		file = "<input>"
	} else {
		file = filepath.Base(file)
	}
	return fmt.Errorf("syntax error (%s): invalid token %s", file, f.curr)
}
