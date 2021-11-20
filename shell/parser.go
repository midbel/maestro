package shell

import (
	"fmt"
	"io"
	"strconv"
)

type Parser struct {
	scan *Scanner
	curr Token
	peek Token

	quoted bool
}

func NewParser(r io.Reader) *Parser {
	p := Parser{
		scan: Scan(r),
	}
	p.next()
	p.next()

	return &p
}

func (p *Parser) Parse() (Executer, error) {
	if p.done() {
		return nil, io.EOF
	}
	return p.parse()
}

func (p *Parser) parse() (Executer, error) {
	if p.peek.Type == Assign {
		return p.parseAssignment()
	}
	ex, err := p.parseSimple()
	if err != nil {
		return nil, err
	}
	for !p.done() {
		switch p.curr.Type {
		case List, Comment:
			p.next()
			return ex, nil
		case And:
			return p.parseAnd(ex)
		case Or:
			return p.parseOr(ex)
		case Pipe, PipeBoth:
			ex, err = p.parsePipe(ex)
		default:
			err = p.unexpected()
		}
		if err != nil {
			return nil, err
		}
	}
	return ex, nil
}

func (p *Parser) parseSimple() (Executer, error) {
	var ex ExpandList
	for !p.done() {
		if p.curr.IsSequence() {
			break
		}
		var (
			next Expander
			err  error
		)
		switch p.curr.Type {
		case BegExp, Variable, Quote, Literal:
			next, err = p.parseWords()
		default:
			err = p.unexpected()
		}
		if err != nil {
			return nil, err
		}
		ex.List = append(ex.List, next)
	}
	e := ExecSimple{
		Expander: ex,
	}
	return e, nil
}

func (p *Parser) parseAssignment() (Executer, error) {
	if p.curr.Type != Literal {
		return nil, p.unexpected()
	}
	ex := ExecAssign{
		Ident: p.curr.Literal,
	}
	p.next()
	if p.curr.Type != Assign {
		return nil, p.unexpected()
	}
	p.next()
	var list ExpandList
	for !p.done() {
		if p.curr.IsSequence() {
			break
		}
		w, err := p.parseWords()
		if err != nil {
			return nil, err
		}
		list.List = append(list.List, w)
	}
	ex.Expander = list
	
	if p.curr.Type == List || p.curr.Type == Comment {
		p.next()
	}
	return ex, nil
}

func (p *Parser) parsePipe(left Executer) (Executer, error) {
	ex := ExecPipe{
		List: []Executer{left},
	}
	for !p.done() {
		if p.curr.Type != Pipe && p.curr.Type != PipeBoth {
			break
		}
		p.next()
		e, err := p.parseSimple()
		if err != nil {
			return nil, err
		}
		if _, ok := e.(ExecSimple); !ok {
			return nil, fmt.Errorf("single command expected")
		}
		ex.List = append(ex.List, e)
	}
	return ex, nil
}

func (p *Parser) parseAnd(left Executer) (Executer, error) {
	p.next()
	right, err := p.parse()
	if err != nil {
		return nil, err
	}
	return ExecAnd{
		Left:  left,
		Right: right,
	}, nil
}

func (p *Parser) parseOr(left Executer) (Executer, error) {
	p.next()
	right, err := p.parse()
	if err != nil {
		return nil, err
	}
	return ExecOr{
		Left:  left,
		Right: right,
	}, nil
}

func (p *Parser) parseWords() (Expander, error) {
	var list ExpandMulti
	for !p.done() {
		if p.curr.Eow() {
			if !p.curr.IsSequence() {
				p.next()
			}
			break
		}
		var (
			next Expander
			err  error
		)
		switch p.curr.Type {
		case Literal:
			next, err = p.parseLiteral()
		case Variable:
			next, err = p.parseVariable()
		case Quote:
			next, err = p.parseQuote()
		case BegExp:
			next, err = p.parseExpansion()
		default:
			err = p.unexpected()
		}
		if err != nil {
			return nil, err
		}
		list.List = append(list.List, next)
	}
	var ex Expander = list
	if len(list.List) == 1 {
		ex = list.List[0]
	}
	return ex, nil
}

func (p *Parser) parseQuote() (ExpandMulti, error) {
	p.enterQuote()

	p.next()
	var ex ExpandMulti
	for !p.done() {
		if p.curr.Type == Quote {
			break
		}
		var (
			next Expander
			err  error
		)
		switch p.curr.Type {
		case Literal:
			next, err = p.parseLiteral()
		case Variable:
			next, err = p.parseVariable()
		case BegExp:
			next, err = p.parseExpansion()
		default:
			err = p.unexpected()
		}
		if err != nil {
			return ex, err
		}
		ex.List = append(ex.List, next)
	}
	if p.curr.Type != Quote {
		return ex, p.unexpected()
	}
	p.leaveQuote()
	p.next()
	return ex, nil
}

