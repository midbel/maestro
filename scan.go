package maestro

import (
	"bytes"
	"io"
	"strings"
	"unicode/utf8"
)

const (
	zero       = 0
	cr         = '\r'
	nl         = '\n'
	tab        = '\t'
	space      = ' '
	squote     = '\''
	dquote     = '"'
	lcurly     = '{'
	rcurly     = '}'
	lparen     = '('
	rparen     = ')'
	dot        = '.'
	underscore = '_'
	comma      = ','
	dollar     = '$'
	equal      = '='
	colon      = ':'
	pound      = '#'
	backslash  = '\\'
	semicolon  = ';'
	ampersand  = '&'
	langle     = '<'
)

func IsValue(r rune) bool {
	return !isVariable(r) && !isBlank(r) && !isNL(r)
}

func IsLine(r rune) bool {
	return !isNL(r)
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

	script    bool
	identFunc func(rune) bool
}

func Scan(r io.Reader) (*Scanner, error) {
	buf, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	s := Scanner{
		input:     bytes.ReplaceAll(buf, []byte{cr, nl}, []byte{nl}),
		line:      1,
		column:    0,
		identFunc: isIdent,
	}
	s.read()
	return &s, nil
}

func (s *Scanner) SetIdentFunc(fn func(rune) bool) {
	s.identFunc = fn
}

func (s *Scanner) ResetIdentFunc() {
	s.identFunc = isIdent
}

func (s *Scanner) Scan() Token {
	s.skipBlank()
	s.reset()
	var tok Token
	tok.Position = Position{
		Line:   s.line,
		Column: s.column,
	}
	if s.char == zero || s.char == utf8.RuneError {
		tok.Type = Eof
		return tok
	}
	if s.char != rcurly && s.script {
		s.scanScript(&tok)
		return tok
	}
	switch {
	case isHeredoc(s.char, s.peek()):
		s.scanHeredoc(&tok)
	case isComment(s.char):
		s.scanComment(&tok)
	case isLetter(s.char):
		s.scanIdent(&tok)
	case isVariable(s.char):
		s.scanVariable(&tok)
	case isQuote(s.char):
		s.scanString(&tok)
	case isDigit(s.char):
		s.scanInteger(&tok)
	case isDelimiter(s.char):
		s.scanDelimiter(&tok)
	case isMeta(s.char):
		s.scanMeta(&tok)
	case isNL(s.char):
		s.scanEol(&tok)
	default:
		tok.Type = Invalid
	}
	return tok
}

func (s *Scanner) scanScript(tok *Token) {
	s.skipNL()
	s.skipBlank()
	for !isNL(s.char) && !s.done() {
		if s.char == backslash && isNL(s.peek()) {
			s.read()
			s.read()
			s.skipBlank()
		}
		s.str.WriteRune(s.char)
		s.read()
	}
	tok.Literal = s.str.String()
	tok.Type = Script
	s.skipNL()
}

func (s *Scanner) scanEol(tok *Token) {
	tok.Type = Eol
	s.skipNL()
}

func (s *Scanner) scanMeta(tok *Token) {
	s.read()
	for isUpper(s.char) {
		s.str.WriteRune(s.char)
		s.read()
	}
	tok.Literal = s.str.String()
	tok.Type = Meta
}

func (s *Scanner) scanHeredoc(tok *Token) {
	s.read()
	s.read()
	for !s.done() && isUpper(s.char) {
		s.str.WriteRune(s.char)
		s.read()
	}
	if !isNL(s.char) {
		tok.Type = Invalid
		return
	}
	var (
		tmp bytes.Buffer
		prefix = s.str.String()
	)
	s.str.Reset()
	s.skipNL()
	for !s.done() {
		for !isNL(s.char) {
			tmp.WriteRune(s.char)
			s.read()
		}
		if tmp.String() == prefix {
			break
		}
		for isNL(s.char) {
			tmp.WriteRune(s.char)
			s.read()
		}
		io.Copy(&s.str, &tmp)
	}
	tok.Literal = strings.TrimSpace(s.str.String())
	tok.Type = String
}

func (s *Scanner) scanString(tok *Token) {
	quote := s.char
	s.read()
	for s.char != quote && !s.done() {
		s.str.WriteRune(s.char)
		s.read()
	}
	if s.char != quote {
		tok.Type = Invalid
		return
	}
	s.read()

	tok.Literal = s.str.String()
	tok.Type = String
}

func (s *Scanner) scanInteger(tok *Token) {
	for isDigit(s.char) {
		s.str.WriteRune(s.char)
		s.read()
	}
	tok.Type = Integer
	tok.Literal = s.str.String()
}

func (s *Scanner) scanVariable(tok *Token) {
	s.read()
	for isIdent(s.char) {
		s.str.WriteRune(s.char)
		s.read()
	}
	tok.Type = Variable
	tok.Literal = s.str.String()
}

func (s *Scanner) scanIdent(tok *Token) {
	for s.identFunc(s.char) {
		s.str.WriteRune(s.char)
		s.read()
	}
	tok.Literal = s.str.String()
	switch tok.Literal {
	case kwTrue, kwFalse:
		tok.Type = Boolean
	case kwInclude, kwExport, kwDelete, kwAlias:
		tok.Type = Keyword
	default:
		tok.Type = Ident
	}
}

func (s *Scanner) scanDelimiter(tok *Token) {
	switch s.char {
	case ampersand:
		tok.Type = Background
	case colon:
		tok.Type = Dependency
	case equal:
		tok.Type = Assign
	case comma:
		tok.Type = Comma
	case lparen:
		tok.Type = BegList
	case rparen:
		tok.Type = EndList
	case lcurly:
		tok.Type = BegScript
		s.script = true
	case rcurly:
		tok.Type = EndScript
		s.script = false
	default:
	}
	s.read()
}

func (s *Scanner) scanComment(tok *Token) {
	s.read()
	s.skipBlank()
	for !isNL(s.char) {
		s.str.WriteRune(s.char)
		s.read()
	}
	s.skipNL()
	tok.Literal = s.str.String()
	tok.Type = Comment
}

func (s *Scanner) done() bool {
	return s.char == zero || s.char == utf8.RuneError
}

func (s *Scanner) reset() {
	s.str.Reset()
}

func (s *Scanner) peek() rune {
	r, _ := utf8.DecodeRune(s.input[s.next:])
	return r
}

func (s *Scanner) prev() rune {
	if s.curr == 0 {
		return zero
	}
	r, _ := utf8.DecodeLastRune(s.input[:s.curr])
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
	s.skip(isBlank)
}

func (s *Scanner) skipNL() {
	s.skip(isNL)
}

func (s *Scanner) skip(fn func(rune) bool) {
	for fn(s.char) {
		s.read()
	}
}

func isHeredoc(c, p rune) bool {
	return c == p && c == langle
}

func isNL(b rune) bool {
	return b == nl || b == cr
}

func isEOF(b rune) bool {
	return b == zero || b == utf8.RuneError
}

func isBlank(b rune) bool {
	return b == space || b == tab
}

func isIdent(b rune) bool {
	return isLetter(b) || isDigit(b) || b == underscore
}

func isDigit(b rune) bool {
	return b >= '0' && b <= '9'
}

func isQuote(b rune) bool {
	return b == dquote || b == squote
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

func isComment(b rune) bool {
	return b == pound
}

func isMeta(b rune) bool {
	return b == dot
}

func isVariable(b rune) bool {
	return b == dollar
}

func isDelimiter(b rune) bool {
	return b == colon || b == comma || b == lparen || b == rparen ||
		b == lcurly || b == rcurly || b == equal || b == ampersand
}
