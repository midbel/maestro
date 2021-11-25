package shlex_test

import (
	"strings"
	"testing"

	"github.com/midbel/maestro/shlex"
)

var list = []struct {
	Input string
	Want  []string
}{
	{
		Input: `echo`,
		Want:  []string{"echo"},
	},
	{
		Input: `echo -e foo    bar`,
		Want:  []string{"echo", "-e", "foo", "bar"},
	},
	{
		Input: `echo foo bar`,
		Want:  []string{"echo", "foo", "bar"},
	},
	{
		Input: `echo "foo bar"`,
		Want:  []string{"echo", "foo bar"},
	},
	{
		Input: `echo 'foo bar'`,
		Want:  []string{"echo", "foo bar"},
	},
	{
		Input: `echo; echo | cat |& cut; echo && cut;`,
		Want:  []string{"echo", ";", "echo", "|", "cat", "|&", "cut", ";", "echo", "&&", "cut", ";"},
	},
}

func TestSplit(t *testing.T) {
	for _, in := range list {
		str, err := shlex.Split(strings.NewReader(in.Input))
		if err != nil {
			t.Errorf("%s: unexpected error! %s", in.Input, err)
			continue
		}
		if len(str) != len(in.Want) {
			t.Errorf("%s: length mismatched! got %d, want %d", in.Input, len(str), len(in.Want))
			t.Logf("got:  %s", str)
			t.Logf("want: %s", in.Want)
			continue
		}
		for i := range str {
			if str[i] != in.Want[i] {
				t.Errorf("word mismatched! got %s, want %s", str[i], in.Want[i])
			}
		}
	}
}
