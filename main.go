package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"unicode/utf8"
)

func main() {
	flag.Parse()
	r, err := os.Open(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(125)
	}
	defer r.Close()

	x, err := Lex(r)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(125)
	}
	for k := x.Next(); k.Type != eof; k = x.Next() {
		fmt.Println(k)
	}
}

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
	}
	return fmt.Sprintf("<%s '%s'>", str, t.Literal)
}

// map between recognized commands and their expected number of arguments
var commands = map[string]int{
	"echo":    -1,
	"export":  2,
	"include": 1,
}

const (
	space   = ' '
	tab     = '\t'
	period  = '.'
	colon   = ':'
	percent = '%'
	lparen  = '('
	rparen  = ')'
	comment = '#'
	quote   = '"'
	equal   = '='
	comma   = ','
	nl      = '\n'
)

const (
	eof rune = -(iota + 1)
	ident
	value
	variable
	command // include, export
	script
	invalid
)

const (
	lexDefault = iota
	lexValue
	lexScript
	lexDone
)

type lexer struct {
	inner []byte

	state int

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
	switch x.state {
	case lexValue:
		x.nextValue(&t)
	case lexScript:
		x.nextScript(&t)
	case lexDone:
	default:
		x.nextDefault(&t)
	}
	switch t.Type {
	case colon:
		x.state = lexScript
	case equal, command:
		x.state = lexValue
	case nl, script:
		x.state = lexDefault
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
		if x.char == nl {
			x.skipSpace()
		}
		if isComment(x.char) {
			x.skipComment()
			continue
		}
		str.WriteRune(x.char)
		x.readRune()
	}
	t.Literal, t.Type = str.String(), script
}

func (x *lexer) nextValue(t *Token) {
	if x.char == space {
		x.readRune()
	}
	switch {
	case x.char == nl:
		t.Type = nl
	case isQuote(x.char):
		x.readString(t)
	case x.char == percent:
		x.readVariable(t)
	default:
		x.readValue(t)
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
		x.skipComment()
		x.nextDefault(t)
	default:
		t.Type = x.char
	}
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
	for x.char != space && x.char != nl {
		x.readRune()
	}
	t.Literal, t.Type = string(x.inner[pos:x.pos]), value
	x.unreadRune()
}

func (x *lexer) readString(t *Token) {
	x.readRune()
	pos := x.pos
	for !isQuote(x.char) {
		x.readRune()
	}
	t.Literal, t.Type = string(x.inner[pos:x.pos]), value
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

func (x *lexer) skipComment() {
	for x.char != nl {
		x.readRune()
	}
}

func (x *lexer) skipSpace() {
	for isSpace(x.char) {
		x.readRune()
	}
}

func isQuote(x rune) bool {
	return x == quote
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
