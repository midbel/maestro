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
		"echo {}",
		"echo {foo,bar,foobar}",
		"echo prefix-{foo,bar,foobar}",
		"echo {foo,bar,foobar}-suffix",
		"echo prefix-{foo,bar,foobar}-suffix",
		"echo {1..5}",
		"echo {1..5..2}",
		"echo {01..5..2}",
		"echo {5..-1..-2}",
		"echo {A,B,C,D,E}{0..9}",
		"echo pre-{{A,B,C,D,E},_{a,b,c,d,e}_}-suff",
		"echo $(wc -l data/simple.mf)",
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
	env := shell.EmptyEnv()
	env.Define("foobar", []string{"foobar"})
	env.Define("file", []string{"file.txt.gz"})

	sh, _ := shell.New(shell.WithEnv(env))
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
			// fmt.Printf("%#v\n", e)
			s, ok := e.(shell.ExecSimple)
			if !ok {
				continue
			}
			fmt.Println(s.Expand(sh))
		}
	}
}
