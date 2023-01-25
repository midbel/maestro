package scan

import (
	"fmt"
)

type Position struct {
	Line   int
	Column int
}

func (p Position) String() string {
	return fmt.Sprintf("%d:%d", p.Line, p.Column)
}

const (
	Eof rune = -(iota + 1)
	Eol
	Blank
	Comment
	Ident
	Keyword
	String
	Boolean
	Variable
	Meta
	Script
	Quote
	Assign
	Append
	Comma
	Dependency
	BegList
	EndList
	BegScript
	EndScript
	Hidden
	Invalid
)

const (
	KwTrue    = "true"
	KwFalse   = "false"
	KwExport  = "export"
	KwAlias   = "alias"
	KwDelete  = "delete"
	KwInclude = "include"
)

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
	case Hidden:
		return "<hidden>"
	case Eof:
		return "<eof>"
	case Eol:
		return "<eol>"
	case Assign:
		return "<assign>"
	case Append:
		return "<append>"
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
	case Quote:
		return "<quote>"
	case Ident:
		prefix = "ident"
	case String:
		prefix = "string"
	case Boolean:
		prefix = "boolean"
	case Meta:
		prefix = "meta"
	case Variable:
		prefix = "variable"
	case Comment:
		prefix = "comment"
	case Script:
		prefix = "script"
	case Keyword:
		prefix = "keyword"
	}
	return fmt.Sprintf("%s(%s)", prefix, t.Literal)
}