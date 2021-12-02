package todos

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

var ErrSyntax = errors.New("syntax error")

type Status int8

const (
	StatusWait Status = iota
	StatusProgress
	StatusDone
	StatusIgnored
	StatusSuspended
)

func (s Status) String() string {
	switch s {
	default:
		return "unknown"
	case StatusWait:
		return "wait"
	case StatusProgress:
		return "progress"
	case StatusDone:
		return "done"
	case StatusIgnored:
		return "ignored"
	case StatusSuspended:
		return "suspended"
	}
}

type Todo struct {
	Category string // eg: todos, bugs, improvements,...
	Section  string
	Title    string
	Desc     string
	State    Status
	Tags     []string
	Props    map[string][]string
}

type Parser struct {
	scan *Scanner
	curr Token
	peek Token

	category string
}

func Parse(r io.Reader) ([]Todo, error) {
	pars := Parser{
		scan: Scan(r),
	}
	pars.next()
	pars.next()
	return pars.parse()
}

func (p *Parser) parse() ([]Todo, error) {
	var list []Todo
	for !p.done() {
		var err error
		switch p.curr.Type {
		case Category:
			err = p.parseCategory()
		case Section:
			t, err1 := p.parseTodo()
			if err1 != nil {
				err = err1
				break
			}
			list = append(list, t)
		case Comment, EOL:
			p.next()
		default:
			err = ErrSyntax
		}
		if err != nil {
			return nil, err
		}
	}
	return list, nil
}

func (p *Parser) parseTodo() (Todo, error) {
	p.next()
	var (
		todo Todo
		err  error
	)
	if p.curr.IsStatus() {
		todo.State = p.curr.Status()
		p.next()
	}
	if !p.curr.IsLiteral() {
		return todo, ErrSyntax
	}
	todo.Section = p.curr.Literal
	todo.Category = p.category
	p.next()
	if todo.Tags, err = p.parseTags(); err != nil {
		return todo, err
	}
	if p.curr.Type != Sep {
		return todo, ErrSyntax
	}
	p.next()
	switch p.curr.Type {
	case Literal:
		todo.Title = p.curr.Literal
	case EOL:
	default:
		return todo, ErrSyntax
	}
	p.next()
	if todo.Desc, err = p.parseDescription(); err != nil {
		return todo, err
	}
	if todo.Props, err = p.parseProperties(); err != nil {
		return todo, err
	}
	return todo, err
}

func (p *Parser) parseProperties() (map[string][]string, error) {
	if p.curr.Type != Property {
		return nil, nil
	}
	list := make(map[string][]string)
	for !p.done() && p.curr.Type == Property {
		p.next()
		if !p.curr.IsLiteral() || p.peek.Type != Sep {
			return nil, ErrSyntax
		}
		ident := p.curr
		p.next()
		p.next()
		for p.curr.IsLiteral() {
			list[ident.Literal] = append(list[ident.Literal], p.curr.Literal)
			p.next()
		}
		if p.curr.Type != EOL {
			return nil, ErrSyntax
		}
		p.next()
	}
	return list, nil
}

func (p *Parser) parseDescription() (string, error) {
	stop := func(t rune) bool {
		return t == Property || t == Section || t == Category
	}
  for p.curr.Type == EOL {
    p.next()
  }
	var list []string
	for !p.done() && !stop(p.curr.Type) {
		switch p.curr.Type {
		case Literal:
		case EOL:
		default:
			return "", ErrSyntax
		}
		list = append(list, p.curr.Literal)
		p.next()
	}
	return strings.Join(list, "\n"), nil
}

func (p *Parser) parseTags() ([]string, error) {
	if p.curr.Type != BegList {
		return nil, nil
	}
	p.next()
	var list []string
	for !p.done() && p.curr.Type != EndList {
		if !p.curr.IsLiteral() {
			return nil, ErrSyntax
		}
		list = append(list, p.curr.Literal)
		p.next()
		switch p.curr.Type {
		case Comma:
			p.next()
		case EndList:
		default:
			return nil, ErrSyntax
		}
	}
	if p.curr.Type != EndList {
		return nil, ErrSyntax
	}
	p.next()
	return list, nil
}

