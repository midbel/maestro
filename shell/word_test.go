package shell_test

import (
	"testing"

	"github.com/midbel/maestro/shell"
)

func TestExpander(t *testing.T) {
	data := []struct {
		shell.Expander
		Want []string
	}{
		{
			Expander: createSlice("foobar", 0, 3),
			Want:     []string{"foo"},
		},
		{
			Expander: createSlice("foobar", 0, 10),
			Want:     []string{"foobar"},
		},
		{
			Expander: createSlice("foobar", 3, 0),
			Want:     []string{"bar"},
		},
		{
			Expander: createSlice("foobar", 3, 3),
			Want:     []string{"bar"},
		},
		{
			Expander: createSlice("foobar", 3, -3),
			Want:     []string{"foo"},
		},
		{
			Expander: createSlice("foobar", 3, 10),
			Want:     []string{"bar"},
		},
		{
			Expander: createSlice("foobar", 10, 10),
			Want:     []string{""},
		},
		{
			Expander: createSlice("foobar", -3, 0),
			Want:     []string{"bar"},
		},
		{
			Expander: createSlice("foobar", 0, -3),
			Want:     []string{"bar"},
		},
	}
	env := shell.EmptyEnv()
	for i, d := range data {
		env.Define("foobar", []string{"foobar"})

		got, err := d.Expand(env, false)
		if err != nil {
			t.Errorf("unexpected error expanding foobar! %s", err)
			continue
		}
		if len(got) != len(d.Want) {
			t.Errorf("length mismatched! want %d, got %d", len(d.Want), len(got))
			continue
		}
		for j := range d.Want {
			if d.Want[j] != got[j] {
				t.Errorf("%d) strings mismatched! want %s, got %s", i+1, d.Want[j], got[j])
			}
		}
	}
}

func createSlice(ident string, off, siz int) shell.Expander {
	return shell.ExpandSlice{
		Ident:  ident,
		Offset: off,
		Size:   siz,
	}
}
