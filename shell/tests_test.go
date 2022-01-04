package shell_test

import (
	"testing"

	"github.com/midbel/maestro/shell"
)

func TestTester(t *testing.T) {
	data := []struct {
		Name  string
		Input string
		Want  bool
	}{
		{
			Name:  "not empty",
			Input: `[[ -z str ]]`,
			Want:  true,
		},
		{
			Name:  "empty(1)",
			Input: `[[ -n str ]]`,
			Want:  false,
		},
		{
			Name:  "empty(2)",
			Input: `[[ -n "" ]]`,
			Want:  true,
		},
		{
			Name:  "file-exists",
			Input: `[[ -e tests_test.go && -w tests_test.go ]]`,
			Want:  true,
		},
		{
			Name:  "file-regular",
			Input: `[[ -f tests_test.go && -r tests_test.go ]]`,
			Want:  true,
		},
		{
			Name:  "file-directory",
			Input: `[[ -d tests_test.go ]]`,
			Want:  false,
		},
		{
			Name:  "file-directory(not)",
			Input: `[[ ! -d tests_test.go && ! -x tests_test.go ]]`,
			Want:  true,
		},
		{
			Name:  "file-directory",
			Input: `[[ -d testdata ]]`,
			Want:  true,
		},
		{
			Name:  "file-size",
			Input: `[[ -s tests_test.go ]]`,
			Want:  true,
		},
		{
			Name:  "file-size(not)",
			Input: `[[ ! -s tests_test.go ]]`,
			Want:  false,
		},
		{
			Name:  "same-file",
			Input: `[[ tests_test.go -eq tests_test.go ]]`,
			Want:  true,
		},
	}
	env := shell.EmptyEnv()
	for _, d := range data {
		t.Run(d.Name, func(t *testing.T) {
			ex, err := shell.Parse(d.Input)
			if err != nil {
				t.Fatalf("%s: fail to parse: %s", d.Input, err)
			}
			tester, ok := ex.(shell.Tester)
			if !ok {
				t.Fatalf("%s: parsing give unexpected type %T", d.Input, ex)
			}
			got, err := tester.Test(env)
			if err != nil {
				t.Fatalf("unexpected error testing %s: %s", d.Input, err)
			}
			if d.Want != got {
				t.Fatalf("%s: results mismatched! want %t, got %t", d.Input, d.Want, got)
			}
		})
	}
}
