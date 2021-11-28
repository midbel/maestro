package shell

import (
	"fmt"
	"io"
	"strconv"
	"strings"
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
	ex, err := p.parse()
	if err != nil {
		return nil, err
	}
	switch p.curr.Type {
	case List, Comment, EOF:
		p.next()
	default:
		return nil, p.unexpected()
	}
	return ex, nil
}

func (p *Parser) parse() (Executer, error) {
	if p.peek.Type == Assign {
		return p.parseAssignment()
	}
	ex, err := p.parseSimple()
	if err != nil {
		return nil, err
	}
	for {
		switch p.curr.Type {
		case And:
			return p.parseAnd(ex)
		case Or:
			return p.parseOr(ex)
		case Pipe, PipeBoth:
			ex, err = p.parsePipe(ex)
			if err != nil {
				return nil, err
			}
		default:
			return ex, nil
		}
	}
}

func (p *Parser) parseSimple() (Executer, error) {
	var (
		ex   ExpandList
		dirs []ExpandRedirect
	)
	for {
		switch p.curr.Type {
		case Literal, Quote, Variable, BegExp, BegBrace, BegSub:
			next, err := p.parseWords()
			if err != nil {
				return nil, err
			}
			ex.List = append(ex.List, next)
		case RedirectIn, RedirectOut, RedirectErr, RedirectBoth, AppendOut, AppendBoth:
			next, err := p.parseRedirection()
			if err != nil {
				return nil, err
			}
			dirs = append(dirs, next)
		default:
			sg := createSimple(ex)
			sg.Redirect = append(sg.Redirect, dirs...)
			return sg, nil
		}
	}
}

func (p *Parser) parseRedirection() (ExpandRedirect, error) {
	kind := p.curr.Type
	p.next()
	e, err := p.parseWords()
	if err != nil {
		return ExpandRedirect{}, err
	}
	return createRedirect(e, kind), nil
}

func (p *Parser) parseAssignment() (Executer, error) {
	if p.curr.Type != Literal {
		return nil, p.unexpected()
	}
	var (
		ident = p.curr.Literal
		list  ExpandList
	)
	p.next()
	if p.curr.Type != Assign {
		return nil, p.unexpected()
	}
	p.next()
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
	return createAssign(ident, list), nil
}

func (p *Parser) parsePipe(left Executer) (Executer, error) {
	var list []pipeitem
	list = append(list, createPipeItem(left, p.curr.Type == PipeBoth))
	for !p.done() {
		if p.curr.Type != Pipe && p.curr.Type != PipeBoth {
			break
		}
		var (
			both = p.curr.Type == PipeBoth
			ex   Executer
			err  error
		)
		p.next()
		if ex, err = p.parseSimple(); err != nil {
			return nil, err
		}
		list = append(list, createPipeItem(ex, both))
	}
	return createPipe(list), nil
}

func (p *Parser) parseAnd(left Executer) (Executer, error) {
	p.next()
	right, err := p.parse()
	if err != nil {
		return nil, err
	}
	return createAnd(left, right), nil
}

