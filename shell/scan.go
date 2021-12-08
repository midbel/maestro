package shell

import (
	"bytes"
	"io"
	"unicode/utf8"
)

const (
	zero       = 0
	space      = ' '
	tab        = '\t'
	squote     = '\''
	dquote     = '"'
	dollar     = '$'
	pound      = '#'
	percent    = '%'
	slash      = '/'
	comma      = ','
	colon      = ':'
	minus      = '-'
	plus       = '+'
	question   = '?'
	underscore = '_'
	lcurly     = '{'
	rcurly     = '}'
	lparen     = '('
	rparen     = ')'
	equal      = '='
	caret      = '^'
	ampersand  = '&'
	pipe       = '|'
	semicolon  = ';'
	langle     = '<'
	rangle     = '>'
	backslash  = '\\'
	dot        = '.'
	star       = '*'
	arobase    = '@'
	bang       = '!'
	nl         = '\n'
	cr         = '\r'
)

const (
	kwFor      = "for"
	kwDo       = "do"
	kwDone     = "done"
	kwIn       = "in"
	kwWhile    = "while"
	kwUntil    = "until"
	kwIf       = "if"
	kwFi       = "fi"
	kwThen     = "then"
	kwElse     = "else"
	kwCase     = "case"
	kwEsac     = "esac"
	kwBreak    = "break"
	kwContinue = "continue"
)

var colonOps = map[rune]rune{
	minus:    ValIfUnset,
	plus:     ValIfSet,
	equal:    SetValIfUnset,
	question: ExitIfUnset,
	langle:   PadLeft,
	rangle:   PadRight,
}

var slashOps = map[rune]rune{
	slash:   ReplaceAll,
	percent: ReplaceSuffix,
	pound:   ReplacePrefix,
}

type Scanner struct {
	input []byte
	char  rune
	curr  int
	next  int

	str   bytes.Buffer
	state stack
}

