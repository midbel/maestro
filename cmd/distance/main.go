package main

import (
	"flag"
	"fmt"

	"github.com/midbel/maestro/distance"
)

func main() {
	flag.Parse()

	var (
		str  = flag.Arg(0)
		args []string
	)
	for i := 1; i < flag.NArg(); i++ {
		args = append(args, flag.Arg(i))
	}
	for _, sim := range distance.Levenshtein(str, args) {
		fmt.Printf("%s: %s", str, sim)
		fmt.Println()
	}
}