func (p *Parser) parseLiteral() (ExpandWord, error) {
	ex := ExpandWord{
		Literal: p.curr.Literal,
	}
	p.next()
	return ex, nil
}

func (p *Parser) parseSlice(ident Token) (Expander, error) {
	e := ExpandSlice{
		Ident: ident.Literal,
	}
	p.next()
	if p.curr.Type == Literal {
		i, err := strconv.Atoi(p.curr.Literal)
		if err != nil {
			return nil, err
		}
		e.From = i
		p.next()
	}
	if p.curr.Type != Slice {
		return nil, p.unexpected()
	}
	p.next()
	if p.curr.Type == Literal {
		i, err := strconv.Atoi(p.curr.Literal)
		if err != nil {
			return nil, err
		}
		e.To = i
		p.next()
	}
	return e, nil
}

func (p *Parser) parseReplace(ident Token) (Expander, error) {
	e := ExpandReplace{
		Ident: ident.Literal,
		What:  p.curr.Type,
	}
	p.next()
	if p.curr.Type != Literal {
		return nil, p.unexpected()
	}
	e.From = p.curr.Literal
	p.next()
	if p.curr.Type != Replace {
		return nil, p.unexpected()
	}
	p.next()
	switch p.curr.Type {
	case Literal:
		e.To = p.curr.Literal
		p.next()
	case EndExp:
	default:
		return nil, p.unexpected()
	}
	return e, nil
}

func (p *Parser) parseTrim(ident Token) (Expander, error) {
	e := ExpandTrim{
		Ident: ident.Literal,
		What:  p.curr.Type,
	}
	p.next()
	if p.curr.Type != Literal {
		return nil, p.unexpected()
	}
	e.Trim = p.curr.Literal
	p.next()
	return e, nil
}

func (p *Parser) parseLower(ident Token) (Expander, error) {
	e := ExpandLower{
		Ident: ident.Literal,
		All:   p.curr.Type == LowerAll,
	}
	p.next()
	return e, nil
}

func (p *Parser) parseUpper(ident Token) (Expander, error) {
	e := ExpandUpper{
		Ident: ident.Literal,
		All:   p.curr.Type == UpperAll,
	}
	p.next()
	return e, nil
}

func (p *Parser) parseExpansion() (Expander, error) {
	p.next()
	if p.curr.Type == Length {
		p.next()
		if p.curr.Type != Literal {
			return nil, p.unexpected()
		}
		ex := ExpandLength{
			Ident: p.curr.Literal,
		}
		p.next()
		if p.curr.Type != EndExp {
			return nil, p.unexpected()
		}
		p.next()
		return ex, nil
	}
	if p.curr.Type != Literal {
		return nil, p.unexpected()
	}
	ident := p.curr
	p.next()
	var (
		ex  Expander
		err error
	)
	switch p.curr.Type {
	case EndExp:
		ex = ExpandVariable{
			Ident: ident.Literal,
		}
	case Slice:
		ex, err = p.parseSlice(ident)
	case TrimSuffix, TrimSuffixLong, TrimPrefix, TrimPrefixLong:
		ex, err = p.parseTrim(ident)
	case Replace, ReplaceAll, ReplacePrefix, ReplaceSuffix:
		ex, err = p.parseReplace(ident)
	case Lower, LowerAll:
		ex, err = p.parseLower(ident)
	case Upper, UpperAll:
		ex, err = p.parseUpper(ident)
	case ValIfUnset:
		e := ExpandValIfUnset{
			Ident: ident.Literal,
		}
		p.next()
		e.Str = p.curr.Literal
		p.next()
		ex = e
	case SetValIfUnset:
		e := ExpandSetValIfUnset{
			Ident: ident.Literal,
		}
		p.next()
		e.Str = p.curr.Literal
		p.next()
		ex = e
	case ValIfSet:
		e := ExpandValIfSet{
			Ident: ident.Literal,
		}
		p.next()
		e.Str = p.curr.Literal
		p.next()
		ex = e
	case ExitIfUnset:
		e := ExpandExitIfUnset{
			Ident: ident.Literal,
		}
		p.next()
		e.Str = p.curr.Literal
		p.next()
		ex = e
	default:
		err = p.unexpected()
	}
	if err != nil {
		return nil, err
	}
	if p.curr.Type != EndExp {
		return nil, p.unexpected()
	}
	p.next()
	return ex, nil
}

func (p *Parser) parseVariable() (ExpandVariable, error) {
	ex := ExpandVariable{
		Ident:  p.curr.Literal,
		Quoted: p.quoted,
	}
	p.next()
	return ex, nil
}

func (p *Parser) enterQuote() {
	p.quoted = true
}

func (p *Parser) leaveQuote() {
	p.quoted = false
}

func (p *Parser) next() {
	p.curr = p.peek
	p.peek = p.scan.Scan()
}

func (p *Parser) done() bool {
	return p.curr.Type == EOF
}

func (p *Parser) unexpected() error {
	return fmt.Errorf("unexpected token %s", p.curr)
}
