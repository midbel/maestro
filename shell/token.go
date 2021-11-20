package shell

import (
	"fmt"
)

const (
	EOF = -(iota + 1)
	Blank
	Literal
	Quote
	Comment
	Variable
	BegExp
	EndExp
	List
	Pipe
	PipeBoth
	And
	Or
	Assign
	RedirectIn
	RedirectOut
	RedirectBoth
	AppendOut
	Length         // ${#var}
	Slice          // ${var:from:to}
	Replace        // ${var/from/to}
	ReplaceAll     // ${var//from/to}
	ReplaceSuffix  // ${var/%from/to}
	ReplacePrefix  // ${var/#from/to}
	TrimSuffix     // ${var%suffix}
	TrimSuffixLong // ${var%%suffix}
	TrimPrefix     // ${var#suffix}
	TrimPrefixLong // ${var##suffix}
	Lower          // ${var,}
	LowerAll       // ${var,,}
	Upper          // ${var^}
	UpperAll       // ${var^^}
	ValIfUnset     // ${var:-val}
	SetValIfUnset  // ${var:=val}
	ValIfSet       // ${var:+val}
	ExitIfUnset    // ${var:?val}
	Invalid
)

type Token struct {
	Literal string
	Type    rune
}

func (t Token) IsSequence() bool {
	switch t.Type {
	case And, Or, List, Pipe, PipeBoth, Comment:
		return true
	default:
		return false
	}
}

func (t Token) Eow() bool {
	return t.Type == Comment || t.Type == EOF || t.Type == Blank || t.IsSequence()
}

func (t Token) String() string {
	var prefix string
	switch t.Type {
	case EOF:
		return "<eof>"
	case Blank:
		return "<blank>"
	case Quote:
		return "<quote>"
	case And:
		return "<and>"
	case Or:
		return "<or>"
	case Pipe:
		return "<pipe>"
	case PipeBoth:
		return "<pipe-both>"
	case List:
		return "<list>"
	case BegExp:
		return "<beg-expansion>"
	case EndExp:
		return "<end-expansion>"
	case Length:
		return "<length>"
	case Slice:
		return "<slice>"
	case Replace:
		return "<replace>"
	case ReplaceAll:
		return "<replace-all>"
	case ReplaceSuffix:
		return "<replace-suffix>"
	case ReplacePrefix:
		return "<replace-prefix>"
	case TrimSuffix:
		return "<trim-suffix>"
	case TrimSuffixLong:
		return "<trim-suffix-long>"
	case TrimPrefix:
		return "<trim-prefix>"
	case TrimPrefixLong:
		return "<trim-prefix-long>"
	case Lower:
		return "<lower>"
	case LowerAll:
		return "<lower-all>"
	case Upper:
		return "<upper>"
	case UpperAll:
		return "<upper-all>"
	case ValIfUnset:
		return "<val-if-unset>"
	case SetValIfUnset:
		return "<set-val-if-unset>"
	case ValIfSet:
		return "<val-if-set>"
	case ExitIfUnset:
		return "<exit-if-unset>"
	case Assign:
		return "<assignment>"
	case RedirectIn:
		return "<redirect-in>"
	case RedirectOut:
		return "<redirect-out>"
	case RedirectBoth:
		return "<redirect-both>"
	case AppendOut:
		return "<append-out>"
	case Variable:
		prefix = "variable"
	case Comment:
		prefix = "comment"
	case Literal:
		prefix = "literal"
	case Invalid:
		prefix = "invalid"
	default:
		prefix = "unknown"
	}
	return fmt.Sprintf("%s(%s)", prefix, t.Literal)
}
