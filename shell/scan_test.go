package shell_test

import (
	"strings"
	"testing"

	"github.com/midbel/maestro/shell"
)

var tokens = []struct {
	Input  string
	Tokens []rune
}{
	{
		Input:  `echo 'foobar' # a comment`,
		Tokens: []rune{shell.Literal, shell.Blank, shell.Literal, shell.Comment},
	},
	{
		Input:  `echo "$foobar" # a comment`,
		Tokens: []rune{shell.Literal, shell.Blank, shell.Quote, shell.Variable, shell.Quote, shell.Comment},
	},
	{
		Input:  `echo err 2> err.txt`,
		Tokens: []rune{shell.Literal, shell.Blank, shell.Literal, shell.RedirectErr, shell.Literal},
	},
	{
		Input:  `echo err 2>> err.txt`,
		Tokens: []rune{shell.Literal, shell.Blank, shell.Literal, shell.AppendErr, shell.Literal},
	},
	{
		Input:  `echo out1 1> out.txt`,
		Tokens: []rune{shell.Literal, shell.Blank, shell.Literal, shell.RedirectOut, shell.Literal},
	},
	{
		Input:  `echo out1 1>> out.txt`,
		Tokens: []rune{shell.Literal, shell.Blank, shell.Literal, shell.AppendOut, shell.Literal},
	},
	{
		Input:  `echo out > out.txt`,
		Tokens: []rune{shell.Literal, shell.Blank, shell.Literal, shell.RedirectOut, shell.Literal},
	},
	{
		Input:  `echo out >> out.txt`,
		Tokens: []rune{shell.Literal, shell.Blank, shell.Literal, shell.AppendOut, shell.Literal},
	},
	{
		Input:  `echo both &> both.txt`,
		Tokens: []rune{shell.Literal, shell.Blank, shell.Literal, shell.RedirectBoth, shell.Literal},
	},
	{
		Input:  `echo both &>> both.txt`,
		Tokens: []rune{shell.Literal, shell.Blank, shell.Literal, shell.AppendBoth, shell.Literal},
	},
	{
		Input:  `echo $etc/$plug/files/*`,
		Tokens: []rune{shell.Literal, shell.Blank, shell.Variable, shell.Literal, shell.Variable, shell.Literal},
	},
}

func TestScan(t *testing.T) {
	for _, in := range tokens {
		scan := shell.Scan(strings.NewReader(in.Input))
		for i := 0; ; i++ {
			tok := scan.Scan()
			if tok.Type == shell.EOF {
				break
			}
			if i >= len(in.Tokens) {
				t.Errorf("too many token generated! expected %d, got %d", len(in.Tokens), i)
				break
			}
			if tok.Type != in.Tokens[i] {
				t.Errorf("token mismatched! %s (got %d, want %d)", tok, tok.Type, in.Tokens[i])
				break
			}
		}
	}
}
