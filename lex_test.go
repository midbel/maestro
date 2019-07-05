package main

import (
	"strings"
	"testing"
)

func TestLexer(t *testing.T) {
	t.Run("Variables", testScanVariables)
	t.Run("Commands", testScanCommands)
	t.Run("Scripts", testScanScripts)
	t.Run("Specials", testScanSpecialVariables)
	t.Run("Scripts+Deps", testScanScriptsWithDependencies)
	t.Run("Scripts+Specials", testScanScriptsSpecials)
}

func testLexer(t *testing.T, input string, tokens []Token) {
	t.Helper()

	x, err := Lex(strings.NewReader(input))
	if err != nil {
		t.Fatalf("fail to create lexer: %s", err)
	}
	for i := 0; ; i++ {
		k := x.Next()
		if k.Type == eof {
			break
		}
		if i >= len(tokens) {
			t.Logf("unexpected token! %s", k)
			t.Fatalf("too many tokens produced (want: %d, got: %d)", len(tokens), i+1)
		}
		got, want := k.String(), tokens[i].String()
		t.Log(got, want)
		if got != want {
			t.Fatalf("%d) unexpected token! want %s, got: %s (%02x)", i+1, want, got, k.Type)
		}
	}
}

func testScanScriptsSpecials(t *testing.T) {
	input := `
empty1:
empty2:
test: empty1 empty2
sleep(shell=bash):
	sleep 3
`
	tokens := []Token{
		{Type: ident, Literal: "empty1"},
		{Type: colon},
		{Type: ident, Literal: "empty2"},
		{Type: colon},
		{Type: ident, Literal: "test"},
		{Type: colon},
		{Type: dependency, Literal: "empty1"},
		{Type: dependency, Literal: "empty2"},
		{Type: ident, Literal: "sleep"},
		{Type: lparen},
		{Type: ident, Literal: "shell"},
		{Type: equal},
		{Type: value, Literal: "bash"},
		{Type: rparen},
		{Type: colon},
		{Type: script, Literal: "sleep 3"},
	}
	testLexer(t, input, tokens)
}

func testScanScriptsWithDependencies(t *testing.T) {
	input := `
restart(shell=bash): stop start
  echo %(TARGET) "restarted"
`
	tokens := []Token{
		{Type: ident, Literal: "restart"},
		{Type: lparen},
		{Type: ident, Literal: "shell"},
		{Type: equal},
		{Type: value, Literal: "bash"},
		{Type: rparen},
		{Type: colon},
		{Type: dependency, Literal: "stop"},
		{Type: dependency, Literal: "start"},
		{Type: script, Literal: "echo %(TARGET) \"restarted\""},
	}
	testLexer(t, input, tokens)
}

