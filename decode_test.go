package maestro_test

import (
	"os"
	"strings"
	"testing"

	"github.com/midbel/maestro"
)

func TestDecode(t *testing.T) {
	t.Run("file", testDecodeFile)
	t.Run("end-of-line", testDecodeEndOfLine)
}

func testDecodeFile(t *testing.T) {
	r, err := os.Open("testdata/sample.mf")
	if err != nil {
		t.Fatalf("fail to open sample file: %s", err)
	}
	defer r.Close()

	_, err = maestro.Decode(r)
	if err != nil {
		t.Fatalf("fail to decode sample file: %s", err)
	}
}

const multiline = `
var = foobar
classic = (
	classic_prop1 = value1, # a comment
	classic_prop2 = value2,
	# comment should be skipped
	classic_nested = (
		classic_sub1 = value1,
		classic_sub2 = value2
	)
)
obj = (
	obj_prop1  = value1
	obj_prop2  = value2 # a comment
	obj_prop3  = 100
	obj_prop4  = false
	obj_nested = (
		obj_subprop1 = subvalue1
		obj_subprop2 = subvalue2
		nested   = (
			obj_las" = $var
		)
	) # a comment
	obj_prop5 = $var
)
action(
	short = "basic action"
	tag   = test demo # a comment
): {
	echo $0
}
`

func testDecodeEndOfLine(t *testing.T) {
	_, err := maestro.Decode(strings.NewReader(multiline))
	if err != nil {
		t.Fatalf("fail to decode multiline object: %s", err)
	}
}
