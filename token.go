package maestro

import (
  "fmt"
)

const (
	Eof rune = -(iota + 1)
	Eol
	Comment
	Ident
	Meta
	Command
	Script
	Assign
	Comma
	Dependency
	BegList
	EndList
	BegScript
	EndScript
	Invalid
)

type Position struct {
	Line   int
	Column int
}

type Token struct {
	Literal string
	Type    rune
	Position
}

func (t Token) String() string {
	var prefix string
	switch t.Type {
	default:
		prefix = "unknown"
	case Eof:
		return "<eof>"
	case Eol:
		return "<eol>"
	case Assign:
		return "<assign>"
	case Comma:
		return "<comma>"
	case Dependency:
		return "<dependency>"
	case BegList:
		return "<beg-list>"
	case EndList:
		return "<end-list>"
	case BegScript:
		return "<beg-script>"
	case EndScript:
		return "<end-script>"
	case Invalid:
		return "<invalid>"
	case Ident:
		prefix = "ident"
	case Meta:
		prefix = "meta"
	case Comment:
		prefix = "comment"
	case Script:
		prefix = "script"
	}
	return fmt.Sprintf("%s(%s)", prefix, t.Literal)
}

func (t Token) IsEOF() bool {
	return t.Type == Eof
}

func (t Token) IsInvalid() bool {
	return t.Type == Invalid
}
