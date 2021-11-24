package main

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/midbel/maestro/shell"
)

func main() {
	lines := []string{
		"echo foobar",
		"echo foo | pipe && echo bar | pipe",
		"foo=bar",
		"foo=bar; echo foobar # a comment",
		"echo $(cat | grep | cut)",
	}
	for i, str := range lines {
		if i > 0 {
			fmt.Println("---")
		}
		s := shell.Scan(strings.NewReader(str))
		for {
			tok := s.Scan()
			fmt.Println(tok)
			if tok.Type == shell.EOF || tok.Type == shell.Invalid {
				break
			}
		}
	}
	fmt.Println("========")
	fmt.Println("========")
	// env := shell.EmptyEnv()
	// env.Define("foobar", []string{"foobar"})
	// env.Define("file", []string{"file.txt.gz"})
	// sh, _ := shell.New(shell.WithEnv(env))

	for i, str := range lines {
		if i > 0 {
			fmt.Println("---")
		}
		fmt.Println("input:", str)
		p := shell.NewParser(strings.NewReader(str))
		for {
			e, err := p.Parse()
			if err != nil {
				if !errors.Is(err, io.EOF) {
					fmt.Println(err)
				}
				break
			}
			fmt.Printf("%#v\n", e)
			// s, ok := e.(shell.ExecSimple)
			// if !ok {
			// 	continue
			// }
			// fmt.Println(s.Expand(sh))
		}
	}
}
