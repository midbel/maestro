package maestro_test

import (
	"strings"
	"testing"

	"github.com/midbel/maestro"
)

func TestScanner(t *testing.T) {
	var (
		str    = `test = "pre1-"${foo}"-post1" "pre2-"${bar}"-post2"`
		tokens = []rune{
			maestro.Ident,
			maestro.Assign,
			maestro.String,
			maestro.Variable,
			maestro.String,
			maestro.Blank,
			maestro.String,
			maestro.Variable,
			maestro.String,
		}
	)
	s, _ := maestro.Scan(strings.NewReader(str))
	for i := 0; i < len(tokens); i++ {
		tok := s.Scan()
		if tok.IsInvalid() {
			t.Fatalf("invalid token found")
		}
		if tok.IsEOF() {
			break
		}
		if tok.Type != tokens[i] {
			t.Fatalf("token mismatched! %s", tok)
		}
	}
}