func (p *Parser) parseCategory() error {
	p.next()
	if !p.curr.IsLiteral() {
		return ErrSyntax
	}
	p.category = p.curr.Literal
	p.next()
  if p.curr.Type != EOL {
    return ErrSyntax
  }
	return nil
}

func (p *Parser) done() bool {
	return p.curr.Type == EOF
}

func (p *Parser) next() {
	p.curr = p.peek
	p.peek = p.scan.Scan()
}

const (
	zero      = 0
	space     = ' '
	tab       = '\t'
	nl        = '\n'
	cr        = '\r'
	langle    = '<'
	rangle    = '>'
	colon     = ':'
	dash      = '-'
	bang      = '!'
	question  = '?'
	comma     = ','
	star      = '*'
	slash     = '/'
	pound     = '#'
	backslash = '\\'
	lparen    = '('
	rparen    = ')'
)

const (
	EOF = -(iota + 1)
	EOL
	Literal
	BegList
	EndList
	Comma
	Category // #
	Property // - prop: value
	Section  // *
	Sep      // :
	Comment  // //, /**/
	Done
	Progress
	Ignored
	Suspended
	Invalid
)

type Token struct {
	Literal string
	Type    rune
}

func (t Token) IsLiteral() bool {
	return t.Type == Literal
}

func (t Token) IsStatus() bool {
	switch t.Type {
	case Done, Progress, Ignored, Suspended:
		return true
	default:
		return false
	}
}

func (t Token) Status() Status {
	switch t.Type {
	case Done:
		return StatusDone
	case Progress:
		return StatusProgress
	case Ignored:
		return StatusIgnored
	case Suspended:
		return StatusSuspended
	default:
		return StatusWait
	}
}

func (t Token) String() string {
	switch t.Type {
	default:
		return "<unknown>"
	case EOF:
		return "<eof>"
	case EOL:
		return "<eol>"
	case BegList:
		return "<beg>"
	case EndList:
		return "<end>"
	case Comma:
		return "<comma>"
	case Category:
		return "<category>"
	case Property:
		return "<property>"
	case Section:
		return "<section>"
	case Sep:
		return "<sep>"
	case Done:
		return "<done>"
	case Progress:
		return "<progress>"
	case Ignored:
		return "<ignored>"
	case Suspended:
		return "<suspended>"
	case Invalid:
		return "<invalid>"
	case Comment:
		return fmt.Sprintf("comment(%s)", t.Literal)
	case Literal:
		return fmt.Sprintf("literal(%s)", t.Literal)
	}
}

type Scanner struct {
	input []byte
	curr  int
	next  int
	char  rune

	str bytes.Buffer

	line   int
	column int
	seen   int

  scanlines bool
}

func Scan(r io.Reader) *Scanner {
	buf, _ := io.ReadAll(r)
	s := Scanner{
		input: bytes.ReplaceAll(buf, []byte{cr, nl}, []byte{nl}),
		line:  1,
	}
	s.read()
	return &s
}

func (s *Scanner) Scan() Token {
	s.reset()
	var tok Token
	if s.char == zero || s.char == utf8.RuneError {
		tok.Type = EOF
		return tok
	}
	s.skipBlank()
	switch {
	case isComment(s.char, s.peek()):
		s.scanComment(&tok)
	case isStatus(s.char):
		s.scanStatus(&tok)
	case isDelimiter(s.char):
		s.scanDelimiter(&tok)
	case isProperty(s.char):
		s.scanProperty(&tok)
	case isNL(s.char):
		tok.Type = EOL
		s.skipNL()
	default:
		s.scanLiteral(&tok)
	}
	return tok
}

