package distance_test

import (
	"fmt"
	"testing"

	"github.com/midbel/maestro/distance"
)

func TestGetLevenshteinDistance(t *testing.T) {
	data := []struct {
		First  string
		Second string
		Want   int
	}{
		{
			First:  "kitten",
			Second: "kitten",
		},
		{
			First:  "kitten",
			Second: "sitting",
			Want:   3,
		},
		{
			First:  "",
			Second: "abc",
			Want:   3,
		},
		{
			First:  "abc",
			Second: "",
			Want:   3,
		},
	}
	for _, d := range data {
		t.Run(fmt.Sprintf("%s-%s", d.First, d.Second), func(t *testing.T) {
			got := distance.GetLevenshteinDistance(d.First, d.Second)
			if got < 0 {
				t.Fatalf("computed distance is < 0! it is impossible")
			}
			if d.Want != got {
				t.Fatalf("distance mismatched! want %d, got %d", d.Want, got)
			}
		})
	}
}
