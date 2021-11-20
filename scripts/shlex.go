package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

func main() {
	list := []string{
		`maestro -f "maestro.mf" all $file foo-"test"-bar # comment`,
		`ident = foo bar $variable`,
		`echo "$ident"`,
		`echo "string with a $variable in middle"`,
		`echo test-$variable-test`,
		`echo ${var}`,
		`echo ${var:-val}`,
		`echo ${var:=val}`,
		`echo ${var:+val}`,
		`echo ${var:?message}`,
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
		`echo ${lowerfirst} "=>" ${lowerfirst,}`,
		`echo ${lowerall} "=>" ${lowerall,,}`,
		`echo ${upperfirst} "=>" ${upperfirst^}`,
		`echo ${upperall} "=>" ${upperall^^}`,
		`echo first && echo second`,
		`echo first || echo second`,
		`echo first | echo - | echo`,
		`echo | echo; echo && echo || echo; echo`,
		// `echo < file.txt`,
		// `echo > file.txt`,
		// `echo >> file.txt`,
	}

	// for i := range list {
	// 	if i > 0 {
	// 		fmt.Println("---")
	// 	}
	// 	scan := Scan(strings.NewReader(list[i]))
	// 	for {
	// 		tok := scan.Scan()
	// 		fmt.Println(tok)
	// 		if tok.Type == EOF || tok.Type == Invalid {
	// 			break
	// 		}
	// 	}
	// }
	// fmt.Println("===")
	// fmt.Println("===")
	env := EmptyEnv()
	env.Define("file", []string{"shlex"})
	env.Define("variable", []string{"foobar"})
	env.Define("var", []string{"a variable"})
	env.Define("upperfirst", []string{"upper-First"})
	env.Define("upperall", []string{"upper-All"})
	env.Define("lowerfirst", []string{"Lower-First"})
	env.Define("lowerall", []string{"Lower-All"})
	for i := range list {
		if i > 0 {
			fmt.Println("---")
		}
		fmt.Println(">>", list[i])
		p := NewParser(strings.NewReader(list[i]))
		for {
			ex, err := p.Parse()
			if err != nil {
				if !errors.Is(err, io.EOF) {
					fmt.Fprintln(os.Stderr, "error while parsing", err)
				}
				break
			}
			if err := Execute(ex, env); err != nil {
				fmt.Fprintln(os.Stderr, "execute:", err)
			}
		}
	}
}

func Execute(ex Executer, env Environment) error {
	var err error
	switch ex := ex.(type) {
	case ExecSimple:
		fmt.Println("simple")
		err = execute(ex.Expander, env)
	case ExecAssign:
		fmt.Println("assignment", ex.Ident)
		str, err := ex.Expand(env)
		if err != nil {
			return err
		}
		env.Define(ex.Ident, str)
		err = execute(ex.Expander, env)
	case ExecAnd:
		fmt.Println("and")
		if err = Execute(ex.Left, env); err != nil {
			break
		}
		err = Execute(ex.Right, env)
	case ExecOr:
		fmt.Println("or")
		if err = Execute(ex.Left, env); err == nil {
			break
		}
		err = Execute(ex.Right, env)
	case ExecPipe:
		fmt.Println("pipe")
		for i := range ex.List {
			Execute(ex.List[i], env)
		}
	default:
		err = fmt.Errorf("unsupported executer type %s", ex)
	}
	return err
}

func execute(ex Expander, env Environment) error {
	str, err := ex.Expand(env)
	if err == nil {
		fmt.Printf("%d: %q\n", len(str), str)
	}
	return err
}
