package main

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

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
