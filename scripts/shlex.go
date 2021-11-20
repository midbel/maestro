package main

import (
	"fmt"
	"strings"
)

func main() {
	list := []string{
		`maestro -f "maestro.mf" all $file foo-"test"-bar # comment`,
		`echo "string with a $variable in middle"`,
		`echo test-$variable-test`,
		`echo ${var}`,
		// `echo ${var:-val}`,
		// `echo ${var:=val}`,
		// `echo ${var:+val}`,
		// `echo ${var:?message}`,
		`echo ${#length}`,
		`echo ${var/from/to}`,
		`echo ${var//from/to}`,
		`echo ${var/%replace/suffix}`,
		`echo ${var/#replace/prefix}`,
		`echo ${var:0:2}`,
		`echo ${var%suffix}`,
		`echo ${var#prefix}`,
		`echo ${var%%long-suffix}`,
		`echo ${var##long-prefix}`,
		`echo ${lower-first,}`,
		`echo ${lower-all,,}`,
		`echo ${upper-first^}`,
		`echo ${upper-all^^}`,
		`echo first && echo second`,
		`echo first || echo second`,
		`echo first | echo -`,
		`echo | echo; echo && echo || echo; echo`,
		// `echo < file.txt`,
		// `echo > file.txt`,
		// `echo >> file.txt`,
	}

	env := EmptyEnv()
	env.Define("file", []string{"shlex"})
	env.Define("variable", []string{"foobar"})
	env.Define("var", []string{"a variable"})
	for i := range list {
		if i > 0 {
			fmt.Println("---")
		}
		scan := Scan(strings.NewReader(list[i]))
		for {
			tok := scan.Scan()
			fmt.Println(tok)
			if tok.Type == EOF || tok.Type == Invalid {
				break
			}
		}
	}
	fmt.Println("===")
	fmt.Println("===")
	for i := range list {
		if i > 0 {
			fmt.Println("---")
		}
		p := NewParser(strings.NewReader(list[i]))
		ex, err := p.Parse()
		if err != nil {
			fmt.Println(list[i], err)
			continue
		}
		fmt.Println(">>", list[i])
		fmt.Printf("%#v\n", ex)
		fmt.Println("result: ", ex.Execute(env))
	}
}
