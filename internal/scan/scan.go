package scan

import (
	"bytes"
	"io"
	"strings"
	"unicode/utf8"
)

type Scanner struct {
	input []byte
	curr  int
	next  int
	char  rune

	buf   bytes.Buffer
	stack *stack

	Position
	prev Position
}

func Scan(r io.Reader) (*Scanner, error) {
	buf, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	s := Scanner{
		input: bytes.ReplaceAll(buf, []byte{cr, nl}, []byte{nl}),
		stack: emptyStack(),
	}
	s.Line = 1
	return &s, nil
}

func (s *Scanner) Scan() Token {
	defer s.reset()
	s.read()

	var tok Token
	if s.done() {
		tok.Type = Eof
		return tok
	}
	if s.stack.skipBlank() {
		s.skipBlank()
	}
	tok.Position = s.Position
	switch {
	case s.stack.isDefault():
		s.scanDefault(&tok)
	case s.stack.isQuote():
		s.scanTemplate(&tok)
	case s.stack.isScript():
		s.scanScript(&tok)
	default:
		tok.Type = Invalid
	}
	return tok
}

func (s *Scanner) scanTemplate(tok *Token) {
	switch {
	case isDouble(s.char):
		tok.Type = Quote
		s.stack.toggle()
		return
	case isVariable(s.char):
		s.scanVariable(tok)
		return
	default:
	}
	defer s.unread()
	for !isVariable(s.char) && !isDouble(s.char) && !isNL(s.char) && !s.done() {
		s.writeChar()
		s.read()
	}
	tok.Type = String
	tok.Literal = s.literal()
}

func (s *Scanner) scanDefault(tok *Token) {
	switch {
	case isComment(s.char):
		s.scanComment(tok)
	case isHeredoc(s.char, s.peek()):
		s.scanHeredoc(tok)
	case isMeta(s.char):
		s.scanMeta(tok)
	case isLetter(s.char):
		s.scanIdent(tok)
	case isVariable(s.char):
		s.scanVariable(tok)
	case isDouble(s.char):
		tok.Type = Quote
		s.stack.toggle()
	case isSingle(s.char):
		s.scanQuote(tok)
	case isOperator(s.char):
		s.scanOperator(tok)
	case isDelimiter(s.char):
		s.scanDelimiter(tok)
	case isNL(s.char):
		s.skipNL()
		tok.Type = Eol
	default:
		s.scanLiteral(tok)
	}
}

func (s *Scanner) scanScript(tok *Token) {
	switch s.char {
	case lcurly:
		tok.Type = BegScript
		s.stack.enter()
	case rcurly:
		tok.Type = EndScript
		s.stack.leave()
	case pound:
		s.scanComment(tok)
		return
	default:
	}
	if isScript(s.char) {
		s.read()
		s.skipNL()
		return
	}
	for !isNL(s.char) && !isScript(s.char) && !s.done() {
		if s.char == backslash && isNL(s.peek()) {
			s.read()
			s.read()
			s.skipBlank()
			continue
		}
		s.writeChar()
		s.read()
	}
	tok.Type = Script
	tok.Literal = strings.TrimSpace(s.literal())
	s.skipNL()
}

func (s *Scanner) scanDelimiter(tok *Token) {
	switch s.char {
	case comma:
		tok.Type = Comma
		s.read()
		if isBlank(s.char) {
			s.skipBlank()
			s.unread()
		}
	case colon:
		tok.Type = Dependency
	case lparen:
		tok.Type = BegList
	case rparen:
		tok.Type = EndList
	case lcurly:
		tok.Type = BegScript
		s.stack.enter()
	case rcurly:
		tok.Type = EndScript
		s.stack.leave()
	}
	if isScript(s.char) {
		s.read()
		s.skipNL()
	}
}

func (s *Scanner) scanOperator(tok *Token) {
	switch s.char {
	case equal:
		tok.Type = Assign
	case plus:
		s.read()
		if s.char != equal {
			tok.Type = Invalid
			break
		}
		tok.Type = Append
	case percent:
		tok.Type = Hidden
	}
}

func (s *Scanner) scanComment(tok *Token) {
	s.read()
	for !isNL(s.char) && !s.done() {
		s.writeChar()
		s.read()
	}
	tok.Type = Comment
	tok.Literal = strings.TrimSpace(s.literal())
}

func (s *Scanner) scanLiteral(tok *Token) {
	defer s.unread()
	for !isBlank(s.char) && !isNL(s.char) && !s.done() {
		s.writeChar()
		s.read()
	}
	tok.Type = String
	tok.Literal = s.literal()
}

