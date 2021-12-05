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
		Input: `echo pre-"foobar"-post`,
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
		Input: `echo pre-{1..5}-post`,
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
	{
		Input: `for ident in {1..5}; do echo $ident done`,
		Len:   1,
	},
	{
		Input: `for ident in $(seq 1 5 10); do echo $ident done`,
		Len:   1,
	},
	{
		Input: `for ident in {1..5}; do echo $ident else echo zero; done`,
		Len:   1,
	},
	{
		Input: `while true; do echo foo; done`,
		Len:   1,
	},
	{
		Input: `until true; do echo foo; done`,
		Len:   1,
	},
	// {
	// 	Input: `if $foo; then echo foo; fi`,
	// 	Len:   1,
	// },
	{
		Input: `if $foo; then echo foo; else if $bar; then echo bar; else echo foobar; fi`,
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
