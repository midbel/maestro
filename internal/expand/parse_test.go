package expand

import (
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

type expandCase struct {
	Input string
	Want  []Expander
}

func TestParse(t *testing.T) {
	t.Run("simple", testSimple)
	t.Run("expansion", testExpansion)
	t.Run("braces", testBraces)
}

func testBraces(t *testing.T) {
	list := []Expander{
		createLiteral("foo"),
		createLiteral("bar"),
	}
	data := []expandCase{
		{
			Input: "{foo,bar}",
			Want: []Expander{
				createBrace(list, nil, nil),
			},
		},
		{
			Input: "{pre,{foo,bar},post}",
			Want: []Expander{
				createBrace([]Expander{
					createLiteral("pre"),
					createBrace(list, nil, nil),
					createLiteral("post"),
				}, nil, nil),
			},
		},
		{
			Input: "{pre-{foo,bar}-post,foobar}",
			Want: []Expander{
				createBrace(
					[]Expander{
						createBrace(
							list,
							createLiteral("pre-"),
							createLiteral("-post"),
						),
						createLiteral("foobar"),
					},
					nil,
					nil,
				),
			},
		},
		{
			Input: "pre-{foo,bar}",
			Want: []Expander{
				createBrace(list, createLiteral("pre-"), nil),
			},
		},
		{
			Input: "{foo,bar}-post",
			Want: []Expander{
				createBrace(list, nil, createLiteral("-post")),
			},
		},
		{
			Input: "pre-{foo,bar}-post",
			Want: []Expander{
				createBrace(
					list,
					createLiteral("pre-"),
					createLiteral("-post"),
				),
			},
		},
	}
	testExpandCase(t, data)
}

func testSimple(t *testing.T) {
	data := []expandCase{
		{
			Input: "foobar",
			Want: []Expander{
				createLiteral("foobar"),
			},
		},
		{
			Input: "echo foobar",
			Want: []Expander{
				createLiteral("echo"),
				createLiteral("foobar"),
			},
		},
		{
			Input: "echo \"foobar\"",
			Want: []Expander{
				createLiteral("echo"),
				createLiteral("foobar"),
			},
		},
		{
			Input: "echo pre-'foobar'-post",
			Want: []Expander{
				createLiteral("echo"),
				createList(
					createLiteral("pre-"),
					createLiteral("foobar"),
					createLiteral("-post"),
				),
			},
		},
		{
			Input: "$foobar",
			Want: []Expander{
				createVariable("foobar"),
			},
		},
		{
			Input: "echo $foobar",
			Want: []Expander{
				createLiteral("echo"),
				createVariable("foobar"),
			},
		},
		{
			Input: "echo \"$foobar\"",
			Want: []Expander{
				createLiteral("echo"),
				createVariable("foobar"),
			},
		},
	}
	testExpandCase(t, data)
}

func testExpansion(t *testing.T) {
	data := []expandCase{
		{
			Input: "${foobar}",
			Want: []Expander{
				createVariable("foobar"),
			},
		},
		{
			Input: "${#foobar}",
			Want: []Expander{
				createLength(createVariable("foobar")),
			},
		},
		{
			Input: "${foobar%suffix}",
			Want: []Expander{
				createSuffix(createVariable("foobar"), "suffix", false),
			},
		},
		{
			Input: "${foobar:1:3}",
			Want: []Expander{
				createSubstring(createVariable("foobar"), 1, 3),
			},
		},
		{
			Input: "${foobar:1}",
			Want: []Expander{
				createSubstring(createVariable("foobar"), 1, 0),
			},
		},
		{
			Input: "${foobar//foo/bar}",
			Want: []Expander{
				createReplaceAll(createVariable("foobar"), "foo", "bar"),
			},
		},
		{
			Input: "${foobar/#foo/bar}",
			Want: []Expander{
				createReplacePrefix(createVariable("foobar"), "foo", "bar"),
			},
		},
	}
	testExpandCase(t, data)
}

func testExpandCase(t *testing.T, data []expandCase) {
	t.Helper()
	primitive := cmp.Exporter(func(t reflect.Type) bool {
		k := t.Kind()
		return k == reflect.String || k == reflect.Bool || k == reflect.Struct
	})
	for _, d := range data {
		got, err := Parse(strings.NewReader(d.Input))
		if err != nil {
			t.Errorf("%s: unexpected error: %s", d.Input, err)
			continue
		}
		if len(got) != len(d.Want) {
			t.Errorf("count mismatched! want %d, got %d", len(d.Want), len(got))
			continue
		}
		for i := range d.Want {
			if !cmp.Equal(d.Want[i], got[i], primitive) {
				t.Errorf("%s: expander mismatched! want %v, got %v", d.Input, d.Want[i], got[i])
				break
			}
		}
	}
}
