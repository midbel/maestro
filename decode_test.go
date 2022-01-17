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
	prop1 = value1, # a comment
	prop2 = value2,
	# comment should be skipped
	nested = (
		sub1 = value1,
		sub2 = value2
	)
)
obj = (
	prop1  = value1
	prop2  = value2 # a comment
	prop3  = 100
	prop4  = false
	nested = (
		subprop1 = subvalue1
		subprop2 = subvalue2
		nested   = (
			last = $var
		)
	) # a comment
	prop5 = $var
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