func (s *Scanner) scanLiteral(tok *Token) {
  ok := isNL
  if !s.scanlines {
    ok = func(r rune) bool {
      return isDelimiter(r) || isNL(r)
    }
  }
	for !ok(s.char) {
		if s.char == backslash && s.peek() == nl {
			s.read()
			s.read()
			continue
		}
		s.write()
		s.read()
	}
	tok.Type = Literal
	tok.Literal = s.string()
}

func (s *Scanner) scanStatus(tok *Token) {
	switch s.char {
	case question:
		tok.Type = Suspended
	case bang:
		tok.Type = Ignored
	case langle:
		tok.Type = Done
	case rangle:
		tok.Type = Progress
	default:
		tok.Type = Invalid
	}
	s.read()
}

func (s *Scanner) scanProperty(tok *Token) {
	tok.Type = Property
  s.toggleLines()
	s.read()
	s.skipBlank()
}

func (s *Scanner) scanDelimiter(tok *Token) {
	switch s.char {
	case pound:
		tok.Type = Category
    s.resetLines()
	case star:
		tok.Type = Section
    s.resetLines()
	case lparen:
		tok.Type = BegList
	case rparen:
		tok.Type = EndList
	case comma:
		tok.Type = Comma
	case colon:
		tok.Type = Sep
    s.toggleLines()
	default:
		tok.Type = Invalid
	}
	s.read()
	s.skipBlank()
}

func (s *Scanner) scanComment(tok *Token) {
	s.read()
	s.read()
	s.skipBlank()
	for !isNL(s.char) {
		s.write()
		s.read()
	}
	s.skipNL()
	tok.Type = Comment
	tok.Literal = s.string()
}

func (s *Scanner) resetLines() {
  s.scanlines = false
}

func (s *Scanner) toggleLines() {
  s.scanlines = !s.scanlines
}

func (s *Scanner) peek() rune {
	r, _ := utf8.DecodeRune(s.input[s.next:])
	return r
}

func (s *Scanner) read() {
	if s.curr >= len(s.input) {
		s.char = 0
		return
	}
	r, n := utf8.DecodeRune(s.input[s.next:])
	if r == utf8.RuneError {
		s.char = 0
		s.next = len(s.input)
	}
	last := s.char
	s.char, s.curr, s.next = r, s.next, s.next+n

	if last == nl {
		s.line++
		s.seen, s.column = s.column, 1
	} else {
		s.column++
	}
}

func (s *Scanner) skipBlank() {
	s.skipWith(isBlank)
}

func (s *Scanner) skipNL() {
	s.skipWith(isNL)
}

func (s *Scanner) skipWith(fn func(rune) bool) {
	for fn(s.char) {
		s.read()
	}
}

func (s *Scanner) string() string {
	return s.str.String()
}

func (s *Scanner) write() {
	s.str.WriteRune(s.char)
}

func (s *Scanner) reset() {
	s.str.Reset()
}

func isComment(c, p rune) bool {
	return (c == slash && p == slash) || (c == slash && p == star)
}

func isProperty(c rune) bool {
	return c == dash
}

func isDelimiter(c rune) bool {
	switch c {
	case lparen, rparen, comma, colon, star, pound:
		return true
	default:
		return false
	}
}

func isStatus(c rune) bool {
	switch c {
	case question, bang, langle, rangle:
		return true
	default:
		return false
	}
}

func isEOF(b rune) bool {
	return b == zero || b == utf8.RuneError
}

func isNL(b rune) bool {
	return b == nl || b == cr
}

func isBlank(b rune) bool {
	return b == space || b == tab
}

func isLetter(b rune) bool {
	return isLower(b) || isUpper(b)
}

func isLower(b rune) bool {
	return b >= 'a' && b <= 'z'
}

func isUpper(b rune) bool {
	return b >= 'A' && b <= 'Z'
}
