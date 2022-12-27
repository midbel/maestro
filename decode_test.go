package maestro_test

import (
	"os"
	"testing"

	"github.com/midbel/maestro"
)

func TestDecode(t *testing.T) {
	t.Run("file", testDecodeFile)
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
