package shell_test

import (
	"testing"

	"github.com/midbel/maestro/shell"
)

func TestExpander(t *testing.T) {
	data := []struct {
		Name string
		shell.Expander
		Want []string
	}{
		{
			Name:     "slice",
			Expander: createSlice("foobar", 0, 3),
			Want:     []string{"foo"},
		},
		{
			Name:     "slice",
			Expander: createSlice("foobar", 0, 10),
			Want:     []string{"foobar"},
		},
		{
			Name:     "slice",
			Expander: createSlice("foobar", 3, 0),
			Want:     []string{"bar"},
		},
		{
			Name:     "slice",
			Expander: createSlice("foobar", 3, 3),
			Want:     []string{"bar"},
		},
		{
			Name:     "slice",
			Expander: createSlice("foobar", 3, -3),
			Want:     []string{"foo"},
		},
		{
			Name:     "slice",
			Expander: createSlice("foobar", 3, 10),
			Want:     []string{"bar"},
		},
		{
			Name:     "slice",
			Expander: createSlice("foobar", 10, 10),
			Want:     []string{""},
		},
		{
			Name:     "slice",
			Expander: createSlice("foobar", -3, 0),
			Want:     []string{"bar"},
		},
		{
			Name:     "slice",
			Expander: createSlice("foobar", 0, -3),
			Want:     []string{"bar"},
		},
		{
			Name:     "list-brace",
			Expander: createListBrace("pre-", "-post", "foo", "bar"),
			Want:     []string{"pre-foo-post", "pre-bar-post"},
		},
		{
			Name:     "list-brace",
			Expander: createListBrace("", "-post", "foo", "bar"),
			Want:     []string{"foo-post", "bar-post"},
		},
		{
			Name:     "list-brace",
			Expander: createListBrace("pre-", "", "foo", "bar"),
			Want:     []string{"pre-foo", "pre-bar"},
		},
		{
			Name:     "range-brace",
			Expander: createRangeBrace(1, 3, 1, "pre-", "-post"),
			Want:     []string{"pre-1-post", "pre-2-post", "pre-3-post"},
		},
	}
	env := shell.EmptyEnv()
	env.Define("foobar", []string{"foobar"})
	for i, d := range data {
		t.Run(d.Name, func(t *testing.T) {
			got, err := d.Expand(env, false)
			if err != nil {
				t.Fatalf("unexpected error expanding foobar! %s", err)
			}
			if len(got) != len(d.Want) {
				t.Fatalf("length mismatched! want %d, got %d", len(d.Want), len(got))
			}
			for j := range d.Want {
				if d.Want[j] != got[j] {
					t.Errorf("%d) strings mismatched! want %s, got %s", i+1, d.Want[j], got[j])
				}
			}
		})
	}
}

func createRangeBrace(from, to, step int, prefix, suffix string) shell.Expander {
	if step == 0 {
		step = 1
	}
	b := shell.ExpandRangeBrace{
		From: from,
		To:   to,
		Step: step,
	}
	if prefix != "" {
		b.Prefix = createWord(prefix)
	}
	if suffix != "" {
		b.Suffix = createWord(suffix)
	}
	return b
}

func createListBrace(prefix, suffix string, words ...string) shell.Expander {
	var b shell.ExpandListBrace
	for i := range words {
		b.Words = append(b.Words, createWord(words[i]))
	}
	if prefix != "" {
		b.Prefix = createWord(prefix)
	}
	if suffix != "" {
		b.Suffix = createWord(suffix)
	}
	return b
}

func createWord(word string) shell.Expander {
	w := shell.ExpandWord{
		Literal: word,
	}
	return w
}

func createSlice(ident string, off, siz int) shell.Expander {
	return shell.ExpandSlice{
		Ident:  ident,
		Offset: off,
		Size:   siz,
	}
}
