package main

import (
	"fmt"
	"strings"

	"github.com/midbel/maestro/shell"
)

func main() {
	lines := []string{
		"echo {}",
		"echo {foo,bar}",
		"echo {1..5}",
		`echo "--build ${maestro,,} in ${bindir,,}/${maestro,,}"`,
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
}