func Scan(r io.Reader) *Scanner {
	buf, _ := io.ReadAll(r)
	s := Scanner{
		input: buf,
		state: defaultStack(),
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
	if s.state.Arithmetic() {
		s.scanArithmetic(&tok)
		return tok
	}
	switch {
	case isBraces(s.char) && s.state.AcceptBraces():
		s.scanBraces(&tok)
	case isList(s.char) && s.state.Braces():
		s.scanList(&tok)
	case isOperator(s.char) && s.state.Expansion():
		s.scanOperator(&tok)
	case isBlank(s.char) && !s.state.Quoted():
		tok.Type = Blank
		s.skipBlank()
	case isSequence(s.char) && !s.state.Quoted():
		s.scanSequence(&tok)
	case isRedirectBis(s.char, s.peek()) && !s.state.Quoted():
		s.scanRedirect(&tok)
	case isAssign(s.char) && !s.state.Quoted():
		s.scanAssignment(&tok)
	case isDouble(s.char):
		s.scanQuote(&tok)
	case isSingle(s.char):
		s.scanString(&tok)
	case isComment(s.char):
		s.scanComment(&tok)
	case isVariable(s.char):
		s.scanVariable(&tok)
	default:
		s.scanLiteral(&tok)
	}
	return tok
}

func (s *Scanner) scanArithmetic(tok *Token) {
	s.skipBlank()
	switch {
	case isMath(s.char):
		s.scanMath(tok)
	case isDigit(s.char):
		s.scanDigit(tok)
	case isLetter(s.char):
		s.scanLiteral(tok)
		tok.Type = Variable
	default:
		tok.Type = Invalid
	}
}

func (s *Scanner) scanDigit(tok *Token) {
	for isDigit(s.char) {
		s.write()
		s.read()
	}
	if s.char == dot {
		s.write()
		s.read()
		for isDigit(s.char) {
			s.write()
			s.read()
		}
	}
	tok.Literal = s.string()
	tok.Type = Number
}

func (s *Scanner) scanMath(tok *Token) {
	switch s.char {
	case semicolon:
		tok.Type = List
	case bang:
		tok.Type = Not
	case plus:
		tok.Type = Add
		if s.peek() == s.char {
			tok.Type = Inc
			s.read()
		}
	case minus:
		tok.Type = Sub
		if s.peek() == s.char {
			tok.Type = Dec
			s.read()
		}
	case star:
		tok.Type = Mul
		if s.peek() == s.char {
			tok.Type = Pow
			s.read()
		}
	case slash:
		tok.Type = Div
	case percent:
		tok.Type = Mod
	case lparen:
		tok.Type = BegMath
		s.state.EnterArithmetic()
	case rparen:
		tok.Type = EndMath
		s.state.LeaveArithmetic()
		if s.state.Depth() == 0 && s.peek() == s.char {
			s.read()
		}
	case pipe:
		tok.Type = BitOr
		if s.peek() == s.char {
			tok.Type = Or
			s.read()
		}
	case ampersand:
		tok.Type = BitAnd
		if s.peek() == s.char {
			tok.Type = And
			s.read()
		}
	case langle:
		if s.peek() == s.char {
			s.read()
			tok.Type = LeftShift
		}
	case rangle:
		if s.peek() == s.char {
			s.read()
			tok.Type = RightShift
		}
	default:
		tok.Type = Invalid
	}
	s.read()
}

func (s *Scanner) scanQuote(tok *Token) {
	tok.Type = Quote
	s.read()
	s.state.ToggleQuote()
	if s.state.Quoted() {
		return
	}
	s.skipBlankUntil(func(r rune) bool {
		return isSequence(r) || isAssign(r) || isComment(r) || isRedirectBis(r, s.peek())
	})
}

func (s *Scanner) scanBraces(tok *Token) {
	switch k := s.peek(); {
	case s.char == rcurly:
		tok.Type = EndBrace
		s.state.LeaveBrace()
	case s.char == lcurly && k != rcurly:
		tok.Type = BegBrace
		s.state.EnterBrace()
	default:
		s.scanLiteral(tok)
		return
	}
	s.read()
	s.skipBlank()
}

func (s *Scanner) scanList(tok *Token) {
	switch k := s.peek(); {
	case s.char == comma:
		tok.Type = Seq
	case s.char == dot && k == s.char:
		tok.Type = Range
		s.read()
	default:
	}
	if tok.Type == Invalid {
		return
	}
	s.read()
	s.skipBlank()
}

func (s *Scanner) scanAssignment(tok *Token) {
	tok.Type = Assign
	s.read()
	s.skipBlank()
}

func (s *Scanner) scanRedirect(tok *Token) {
	switch s.char {
	case langle:
		tok.Type = RedirectIn
	case rangle:
		tok.Type = RedirectOut
		if k := s.peek(); k == s.char {
			tok.Type = AppendOut
			s.read()
		}
	case ampersand:
		s.read()
		if s.char == rangle && s.peek() == s.char {
			s.read()
			tok.Type = AppendBoth
		} else if s.char == rangle {
			tok.Type = RedirectBoth
		} else {
			tok.Type = Invalid
		}
	case '0':
		s.read()
		if s.char != langle {
			tok.Type = Invalid
			break
		}
		tok.Type = RedirectIn
	case '1':
		s.read()
		if s.char == rangle && s.peek() == s.char {
			s.read()
			tok.Type = AppendOut
		} else if s.char == rangle {
			tok.Type = RedirectOut
		} else {
			tok.Type = Invalid
		}
	case '2':
		s.read()
		if s.char == rangle && s.peek() == s.char {
			s.read()
			tok.Type = AppendErr
		} else if s.char == rangle {
			tok.Type = RedirectErr
		} else {
			tok.Type = Invalid
		}
	default:
		tok.Type = Invalid
	}
	s.read()
	s.skipBlank()
}

func (s *Scanner) scanSequence(tok *Token) {
	switch k := s.peek(); {
	case s.char == semicolon:
		tok.Type = List
	case s.char == ampersand && k == s.char:
		tok.Type = And
		s.read()
	case s.char == ampersand && isRedirect(k):
		s.scanRedirect(tok)
		return
	case s.char == pipe && k == s.char:
		tok.Type = Or
		s.read()
	case s.char == pipe && k == ampersand:
		tok.Type = PipeBoth
		s.read()
	case s.char == pipe:
		tok.Type = Pipe
	case s.char == rparen:
		tok.Type = EndSub
		s.state.LeaveSubstitution()
	default:
		tok.Type = Invalid
	}
	s.read()
	s.skipBlank()
}

func (s *Scanner) scanOperator(tok *Token) {
	if k := s.prev(); s.char == pound && k == lcurly {
		tok.Type = Length
		s.read()
		return
	}
	switch s.char {
	case rcurly:
		tok.Type = EndExp
		s.state.LeaveExpansion()
	case colon:
		tok.Type = Slice
		if t, ok := colonOps[s.peek()]; ok {
			s.read()
			tok.Type = t
		}
	case slash:
		tok.Type = Replace
		if t, ok := slashOps[s.peek()]; ok {
			s.read()
			tok.Type = t
		}
	case percent:
		tok.Type = TrimSuffix
		if k := s.peek(); k == percent {
			tok.Type = TrimSuffixLong
			s.read()
		}
	case pound:
		tok.Type = TrimPrefix
		if k := s.peek(); k == pound {
			tok.Type = TrimPrefixLong
			s.read()
		}
	case comma:
		tok.Type = Lower
		if k := s.peek(); k == comma {
			tok.Type = LowerAll
			s.read()
		}
	case caret:
		tok.Type = Upper
		if k := s.peek(); k == caret {
			tok.Type = UpperAll
			s.read()
		}
	default:
		tok.Type = Invalid
	}
	s.read()
}

func (s *Scanner) scanVariable(tok *Token) {
	s.read()
	if s.char == lcurly {
		tok.Type = BegExp
		s.state.EnterExpansion()
		s.read()
		return
	}
	if s.char == lparen && s.peek() == lparen {
		s.read()
		s.read()
		tok.Type = BegMath
		s.state.EnterArithmetic()
		return
	}
	if s.char == lparen {
		tok.Type = BegSub
		s.state.EnterSubstitution()
		s.read()
		return
	}
	tok.Type = Variable
	switch {
	case s.char == dollar:
		tok.Literal = "$"
		s.read()
	case s.char == pound:
		tok.Literal = "#"
		s.read()
	case s.char == question:
		tok.Literal = "?"
		s.read()
	case s.char == star:
		tok.Literal = "*"
		s.read()
	case s.char == arobase:
		tok.Literal = "@"
		s.read()
	case s.char == bang:
		tok.Literal = "!"
		s.read()
	case isDigit(s.char):
		for isDigit(s.char) {
			s.write()
			s.read()
		}
		tok.Literal = s.string()
	default:
		if !isLetter(s.char) {
			tok.Type = Invalid
			return
		}
		for isIdent(s.char) {
			s.write()
			s.read()
		}
		tok.Literal = s.string()
	}
}

func (s *Scanner) scanComment(tok *Token) {
	s.read()
	s.skipBlank()
	for !s.done() {
		s.write()
		s.read()
	}
	tok.Type = Comment
	tok.Literal = s.string()
}

func (s *Scanner) scanString(tok *Token) {
	s.read()
	for !isSingle(s.char) && !s.done() {
		s.write()
		s.read()
	}
	tok.Type = Literal
	tok.Literal = s.string()
	if !isSingle(s.char) {
		tok.Type = Invalid
	}
	s.read()
	s.skipBlankUntil(func(r rune) bool {
		return isSequence(r) || isAssign(r) || isComment(r) || isRedirectBis(r, s.peek())
	})
}

func (s *Scanner) scanLiteral(tok *Token) {
	if s.state.Quoted() {
		s.scanQuotedLiteral(tok)
		return
	}
	for !s.done() && !s.stopLiteral(s.char) {
		if s.char == backslash && canEscape(s.peek()) {
			s.read()
		}
		s.write()
		s.read()
	}
	tok.Type = Literal
	tok.Literal = s.string()
	switch tok.Literal {
	case kwFor, kwWhile, kwUntil, kwIf, kwCase, kwDo, kwDone, kwFi, kwThen, kwIn, kwElse, kwEsac:
		tok.Type = Keyword
		s.skipBlank()
	default:
	}
	s.skipBlankUntil(func(r rune) bool {
		return isSequence(r) || isAssign(r) || isComment(r) || isRedirectBis(r, s.peek())
	})
}

func (s *Scanner) scanQuotedLiteral(tok *Token) {
	for !s.done() {
		if isDouble(s.char) || isVariable(s.char) {
			break
		}
		if s.state.Expansion() && isOperator(s.char) {
			break
		}
		s.write()
		s.read()
	}
	tok.Type = Literal
	tok.Literal = s.string()
}

func (s *Scanner) reset() {
	s.str.Reset()
}

func (s *Scanner) write() {
	s.str.WriteRune(s.char)
}

func (s *Scanner) string() string {
	return s.str.String()
}

func (s *Scanner) peek() rune {
	r, _ := utf8.DecodeRune(s.input[s.next:])
	return r
}

func (s *Scanner) prev() rune {
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
	s.char, s.curr, s.next = r, s.next, s.next+n
}

func (s *Scanner) done() bool {
	return s.char == zero || s.char == utf8.RuneError
}

func (s *Scanner) skipBlank() {
	for isBlank(s.char) {
		s.read()
	}
}

func (s *Scanner) skipBlankUntil(fn func(rune) bool) {
	if !isBlank(s.char) {
		return
	}
	var (
		curr = s.curr
		next = s.next
		char = s.char
	)
	s.skipBlank()
	if !fn(s.char) {
		s.curr = curr
		s.next = next
		s.char = char
	}
}

func (s *Scanner) stopLiteral(r rune) bool {
	if s.state.Braces() && (s.char == dot || s.char == comma || s.char == rcurly) {
		return true
	}
	if s.state.Expansion() && isOperator(r) {
		return true
	}
	if s.char == lcurly {
		return s.peek() != rcurly
	}
	ok := isBlank(s.char) || isSequence(s.char) || isDouble(s.char) ||
		isVariable(s.char) || isAssign(s.char)
	return ok
}

func canEscape(r rune) bool {
	return r == backslash || r == semicolon || r == dquote || r == dollar
}

func isBlank(r rune) bool {
	return r == space || r == tab
}

func isDouble(r rune) bool {
	return r == dquote
}

func isSingle(r rune) bool {
	return r == squote
}

func isQuote(r rune) bool {
	return isDouble(r) || isSingle(r)
}

func isVariable(r rune) bool {
	return r == dollar
}

func isComment(r rune) bool {
	return r == pound
}

func isIdent(r rune) bool {
	return isLetter(r) || isDigit(r) || r == underscore
}

func isLetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

func isDigit(r rune) bool {
	return r >= '0' && r <= '9'
}

func isOperator(r rune) bool {
	return r == caret || r == pound || r == colon || r == slash || r == percent || r == comma || r == rcurly
}

func isSequence(r rune) bool {
	return r == ampersand || r == pipe || r == semicolon || r == rparen
}

func isAssign(r rune) bool {
	return r == equal
}

func isRedirect(r rune) bool {
	return r == langle || r == rangle
}

func isRedirectBis(r, k rune) bool {
	if isRedirect(r) {
		return true
	}
	switch {
	case r == ampersand && k == rangle:
	case r == '0' && k == langle:
	case r == '1' && k == rangle:
	case r == '2' && k == rangle:
	default:
		return false
	}
	return true
}

func isBraces(r rune) bool {
	return r == lcurly || r == rcurly
}

func isList(r rune) bool {
	return r == comma || r == dot
}

func isNL(r rune) bool {
	return r == cr || r == nl
}

func isMath(r rune) bool {
	switch r {
	case lparen, rparen, plus, minus, star, slash, percent, langle, rangle, equal, bang, ampersand, pipe, question, caret, semicolon:
		return true
	default:
		return false
	}
}

type scanState int8

const (
	scanDefault scanState = iota
	scanQuote
	scanSub
	scanExp
	scanBrace
	scanMath
)

func (s scanState) String() string {
	switch s {
	default:
		return "unknown"
	case scanDefault:
		return "default"
	case scanQuote:
		return "quote"
	case scanSub:
		return "substitution"
	case scanExp:
		return "expansion"
	case scanBrace:
		return "braces"
	case scanMath:
		return "arithmetic"
	}
}

type stack []scanState

func defaultStack() stack {
	var s stack
	s.Push(scanDefault)
	return s
}

func (s *stack) Quoted() bool {
	return s.Curr() == scanQuote
}

func (s *stack) ToggleQuote() {
	if s.Quoted() {
		s.Pop()
		return
	}
	s.Push(scanQuote)
}

func (s *stack) Expansion() bool {
	return s.Curr() == scanExp
}

func (s *stack) EnterExpansion() {
	s.Push(scanExp)
}

func (s *stack) LeaveExpansion() {
	if s.Expansion() {
		s.Pop()
	}
}

func (s *stack) Arithmetic() bool {
	return s.Curr() == scanMath
}

func (s *stack) Depth() int {
	var depth int
	for i := len(*s) - 1; i >= 1; i-- {
		if (*s)[i] != scanMath || ((*s)[i] == scanMath && (*s)[i-1] != scanMath) {
			break
		}
		depth++
	}
	return depth
}

func (s *stack) EnterArithmetic() {
	s.Push(scanMath)
}

func (s *stack) LeaveArithmetic() {
	if s.Arithmetic() {
		s.Pop()
	}
}

func (s *stack) Substitution() bool {
	return s.Curr() == scanSub
}

func (s *stack) EnterSubstitution() {
	s.Push(scanSub)
}

func (s *stack) LeaveSubstitution() {
	if s.Substitution() {
		s.Pop()
	}
}

func (s *stack) Braces() bool {
	return s.Curr() == scanBrace
}

func (s *stack) AcceptBraces() bool {
	return !s.Quoted() && !s.Expansion()
}

func (s *stack) EnterBrace() {
	s.Push(scanBrace)
}

func (s *stack) LeaveBrace() {
	if s.Braces() {
		s.Pop()
	}
}

func (s *stack) Default() bool {
	curr := s.Curr()
	return curr == scanDefault || curr == scanSub
}

func (s *stack) Pop() {
	n := s.Len()
	if n == 0 {
		return
	}
	n--
	if n >= 0 {
		*s = (*s)[:n]
	}
}

func (s *stack) Push(st scanState) {
	*s = append(*s, st)
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
