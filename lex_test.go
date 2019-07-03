package main

import (
	"strings"
	"testing"
)

func TestLexer(t *testing.T) {
	input := `
# comment should be skipped
datadir = /var/data
tmpdir  = /var/tmp
welcom  = "hello world"
mode    = 644

# assign datadir value to workdir
workdir = %(datadir)
env     = prod dev test
dirs    = %(datadir) %(tmpdir) %(workdir)

include etc/xsk/globals.xsk
include "etc/xsk/variables.xsk"

export PATH /var/bin
export HOME %(datadir)

echo %(welcom)
echo "data directory set" %(datadir)
echo "working directory set" %(workdir)

action:
  echo %(TARGET)

action():
  echo %(TARGET) %(PROPS)
`
	tokens := []Token{
		{Type: ident, Literal: "datadir"},
		{Type: equal},
		{Type: value, Literal: "/var/data"},
		{Type: nl},
		{Type: ident, Literal: "tmpdir"},
		{Type: equal},
		{Type: value, Literal: "/var/tmp"},
		{Type: nl},
		{Type: ident, Literal: "welcom"},
		{Type: equal},
		{Type: value, Literal: "hello world"},
		{Type: nl},
		{Type: ident, Literal: "mode"},
		{Type: equal},
		{Type: value, Literal: "644"},
		{Type: nl},
		{Type: ident, Literal: "workdir"},
		{Type: equal},
		{Type: variable, Literal: "datadir"},
		{Type: nl},
		{Type: ident, Literal: "env"},
		{Type: equal},
		{Type: value, Literal: "prod"},
		{Type: value, Literal: "dev"},
		{Type: value, Literal: "test"},
		{Type: nl},
		{Type: ident, Literal: "dirs"},
		{Type: equal},
		{Type: variable, Literal: "datadir"},
		{Type: variable, Literal: "tmpdir"},
		{Type: variable, Literal: "workdir"},
		{Type: nl},
		{Type: command, Literal: "include"},
		{Type: value, Literal: "etc/xsk/globals.xsk"},
		{Type: nl},
		{Type: command, Literal: "include"},
		{Type: value, Literal: "etc/xsk/variables.xsk"},
		{Type: nl},
		{Type: command, Literal: "export"},
		{Type: value, Literal: "PATH"},
		{Type: value, Literal: "/var/bin"},
		{Type: nl},
		{Type: command, Literal: "export"},
		{Type: value, Literal: "HOME"},
		{Type: variable, Literal: "datadir"},
		{Type: nl},
		{Type: command, Literal: "echo"},
		{Type: variable, Literal: "welcom"},
		{Type: nl},
		{Type: command, Literal: "echo"},
		{Type: value, Literal: "data directory set"},
		{Type: variable, Literal: "datadir"},
		{Type: nl},
		{Type: command, Literal: "echo"},
		{Type: value, Literal: "working directory set"},
		{Type: variable, Literal: "workdir"},
		{Type: nl},
		{Type: ident, Literal: "action"},
		{Type: colon},
		{Type: script, Literal: "echo %(TARGET)"},
		{Type: ident, Literal: "action"},
		{Type: lparen},
		{Type: rparen},
		{Type: colon},
		{Type: script, Literal: "echo %(TARGET) %(PROPS)"},
	}
	x, err := Lex(strings.NewReader(input))
	if err != nil {
		t.Fatalf("fail to create lexer: %s", err)
		return
	}
	for i := 0; ; i++ {
		k := x.Next()
		if k.Type == eof {
			break
		}
		if i >= len(tokens) {
			t.Fatalf("too many tokens produced (want: %d, got: %d)", len(tokens), i)
			return
		}
		got, want := k.String(), tokens[i].String()
		// t.Logf("want: %s, got: %s", want, got)
		if got != want {
			t.Fatalf("%d) unexpected token! want %s, got: %s (%02x)", i+1, want, got, k.Type)
			return
		}
	}
}
