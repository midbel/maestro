package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"strings"
	"unicode/utf8"
)

type Token struct {
	Literal string
	Type    rune
}

func (t Token) String() string {
	var str string
	switch t.Type {
	default:
		return fmt.Sprintf("<punct '%c'>", t.Type)
	case nl:
		return "<NL>"
	case command:
		str = "command"
	case ident:
		str = "ident"
	case variable:
		str = "variable"
	case value:
		str = "value"
	case script:
		str = "script"
	case dependency:
		str = "dependency"
	case meta:
		str = "meta"
	case comment:
		str = "comment"
	}
	return fmt.Sprintf("<%s '%s'>", str, t.Literal)
}

// map between recognized commands and their expected number of arguments
var commands = map[string]int{
	"echo":    -1,
	"declare": -1,
	"export":  2,
	"include": 1,
	"clear":   0,
}

const (
	space     = ' '
	tab       = '\t'
	period    = '.'
	colon     = ':'
	percent   = '%'
	lparen    = '('
	rparen    = ')'
	comment   = '#'
	quote     = '"'
	tick      = '`'
	equal     = '='
	comma     = ','
	nl        = '\n'
	backslash = '\\'
	plus      = '+'
	minus     = '-'
)

const (
	eof rune = -(iota + 1)
	meta
	ident
	value
	variable
	command // include, export, echo, declare
	script
	dependency
	invalid
)

const (
	lexDefault uint16 = iota << 8
	lexValue
	lexDeps
	lexScript
)

const (
	lexNoop uint16 = iota
	lexProps
	lexMeta
)

type lexer struct {
	inner []byte

	state uint16

	char rune
	pos  int
	next int
}

func Lex(r io.Reader) (*lexer, error) {
	xs, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	x := lexer{
		inner: xs,
		state: lexDefault,
	}
	x.readRune()
	return &x, nil
}

func (x *lexer) Next() Token {
	var t Token
	if x.char == eof || x.char == invalid {
		t.Type = x.char
		return t
	}
	switch state := x.state & 0xFF00; state {
	case lexValue:
		x.nextValue(&t)
		if state, peek := x.state&0xFF, x.peekRune(); state == lexProps && isSpace(peek) {
			x.readRune()
			x.skipSpace()
			x.unreadRune()
		}
	case lexScript:
		x.nextScript(&t)
	case lexDeps:
		x.nextDependency(&t)
	default:
		x.nextDefault(&t)
	}
	switch t.Type {
	case colon:
		x.state = lexDeps | lexNoop
	case equal, command:
		x.state |= lexValue
	case lparen, comma:
		x.state = lexDefault | lexProps
	case nl:
		if state := x.state & 0xFF00; state == lexDeps {
			x.state |= lexScript
			return x.Next()
		} else {
			x.state = lexDefault | lexNoop
		}
	case rparen:
		x.state = lexDefault | lexNoop
	case script:
		x.state = lexDefault | lexNoop
		if t.Literal == "" {
			return x.Next()
		}
	}
	x.readRune()
	return t
}

func (x *lexer) nextScript(t *Token) {
	done := func() bool {
		if x.char == eof {
			return true
		}
		peek := x.peekRune()
		return x.char == nl && (!isSpace(peek) || peek == eof || peek == comment)
	}

	var str strings.Builder
	for !done() {
		if peek := x.peekRune(); x.char == nl && peek != nl {
			str.WriteRune(x.char)
			x.readRune()
			x.skipSpace()
		}
		str.WriteRune(x.char)
		x.readRune()
	}
	t.Literal, t.Type = strings.TrimSpace(str.String()), script
}

func (x *lexer) nextValue(t *Token) {
	if x.char == space {
		x.skipSpace()
	}
	switch {
	case x.char == nl || x.char == comma || x.char == rparen:
		t.Type = x.char
	case x.char == minus:
		t.Literal, t.Type = "-", value
	case isQuote(x.char):
		x.readString(t)
	case x.char == percent:
		x.readVariable(t)
	default:
		x.readValue(t)
	}
}

