package expand

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
)

type resolver map[string]string

func empty() Resolver {
	return make(resolver)
}

func (r resolver) Define(ident, value string) {
	r[ident] = value
}

func (r resolver) Resolve(ident string) ([]string, error) {
	v, ok := r[ident]
	if !ok {
		return nil, fmt.Errorf("%s: identifier not defined", ident)
	}
	return []string{v}, nil
}

func TestExpand(t *testing.T) {
	data := []struct {
		Input string
		Want  []string
	}{
		{
			Input: "echo foobar",
			Want:  []string{"echo", "foobar"},
		},
		{
			Input: "echo $var",
			Want:  []string{"echo", "foobar"},
		},
		{
			Input: "echo foo-$var-bar",
			Want:  []string{"echo", "foo-foobar-bar"},
		},
		{
			Input: "${#var}",
			Want:  []string{"6"},
		},
		{
			Input: "${var#foo}",
			Want:  []string{"bar"},
		},
		{
			Input: "${var%bar}",
			Want:  []string{"foo"},
		},
		{
			Input: "${var//oo/ee}",
			Want:  []string{"feebar"},
		},
		{
			Input: "${var/#foo/bar}",
			Want:  []string{"barbar"},
		},
		{
			Input: "${var/%bar/foo}",
			Want:  []string{"foofoo"},
		},
		{
			Input: "${var/oo/ee}",
			Want:  []string{"feebar"},
		},
		{
			Input: "${var:3}",
			Want:  []string{"bar"},
		},
		{
			Input: "${var:0:3}",
			Want:  []string{"foo"},
		},
		{
			Input: "{foo,bar}",
			Want:  []string{"foo", "bar"},
		},
		{
			Input: "pre-{foo,bar}",
			Want:  []string{"pre-foo", "pre-bar"},
		},
		{
			Input: "{foo,bar}-post",
			Want:  []string{"foo-post", "bar-post"},
		},
		{
			Input: "{pre,{foo,bar},post}",
			Want:  []string{"pre", "foo", "bar", "post"},
		},
	}
	resolv := make(resolver)
	resolv.Define("var", "foobar")
	for _, d := range data {
		got, err := ExpandString(d.Input, resolv)
		if err != nil {
			t.Errorf("%s: unexpected error: %s", d.Input, err)
			continue
		}
		if !cmp.Equal(d.Want, got) {
			t.Errorf("%s: results mismatched! want %s, got %s", d.Input, d.Want, got)
		}
	}
}
