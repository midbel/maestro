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
		Input:  `echo foobar 2> err.txt`,
		Tokens: []rune{shell.Literal, shell.Blank, shell.Literal, shell.RedirectErr, shell.Literal},
	},
	{
		Input:  `echo foobar 2>> err.txt`,
		Tokens: []rune{shell.Literal, shell.Blank, shell.Literal, shell.AppendErr, shell.Literal},
	},
}

func TestScan(t *testing.T) {
	for _, in := range tokens {
		scan := shell.Scan(strings.NewReader(in.Input))
		for i := 0; ; i++ {
			tok := scan.Scan()
      t.Logf("current: %s", tok)
			if tok.Type == shell.EOF {
				break
			}
			if tok.Type == shell.Invalid {
				t.Errorf("invalid token generated")
				break
			}
			if i >= len(in.Tokens) {
				t.Errorf("too many token generated! expected %d, got %d", len(in.Tokens), i)
				break
			}
			if tok.Type != in.Tokens[i] {
				t.Errorf("token mismatched! %s", tok)
			}
		}
	}
}
