package maestro

import (
	"bytes"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/midbel/maestro/internal/stack"
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

type Scanner struct {
	input []byte
	curr  int
	next  int
	char  rune

	str bytes.Buffer

	line   int
	column int
	seen   int

	keepBlank bool
	state     *scanstack
}

func Scan(r io.Reader) (*Scanner, error) {
	buf, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	s := Scanner{
		input:  bytes.ReplaceAll(buf, []byte{cr, nl}, []byte{nl}),
		line:   1,
		column: 0,
		state:  defaultStack(),
	}
	s.read()
	return &s, nil
}

func (s *Scanner) Scan() Token {
	var tok Token
	tok.Position = s.currentPosition()
	if isEOF(s.char) {
		tok.Type = Eof
		return tok
	}
	if s.keepBlank && isBlank(s.char) && s.state.KeepBlank() {
		s.skipBlank()
		tok.Type = Blank
		return tok
	}
	s.reset()
	tok.Position = s.currentPosition()
	if s.char != rcurly && s.state.Script() {
		s.scanScript(&tok)
		return tok
	}
	if s.state.Quote() && !isDouble(s.char) {
		scan := s.scanText
		if isVariable(s.char) {
			scan = s.scanVariable
		}
		scan(&tok)
		return tok
	}
	s.skipBlank()
	switch {
	case isHeredoc(s.char, s.peek()):
		s.scanHeredoc(&tok)
	case isComment(s.char):
		s.scanComment(&tok)
	case isVariable(s.char):
		s.scanVariable(&tok)
	case isSingle(s.char):
		s.scanString(&tok)
	case isDouble(s.char):
		s.scanQuote(&tok)
	case s.state.Default() && isOperator(s.char):
		s.scanOperator(&tok)
	case isDelimiter(s.char):
		s.scanDelimiter(&tok)
	case isMeta(s.char):
		s.scanMeta(&tok)
	case isNL(s.char):
		s.scanEol(&tok)
	default:
		s.scanLiteral(&tok)
	}
	s.toggleBlank(tok)
	return tok
}

func (s *Scanner) CurrentLine() string {
	var (
		pos = s.curr - s.column
		off = bytes.IndexByte(s.input[s.curr:], nl)
	)
	if off < 0 {
		off = len(s.input[s.curr:])
	}
	if pos < 0 {
		pos = 0
	}
	b := s.input[pos : s.curr+off]
	for i := 0; i < len(b); i++ {
		if b[i] != nl {
			b = b[i:]
			break
		}
	}
	for i := 0; i < len(b); i++ {
		if b[i] == tab {
			b[i] = space
		}
	}
	return string(b)
}

func (s *Scanner) scanScript(tok *Token) {
	s.skipNL()
	s.skipBlank()
	if isComment(s.char) {
		s.scanComment(tok)
		return
	}
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
	s.skipBlank()
}

func (s *Scanner) scanEol(tok *Token) {
	tok.Type = Eol
	s.skipNL()
}

func (s *Scanner) scanMeta(tok *Token) {
	s.read()
	for isUpper(s.char) || s.char == underscore {
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
		tmp    bytes.Buffer
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

func (s *Scanner) scanQuote(tok *Token) {
	tok.Type = Quote
	s.state.ToggleQuote()
	s.read()
	if !s.keepBlank && !s.state.Quote() {
		s.skipBlank()
	}
}

func (s *Scanner) scanText(tok *Token) {
	accept := func(r rune) bool {
		return !isDouble(r) && !isVariable(r)
	}
	for accept(s.char) {
		s.str.WriteRune(s.char)
		s.read()
	}
	tok.Literal = s.str.String()
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

func (s *Scanner) scanVariable(tok *Token) {
	s.read()
	if s.char == lparen {
		s.read()
		for !s.done() && s.char != rparen {
			s.str.WriteRune(s.char)
			s.read()
		}
		tok.Literal = s.str.String()
		tok.Type = Script
		if s.char != rparen {
			tok.Type = Invalid
		} else {
			s.read()
		}
		return
	}
	var enclosed bool
	if s.char == lcurly {
		s.read()
		enclosed = true
	}
	for isIdent(s.char) {
		s.str.WriteRune(s.char)
		s.read()
	}
	tok.Type = Variable
	tok.Literal = s.str.String()
	if enclosed {
		if s.char != rcurly {
			tok.Type = Invalid
			return
		}
		s.read()
	}
}

func (s *Scanner) scanLiteral(tok *Token) {
	var (
		ident  = true
		accept = isValue
	)
	if s.state.Default() {
		accept = isLiteral
	}
	for accept(s.char) {
		if ident && !isIdent(s.char) {
			ident = !ident
		}
		s.str.WriteRune(s.char)
		s.read()
	}
	tok.Literal = s.str.String()
	if !ident {
		tok.Type = String
		return
	}
	switch tok.Literal {
	case kwTrue, kwFalse:
		tok.Type = Boolean
	case kwInclude, kwExport, kwDelete, kwAlias, kwNamespace:
		tok.Type = Keyword
	default:
		tok.Type = Ident
	}
}

func (s *Scanner) scanOperator(tok *Token) {
	switch s.char {
	case ampersand:
		tok.Type = Background
	case question:
		tok.Type = Optional
	case star:
		tok.Type = Mandatory
	case percent:
		tok.Type = Hidden
	default:
		tok.Type = Invalid
	}
	s.read()
}

func (s *Scanner) scanDelimiter(tok *Token) {
	switch s.char {
	case colon:
		tok.Type = Dependency
		if s.peek() == s.char {
			s.read()
			tok.Type = Resolution
		}
	case plus:
		tok.Type = Append
		s.read()
		if s.char != equal {
			tok.Type = Invalid
		}
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
		s.state.Push(scanScript)
	case rcurly:
		tok.Type = EndScript
		s.state.Pop()
	default:
		tok.Type = Invalid
	}
	s.read()
	if s.state.Script() && isNL(s.char) {
		s.skipNL()
	}
	if tok.Type == Comma {
		s.skipBlank()
	}
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

func (s *Scanner) toggleBlank(tok Token) {
	if !s.state.Default() && !s.state.Value() {
		return
	}
	switch tok.Type {
	case Assign, Append:
		s.keepBlank = true
		s.skipBlank()
		s.state.Push(scanValue)
	case Comment, Comma, BegList, EndList, Dependency, Eol:
		s.keepBlank = false
		s.skipBlank()
		s.state.Pop()
	default:
	}
}

func (s *Scanner) currentPosition() Position {
	return Position{
		Line:   s.line,
		Column: s.column,
	}
}

func (s *Scanner) reset() {
	s.str.Reset()
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

func isValue(b rune) bool {
	return !isVariable(b) && !isBlank(b) && !isNL(b) && !isDelimiter(b)
}

func isLiteral(b rune) bool {
	return isValue(b) && !isOperator(b)
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

func isSingle(b rune) bool {
	return b == squote
}

func isDouble(b rune) bool {
	return b == dquote
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

func isOperator(b rune) bool {
	return b == ampersand || b == question || b == star || b == percent
}

func isDelimiter(b rune) bool {
	return b == colon || b == comma || b == lparen || b == rparen ||
		b == lcurly || b == rcurly || b == equal || b == plus
}

type scanState int8

const (
	scanDefault scanState = iota
	scanValue
	scanScript
	scanQuote
)

type scanstack struct {
	stack.Stack[scanState]
}

func defaultStack() *scanstack {
	var s scanstack
	s.Stack = stack.New[scanState]()
	s.Stack.Push(scanDefault)
	return &s
}

func (s *scanstack) Pop() {
	s.Stack.Pop()
}

func (s *scanstack) Push(state scanState) {
	s.Stack.Push(state)
}

func (s *scanstack) KeepBlank() bool {
	curr := s.Stack.Curr()
	return curr == scanDefault || curr == scanValue
}

func (s *scanstack) Default() bool {
	return s.Stack.Curr() == scanDefault
}

func (s *scanstack) Value() bool {
	return s.Stack.Curr() == scanValue
}

func (s *scanstack) Quote() bool {
	return s.Stack.Curr() == scanQuote
}

func (s *scanstack) ToggleQuote() {
	if s.Quote() {
		s.Stack.Pop()
		return
	}
	s.Stack.Push(scanQuote)
}

func (s *scanstack) Script() bool {
	return s.Stack.Curr() == scanScript
}

func (s *scanstack) Curr() scanState {
	if s.Len() == 0 {
		return scanDefault
	}
	return s.Stack.Curr()
}

func (s *scanstack) Prev() scanState {
	n := s.Stack.Len()
	n--
	n--
	if n >= 0 {
		return s.Stack.At(n)
	}
	return scanDefault
}