func (x *lexer) nextDependency(t *Token) {
	if x.char == space {
		x.skipSpace()
	}
	if isIdent(x.char) {
		x.readIdent(t)
		t.Type = dependency
	} else if x.char == nl || x.char == plus {
		t.Type = x.char
	} else {
		t.Type = invalid
	}
}

func (x *lexer) nextDefault(t *Token) {
	x.skipSpace()
	switch {
	case isIdent(x.char):
		x.readIdent(t)
	case isQuote(x.char):
		x.readString(t)
	case isComment(x.char):
		x.readComment(t)
	case x.char == period:
		x.readRune()
		x.readIdent(t)
		t.Type = meta
	default:
		t.Type = x.char
	}
}

func (x *lexer) countRuneUntil(fn func(rune) bool) int {
	var (
		i int
		n = x.pos
	)
	for {
		k, nn := utf8.DecodeRune(x.inner[n:])
		if fn(k) || k == utf8.RuneError {
			break
		}
		n += nn
		i++
	}
	return i
}

func (x *lexer) readVariable(t *Token) {
	x.readRune()
	if x.char != lparen {
		t.Type = invalid
		return
	}
	x.readRune()

	pos := x.pos
	for x.char != rparen {
		if x.char == space || x.char == nl {
			t.Type = invalid
			return
		}
		x.readRune()
	}
	t.Literal, t.Type = string(x.inner[pos:x.pos]), variable
}

func (x *lexer) readIdent(t *Token) {
	pos := x.pos
	for isIdent(x.char) || isDigit(x.char) {
		x.readRune()
	}
	t.Literal, t.Type = string(x.inner[pos:x.pos]), ident
	if _, ok := commands[t.Literal]; ok {
		t.Type = command
	}
	x.unreadRune()
}

func (x *lexer) readValue(t *Token) {
	pos := x.pos
	for {
		switch x.char {
		case space, nl, comma, rparen:
			t.Literal, t.Type = string(x.inner[pos:x.pos]), value
			x.unreadRune()

			return
		default:
			x.readRune()
		}
	}
}

func (x *lexer) readComment(t *Token) {
	x.readRune()
	if x.char == space {
		x.skipSpace()
	}
	pos := x.pos
	for x.char != nl {
		x.readRune()
	}
	t.Literal, t.Type = string(x.inner[pos:x.pos]), comment
}

func (x *lexer) readString(t *Token) {
	ticky := x.char == tick
	var eos rune
	if ticky {
		eos = tick
	} else {
		eos = quote
	}
	x.readRune()

	var b strings.Builder
	for x.char != eos {
		if !ticky && x.char == backslash {
			if peek := x.peekRune(); peek == quote {
				x.readRune()
			}
		}
		b.WriteRune(x.char)
		x.readRune()
	}
	t.Literal, t.Type = b.String(), value
	if ticky {
		t.Literal = strings.TrimLeft(t.Literal, "\n\t ")
	}
	// t.Literal, t.Type = string(x.inner[pos:x.pos]), value
}

func (x *lexer) readRune() {
	if x.pos > 0 {
		if x.char == eof || x.char == invalid {
			return
		}
	}
	k, n := utf8.DecodeRune(x.inner[x.next:])
	if k == utf8.RuneError {
		if n == 0 {
			x.char = eof
		} else {
			x.char = invalid
		}
	} else {
		x.char = k
	}
	x.pos = x.next
	x.next += n
}

func (x *lexer) unreadRune() {
	x.next = x.pos
	x.pos -= utf8.RuneLen(x.char)
}

func (x *lexer) peekRune() rune {
	k, _ := utf8.DecodeRune(x.inner[x.next:])
	return k
}

func (x *lexer) skipSpace() {
	for isSpace(x.char) {
		x.readRune()
	}
}

func isQuote(x rune) bool {
	return x == quote || x == tick
}

func isSpace(x rune) bool {
	return x == space || x == tab || x == nl
}

func isComment(x rune) bool {
	return x == comment
}

func isIdent(x rune) bool {
	return (x >= 'a' && x <= 'z') || (x >= 'A' && x <= 'Z')
}

func isDigit(x rune) bool {
	return x >= '0' && x <= '9'
}