func (p *Parser) parseOr(left Executer) (Executer, error) {
	p.next()
	right, err := p.parse()
	if err != nil {
		return nil, err
	}
	return createOr(left, right), nil
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
		case BegSub:
			next, err = p.parseSubstitution()
		case BegBrace:
			next, err = p.parseBraces(list.Pop())
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

func (p *Parser) parseSubstitution() (Expander, error) {
	var ex ExpandSub
	ex.Quoted = p.quoted
	p.next()
	for !p.done() {
		if p.curr.Type == EndSub {
			break
		}
		next, err := p.parse()
		if err != nil {
			return nil, err
		}
		ex.List = append(ex.List, next)
	}
	if p.curr.Type != EndSub {
		return nil, p.unexpected()
	}
	p.next()
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
		case BegSub:
			next, err = p.parseSubstitution()
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
	ex := createWord(p.curr.Literal, p.quoted)
	p.next()
	return ex, nil
}

func (p *Parser) parseBraces(prefix Expander) (Expander, error) {
	p.next()
	if p.peek.Type == Range {
		return p.parseRangeBraces(prefix)
	}
	return p.parseListBraces(prefix)
}

func (p *Parser) parseWordsInBraces() (Expander, error) {
	var list ExpandList
	for !p.done() {
		if p.curr.Type == Seq || p.curr.Type == EndBrace {
			break
		}
		var (
			next Expander
			err  error
		)
		switch p.curr.Type {
		case Literal:
			next, err = p.parseLiteral()
		case BegBrace:
			next, err = p.parseBraces(list.Pop())
		default:
			err = p.unexpected()
		}
		if err != nil {
			return nil, err
		}
		list.List = append(list.List, next)
	}
	return list, nil
}

func (p *Parser) parseListBraces(prefix Expander) (Expander, error) {
	ex := ExpandListBrace{
		Prefix: prefix,
	}
	for !p.done() {
		if p.curr.Type == EndBrace {
			break
		}
		x, err := p.parseWordsInBraces()
		if err != nil {
			return nil, err
		}
		ex.Words = append(ex.Words, x)
		switch p.curr.Type {
		case Seq:
			p.next()
		case EndBrace:
		default:
			return nil, p.unexpected()
		}
	}
	if p.curr.Type != EndBrace {
		return nil, p.unexpected()
	}
	p.next()
	suffix, err := p.parseWordsInBraces()
	if err != nil {
		return nil, err
	}
	ex.Suffix = suffix
	return ex, nil
}

func (p *Parser) parseRangeBraces(prefix Expander) (Expander, error) {
	parseInt := func() (int, error) {
		if p.curr.Type != Literal {
			return 0, p.unexpected()
		}
		i, err := strconv.Atoi(p.curr.Literal)
		if err == nil {
			p.next()
		}
		return i, err
	}
	ex := ExpandRangeBrace{
		Prefix: prefix,
		Step:   1,
	}
	if p.curr.Type == Literal {
		if n := len(p.curr.Literal); strings.HasPrefix(p.curr.Literal, "0") && n > 1 {
			str := strings.TrimLeft(p.curr.Literal, "0")
			ex.Pad = (n - len(str)) + 1
		}
	}
	var err error
	if ex.From, err = parseInt(); err != nil {
		return nil, err
	}
	if p.curr.Type != Range {
		return nil, p.unexpected()
	}
	p.next()
	if ex.To, err = parseInt(); err != nil {
		return nil, err
	}
	if p.curr.Type == Range {
		p.next()
		if ex.Step, err = parseInt(); err != nil {
			return nil, err
		}
	}
	if p.curr.Type != EndBrace {
		return nil, p.unexpected()
	}
	p.next()
	return ex, nil
}

func (p *Parser) parseSlice(ident Token) (Expander, error) {
	e := ExpandSlice{
		Ident:  ident.Literal,
		Quoted: p.quoted,
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

func (p *Parser) parsePadding(ident Token) (Expander, error) {
	e := ExpandPad{
		Ident: ident.Literal,
		What:  p.curr.Type,
		With:  " ",
	}
	p.next()
	switch p.curr.Type {
	case Literal:
		e.With = p.curr.Literal
		p.next()
	case Slice:
	default:
		return nil, p.unexpected()
	}
	if p.curr.Type != Slice {
		return nil, p.unexpected()
	}
	p.next()

	size, err := strconv.Atoi(p.curr.Literal)
	if err != nil {
		return nil, err
	}
	e.Len = size
	p.next()
	return e, nil
}

func (p *Parser) parseReplace(ident Token) (Expander, error) {
	e := ExpandReplace{
		Ident:  ident.Literal,
		What:   p.curr.Type,
		Quoted: p.quoted,
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
		Ident:  ident.Literal,
		What:   p.curr.Type,
		Quoted: p.quoted,
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
		Ident:  ident.Literal,
		All:    p.curr.Type == LowerAll,
		Quoted: p.quoted,
	}
	p.next()
	return e, nil
}

func (p *Parser) parseUpper(ident Token) (Expander, error) {
	e := ExpandUpper{
		Ident:  ident.Literal,
		All:    p.curr.Type == UpperAll,
		Quoted: p.quoted,
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
		ex = createVariable(ident.Literal, p.quoted)
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
	case PadLeft, PadRight:
		ex, err = p.parsePadding(ident)
	case ValIfUnset:
		p.next()
		ex = createValIfUnset(ident.Literal, p.curr.Literal, p.quoted)
		p.next()
	case SetValIfUnset:
		p.next()
		ex = createSetValIfUnset(ident.Literal, p.curr.Literal, p.quoted)
		p.next()
	case ValIfSet:
		p.next()
		ex = createExpandValIfSet(ident.Literal, p.curr.Literal, p.quoted)
		p.next()
	case ExitIfUnset:
		p.next()
		ex = createExpandExitIfUnset(ident.Literal, p.curr.Literal, p.quoted)
		p.next()
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

func (p *Parser) parseVariable() (ExpandVar, error) {
	ex := createVariable(p.curr.Literal, p.quoted)
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
