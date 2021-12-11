package wrap_test

import (
	"bufio"
	"strings"
	"testing"

	"github.com/midbel/maestro/wrap"
)

func TestShorten(t *testing.T) {
	data := []struct {
		Input string
		Want  string
		Len   int
	}{
		{
			Input: "the quick brown fox jumps over the lazy dog",
			Want:  "the quick brown...",
			Len:   15,
		},
		{
			Input: "the quick brown fox jumps over the lazy dog",
			Want:  "the quick brown fox...",
			Len:   18,
		},
		{
			Input: "the quick brown fox jumps over the lazy dog",
			Want: "the quick brown fox jumps over the lazy dog",
			Len:   100,
		},
	}
	for _, d := range data {
		got := wrap.Shorten(d.Input, d.Len)
		if d.Want != got {
			t.Errorf("strings mismatched! want %s, got %s", d.Want, got)
		}
	}
}

func TestWrapN(t *testing.T) {
	data := []struct {
		Input string
		Len   int
	}{
		{
			Input: "the quick brown\n\n\nfox jumps   over the lazy dog",
			Len:   70,
		},
		{
			Input: "the quick brown fox jumps over the lazy dog",
			Len:   15,
		},
		{
			Input: "the quick brown fox jumps over the lazy dog",
			Len:   20,
		},
		{
			Input: "the quick brown fox jumps over the lazy dog",
			Len:   30,
		},
		{
			Input: `simple is a sample maestro file that can be used for any Go project.
It provides commands to automatize building, checking and testing
your Go project.

It has also some commands to give statistics on the status of the
project such as number of remaining todos, line of codes and others.`,
			Len: 70,
		},
	}
	for _, d := range data {
		var (
			got  = wrap.WrapN(d.Input, d.Len)
			scan = bufio.NewScanner(strings.NewReader(got))
		)
		if len(got) == 0 && len(d.Input) > 0 {
			t.Errorf("nothing has been wrapped!")
			continue
		}
		want := d.Len + wrap.DefaultThreshold
		for scan.Scan() {
			str := scan.Text()
			if len(str) > d.Len+wrap.DefaultThreshold {
				t.Errorf("%s: longer than expected! want %d, got %d", str, want, len(str))
				break
			}
			t.Logf("%2d(%d): %s", len(str), d.Len, str)
		}
	}
}
