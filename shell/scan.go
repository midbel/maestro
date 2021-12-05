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

	str      bytes.Buffer
	quoted   bool
	expanded bool
	braced   int
}

func Scan(r io.Reader) *Scanner {
	buf, _ := io.ReadAll(r)
	s := Scanner{
		input: buf,
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
	switch {
	case isBraces(s.char) && (!s.quoted && !s.expanded):
		s.scanBraces(&tok)
	case isList(s.char) && s.isBrace():
		s.scanList(&tok)
	case isOperator(s.char) && s.expanded:
		s.scanOperator(&tok)
	case isBlank(s.char) && !s.quoted:
		tok.Type = Blank
		s.skipBlank()
	case isSequence(s.char) && !s.quoted:
		s.scanSequence(&tok)
	case isRedirectBis(s.char, s.peek()) && !s.quoted:
		s.scanRedirect(&tok)
	case isAssign(s.char) && !s.quoted:
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

func (s *Scanner) scanQuote(tok *Token) {
	tok.Type = Quote
	s.read()
	s.toggleQuote()
	if s.quoted {
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
		s.leaveBrace()
	case s.char == lcurly && k != rcurly:
		tok.Type = BegBrace
		s.enterBrace()
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
		s.expanded = false
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
		s.expanded = true
		s.read()
		return
	}
	if s.char == lparen {
		tok.Type = BegSub
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
	if s.quoted {
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
		if s.expanded && isOperator(s.char) {
			break
		}
		s.write()
		s.read()
	}
	tok.Type = Literal
	tok.Literal = s.string()
}

func (s *Scanner) toggleQuote() {
	s.quoted = !s.quoted
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
	if s.isBrace() && (s.char == dot || s.char == comma || s.char == rcurly) {
		return true
	}
	if s.expanded && isOperator(r) {
		return true
	}
	if s.char == lcurly {
		return s.peek() != rcurly
	}
	ok := isBlank(s.char) || isSequence(s.char) || isDouble(s.char) ||
		isVariable(s.char) || isAssign(s.char)
	return ok
}

func (s *Scanner) isBrace() bool {
	return s.braced > 0
}

func (s *Scanner) enterBrace() {
	s.braced++
}

func (s *Scanner) leaveBrace() {
	s.braced++
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