func (s *Scanner) scanIdent(tok *Token) {
	defer s.unread()
	for isAlpha(s.char) {
		s.writeChar()
		s.read()
	}
	tok.Literal = s.literal()
	switch tok.Literal {
	case KwTrue, KwFalse:
		tok.Type = Boolean
	case KwAlias, KwInclude, KwDelete, KwExport:
		tok.Type = Keyword
	default:
		tok.Type = Ident
	}
}

func (s *Scanner) scanVariable(tok *Token) {
	defer s.unread()
	s.read()
	if !isLetter(s.char) {
		tok.Type = Invalid
	}
	for isAlpha(s.char) {
		s.writeChar()
		s.read()
	}
	if tok.Type != Invalid {
		tok.Type = Variable
	}
	tok.Literal = s.literal()
}

func (s *Scanner) scanMeta(tok *Token) {
	defer s.unread()
	s.read()
	for isUpper(s.char) || s.char == underscore {
		s.writeChar()
		s.read()
	}
	tok.Type = Meta
	tok.Literal = s.literal()
}

func (s *Scanner) scanHeredoc(tok *Token) {
	s.read()
	s.read()
	for !s.done() && isUpper(s.char) {
		s.writeChar()
		s.read()
	}
	tok.Literal = s.literal()
	s.reset()
	if !isNL(s.char) {
		tok.Type = Invalid
		return
	}
	var tmp bytes.Buffer
	s.read()
	for !s.done() {
		for !isNL(s.char) && !s.done() {
			if s.char == backslash && isNL(s.peek()) {
				s.read()
				s.read()
				s.skipBlank()
				continue
			}
			tmp.WriteRune(s.char)
			s.read()
		}
		if tmp.String() == tok.Literal {
			break
		}
		for isNL(s.char) {
			tmp.WriteRune(nl)
			s.read()
		}
		io.Copy(&s.buf, &tmp)
	}
	tok.Type = String
	tok.Literal = strings.TrimSpace(s.literal())
}

func (s *Scanner) scanQuote(tok *Token) {
	s.read()
	for !isSingle(s.char) && !isNL(s.char) && !s.done() {
		s.writeChar()
		s.read()
	}
	tok.Type = String
	tok.Literal = s.literal()
	if !isSingle(s.char) {
		tok.Type = Invalid
	}
}

func (s *Scanner) writeChar() {
	s.buf.WriteRune(s.char)
}

func (s *Scanner) literal() string {
	return s.buf.String()
}

func (s *Scanner) reset() {
	s.buf.Reset()
}

func (s *Scanner) done() bool {
	return s.char == utf8.RuneError
}

func (s *Scanner) read() {
	if s.curr >= len(s.input) || s.done() {
		s.char = utf8.RuneError
		return
	}
	s.prev = s.Position
	if s.char == nl {
		s.Line++
		s.Column = 0
	}
	s.Column++

	r, size := utf8.DecodeRune(s.input[s.next:])
	s.curr = s.next
	s.next += size
	s.char = r
}

func (s *Scanner) unread() {
	var size int
	s.Position = s.prev
	s.char, size = utf8.DecodeRune(s.input[s.curr:])
	s.next = s.curr
	s.curr -= size
}

func (s *Scanner) peek() rune {
	r, _ := utf8.DecodeRune(s.input[s.next:])
	return r
}

func (s *Scanner) skipBlank() {
	s.skip(isBlank)
}

func (s *Scanner) skipNL() {
	defer s.unread()
	s.skip(isNL)
}

func (s *Scanner) skip(fn func(rune) bool) {
	for fn(s.char) {
		s.read()
	}
}

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
	minus      = '-'
	bang       = '!'
	arobase    = '@'
	tilde      = '~'
	question   = '?'
	percent    = '%'
	plus       = '+'
	caret      = '^'
	star       = '*'
)

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
	return isLetter(b) || isDigit(b)
}

func isDigit(b rune) bool {
	return b >= '0' && b <= '9'
}

func isSingle(b rune) bool {
	return b == squote
}

func isDouble(b rune) bool {
	return b == dquote
}

func isAlpha(b rune) bool {
	return isLetter(b) || isDigit(b)
}

func isLetter(b rune) bool {
	return isLower(b) || isUpper(b) || b == underscore
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
	return b == colon || b == comma || isList(b) || isScript(b)
}

func isOperator(b rune) bool {
	return b == equal || b == plus || b == percent
}

func isHeredoc(b, p rune) bool {
	return b == langle && p == b
}

func isList(b rune) bool {
	return b == lparen || b == rparen
}

func isScript(b rune) bool {
	return b == lcurly || b == rcurly
}
