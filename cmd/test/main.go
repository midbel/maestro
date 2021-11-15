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

	_, err = maestro.Decode(r)
	fmt.Println(err)
	// s, err := maestro.Scan(r)
	// if err != nil {
	// 	fmt.Fprintln(os.Stderr, err)
	// 	os.Exit(2)
	// }
	//
	// for {
	// 	tok := s.Scan()
	// 	fmt.Println(tok)
	// 	if tok.IsEOF() || tok.IsInvalid() {
	// 		break
	// 	}
	// }
}