func testScanScripts(t *testing.T) {
	input := `
single:
  echo %(TARGET)

  echo %(TARGET) %(PROPS)

multiline():
  echo %(TARGET) %(PROPS)
  echo %(TARGET) %(PROPS)

  echo %(TARGET) %(PROPS)

empty:

reload(shell=bash,retry=5):
  sudo service %(TARGET) reload

config1(
  shell=ksh,
  env=true,
  home=%(datadir),
):
  sudo service %(TARGET) test

# same as config1 without the trailing comma after the home property
config2(
  shell=ksh,
  env=true,
  home=%(datadir)
):
  sudo service %(TARGET) test

restart(shell=bash): stop start
  echo %(TARGET) "restarted"

# comment
# singlebis:
#  echo %(TARGET)
`
	tokens := []Token{
		{Type: ident, Literal: "single"},
		{Type: colon},
		{Type: script, Literal: "echo %(TARGET)\n\necho %(TARGET) %(PROPS)"},
		{Type: ident, Literal: "multiline"},
		{Type: lparen},
		{Type: rparen},
		{Type: colon},
		{Type: script, Literal: "echo %(TARGET) %(PROPS)\necho %(TARGET) %(PROPS)\n\necho %(TARGET) %(PROPS)"},
		{Type: ident, Literal: "empty"},
		{Type: colon},
		// {Type: script, Literal: ""}
		{Type: ident, Literal: "reload"},
		{Type: lparen},
		{Type: ident, Literal: "shell"},
		{Type: equal},
		{Type: value, Literal: "bash"},
		{Type: comma},
		{Type: ident, Literal: "retry"},
		{Type: equal},
		{Type: value, Literal: "5"},
		{Type: rparen},
		{Type: colon},
		{Type: script, Literal: "sudo service %(TARGET) reload"},
		{Type: ident, Literal: "config1"},
		{Type: lparen},
		{Type: ident, Literal: "shell"},
		{Type: equal},
		{Type: value, Literal: "ksh"},
		{Type: comma},
		{Type: ident, Literal: "env"},
		{Type: equal},
		{Type: value, Literal: "true"},
		{Type: comma},
		{Type: ident, Literal: "home"},
		{Type: equal},
		{Type: variable, Literal: "datadir"},
		{Type: comma},
		{Type: rparen},
		{Type: colon},
		{Type: script, Literal: "sudo service %(TARGET) test"},
		{Type: ident, Literal: "config2"},
		{Type: lparen},
		{Type: ident, Literal: "shell"},
		{Type: equal},
		{Type: value, Literal: "ksh"},
		{Type: comma},
		{Type: ident, Literal: "env"},
		{Type: equal},
		{Type: value, Literal: "true"},
		{Type: comma},
		{Type: ident, Literal: "home"},
		{Type: equal},
		{Type: variable, Literal: "datadir"},
		{Type: rparen},
		{Type: colon},
		{Type: script, Literal: "sudo service %(TARGET) test"},
		{Type: ident, Literal: "restart"},
		{Type: lparen},
		{Type: ident, Literal: "shell"},
		{Type: equal},
		{Type: value, Literal: "bash"},
		{Type: rparen},
		{Type: colon},
		{Type: dependency, Literal: "stop"},
		{Type: dependency, Literal: "start"},
		{Type: script, Literal: "echo %(TARGET) \"restarted\""},
	}
	testLexer(t, input, tokens)
}

func testScanCommands(t *testing.T) {
	input := `
include etc/xsk/globals.xsk
include "etc/xsk/variables.xsk"

export PATH /var/bin
export HOME %(datadir)

echo %(welcom)
echo "data directory set" %(datadir)
echo "working directory set" %(workdir)
`
	tokens := []Token{
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
	}
	testLexer(t, input, tokens)
}

func testScanSpecialVariables(t *testing.T) {
	input := `
.ALL     = xxh md5 sha1
.DEFAULT = all
.ECHO = on

.ABOUT = "test scan special variables"
.USAGE = "go test -v -cover"
`

	tokens := []Token{
		{Type: meta, Literal: "ALL"},
		{Type: equal},
		{Type: value, Literal: "xxh"},
		{Type: value, Literal: "md5"},
		{Type: value, Literal: "sha1"},
		{Type: nl},
		{Type: meta, Literal: "DEFAULT"},
		{Type: equal},
		{Type: value, Literal: "all"},
		{Type: nl},
		{Type: meta, Literal: "ECHO"},
		{Type: equal},
		{Type: value, Literal: "on"},
		{Type: nl},
		{Type: meta, Literal: "ABOUT"},
		{Type: equal},
		{Type: value, Literal: "test scan special variables"},
		{Type: nl},
		{Type: meta, Literal: "USAGE"},
		{Type: equal},
		{Type: value, Literal: "go test -v -cover"},
		{Type: nl},
	}
	testLexer(t, input, tokens)
}

func testScanVariables(t *testing.T) {
	input := `
# comment should be skipped
datadir  = /var/data
tmpdir   = /var/tmp
welcom1  = "hello world"
welcom2  = "hello \"world\""
mode     = 644

# assign datadir value to workdir
workdir = %(datadir)
env     = prod dev test
dirs    = %(datadir) %(tmpdir) %(workdir)
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
		{Type: ident, Literal: "welcom1"},
		{Type: equal},
		{Type: value, Literal: "hello world"},
		{Type: nl},
		{Type: ident, Literal: "welcom2"},
		{Type: equal},
		{Type: value, Literal: "hello \"world\""},
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
	}
	testLexer(t, input, tokens)
}
