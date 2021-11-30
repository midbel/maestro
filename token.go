package maestro

import (
	"fmt"
)

const (
	kwTrue    = "true"
	kwFalse   = "false"
	kwInclude = "include"
	kwExport  = "export"
	kwDelete  = "delete"
	kwAlias   = "alias"
)

const (
	Eof rune = -(iota + 1)
	Eol
	Comment
	Ident
	Keyword
	String
	Integer
	Boolean
	Variable
	Meta
	Script
	Macro
	Assign
	Append
	Comma
	Background
	Dependency
	BegList
	EndList
	BegScript
	EndScript
	Reverse
	Ignore
	Echo
	Copy
	Subshell
	Invalid
	Optional
	Hidden
	Expand
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
	case Expand:
		return "<expand>"
	case Echo:
		return "<echo>"
	case Optional:
		return "<optional>"
	case Hidden:
		return "<hidden>"
	case Reverse:
		return "<reverse>"
	case Ignore:
		return "<ignore>"
	case Copy:
		return "<copy>"
	case Subshell:
		return "<subshell>"
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
	case Background:
		return "<background>"
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
	case String:
		prefix = "string"
	case Boolean:
		prefix = "boolean"
	case Integer:
		prefix = "integer"
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
	case Macro:
		prefix = "macro"
	}
	return fmt.Sprintf("%s(%s)", prefix, t.Literal)
}

func (t Token) IsAssign() bool {
	return t.Type == Append || t.Type == Assign
}

func (t Token) IsVariable() bool {
	return t.Type == Variable
}

func (t Token) IsValue() bool {
	return t.IsVariable() || t.IsPrimitive() || t.IsScript()
}

func (t Token) IsScript() bool {
	return t.Type == Script
}

func (t Token) IsPrimitive() bool {
	return t.Type == Ident || t.Type == String || t.Type == Boolean || t.Type == Integer
}

func (t Token) IsEOF() bool {
	return t.Type == Eof
}

func (t Token) IsInvalid() bool {
	return t.Type == Invalid
}

func (t Token) IsOperator() bool {
	switch t.Type {
	case Echo, Reverse, Ignore, Copy, Subshell:
		return true
	default:
		return false
	}
}
