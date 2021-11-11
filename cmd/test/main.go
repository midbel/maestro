package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/midbel/maestro"
)

func main() {
	flag.Parse()

	r, err := os.Open(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer r.Close()

	s, err := maestro.Scan(r)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for {
		tok := s.Scan()
		if tok.IsEOF() || tok.IsInvalid() {
			break
		}
		fmt.Println(tok)
	}
}
