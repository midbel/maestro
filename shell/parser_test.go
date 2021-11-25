package shell_test

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/midbel/maestro/shell"
)

var list = []struct {
	Input   string
	Invalid bool
	Len     int
}{
	{
		Input: `echo foobar`,
		Len:   1,
	},
	{
		Input: `echo "foobar"`,
		Len:   1,
	},
	{
		Input: `foobar="foo"; echo $foobar`,
		Len:   2,
	},
	{
		Input: `echo foobar | cat | cut -d b -f 1`,
		Len:   1,
	},
	{
		Input: `echo {{A,B,C,D},{001..5..1}}`,
		Len:   1,
	},
	{
		Input: `echo ${lower,,} && cat ${upper^^} || echo ${#foobar} `,
		Len:   1,
	},
	{
		Input: `echo $(cat $(echo ${file##.txt}))`,
		Len:   1,
	},
}

func TestParse(t *testing.T) {
	for _, in := range list {
		c := parse(t, in.Input, in.Invalid)
		if in.Len != c {
			t.Errorf("sequence mismatched! expected %d, got %d", in.Len, c)
		}
	}
}

func parse(t *testing.T, in string, invalid bool) int {
	t.Helper()
	var (
		p = shell.NewParser(strings.NewReader(in))
		c int
	)
	for {
		_, err := p.Parse()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			if invalid && err == nil {
				t.Errorf("expected error parsing %q! got none", in)
				return -1
			}
			if !invalid && err != nil {
				t.Errorf("expected no error parsing %q! got %s", in, err)
				return -1
			}
		}
		c++
	}
	return c
}
