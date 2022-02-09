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
	state     stack
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
	if s.char == zero || s.char == utf8.RuneError {
		tok.Type = Eof
		return tok
	}
	if s.keepBlank && isBlank(s.char) && s.state.Default() {
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
	if s.state.Line() {
		defer s.state.Pop()
	}
	switch {
	case isHeredoc(s.char, s.peek()):
		s.scanHeredoc(&tok)
	case isComment(s.char):
		s.scanComment(&tok)
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
		s.scanLiteral(&tok)
	}
	s.toggleBlank(tok)
	return tok
}

func (s *Scanner) EnterLineMode() {
	s.state.Push(scanLine)
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

func (s *Scanner) scanModifier(tok *Token) {
	switch s.char {
	case dot:
		s.read()
		for isLower(s.char) {
			s.str.WriteRune(s.char)
			s.read()
		}
		tok.Literal = s.str.String()
		tok.Type = Macro
		s.state.Push(scanMacro)
		s.skipBlank()
		return
	case minus:
		tok.Type = Ignore
	case bang:
		tok.Type = Reverse
	case arobase:
		tok.Type = Echo
	case langle:
		tok.Type = Copy
	case tilde:
		tok.Type = Subshell
	default:
		tok.Type = Invalid
		return
	}
	s.read()
}

func (s *Scanner) scanScript(tok *Token) {
	s.skipNL()
	s.skipBlank()

	var ok bool
	switch {
	default:
		ok = !ok
	case isModifier(s.char):
		if ok = s.char == dot && !isLetter(s.peek()); ok {
			break
		}
		s.scanModifier(tok)
	case isComment(s.char):
		s.scanComment(tok)
	}
	if !ok {
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
	ident := true
	for isLiteral(s.char) {
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
	case plus:
		if s.state.Macro() {
			tok.Type = Expand
			break
		}
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
	case question:
		tok.Type = Optional
	case star:
		tok.Type = Mandatory
	case percent:
		tok.Type = Hidden
	default:
	}
	s.read()
	if (s.state.Script() || s.state.Macro()) && isNL(s.char) {
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
	if !s.state.Default() {
		return
	}
	switch tok.Type {
	case Assign, Append:
		s.keepBlank = true
		s.skipBlank()
	case Comment, Comma, BegList, EndList, Dependency, Eol:
		s.keepBlank = false
		s.skipBlank()
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
	s.skipBlank()
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

func isLiteral(b rune) bool {
	return !isVariable(b) && !isBlank(b) && !isNL(b) && !isDelimiter(b)
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
		b == lcurly || b == rcurly || b == equal || b == ampersand ||
		b == question || b == percent || b == plus || b == star
}

func isModifier(b rune) bool {
	switch b {
	case bang, minus, arobase, langle, tilde, dot, plus:
		return true
	default:
		return false
	}
}

type scanState int8

const (
	scanDefault scanState = iota
	scanScript
	scanMacro
	scanQuote
	scanValue
	scanLine
)

func (s scanState) String() string {
	switch s {
	case scanDefault:
		return "default"
	case scanScript:
		return "script"
	case scanMacro:
		return "macro"
	case scanValue:
		return "value"
	case scanQuote:
		return "quoted"
	case scanLine:
		return "line"
	default:
		return "unknown"
	}
}

type stack []scanState

func defaultStack() stack {
	var s stack
	s.Push(scanDefault)
	return s
}

func (s *stack) Pop() {
	n := s.Len()
	if n == 0 {
		return
	}
	n--
	if s.Macro() {
		n--
	}
	if n >= 0 {
		*s = (*s)[:n]
	}
}

func (s *stack) Push(st scanState) {
	*s = append(*s, st)
}

func (s *stack) Default() bool {
	return s.Curr() == scanDefault
}

func (s *stack) Value() bool {
	return s.Curr() == scanValue
}

func (s *stack) Quote() bool {
	return s.Curr() == scanQuote
}

func (s *stack) Line() bool {
	return s.Curr() == scanLine
}

func (s *stack) Script() bool {
	return s.Curr() == scanScript
}

func (s *stack) Macro() bool {
	return s.Curr() == scanMacro || (s.Script() && s.Prev() == scanMacro)
}

func (s *stack) Len() int {
	return len(*s)
}

func (s *stack) Curr() scanState {
	n := s.Len()
	if n == 0 {
		return scanDefault
	}
	n--
	return (*s)[n]
}

func (s *stack) Prev() scanState {
	n := s.Len()
	n--
	n--
	if n >= 0 {
		return (*s)[n]
	}
	return scanDefault
}
