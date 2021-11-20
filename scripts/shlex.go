package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"golang.org/x/sync/errgroup"
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
		`echo foobar | cat`,
		// `echo | echo; echo && echo || echo; echo`,
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
		err = executeAssign(ex, env)
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
		executePipe(ex, env)
	default:
		err = fmt.Errorf("unsupported executer type %s", ex)
	}
	return err
}

func executeAssign(ex ExecAssign, env Environment) error {
	fmt.Println("assignment", ex.Ident)
	str, err := ex.Expand(env)
	if err != nil {
		return err
	}
	return env.Define(ex.Ident, str)
}

func executePipe(ex ExecPipe, env Environment) error {
	fmt.Println("pipe")
	var cs []*exec.Cmd
	for i := range ex.List {
		sex, ok := ex.List[i].(ExecSimple)
		if !ok {
			return fmt.Errorf("single command expected")
		}
		str, err := sex.Expand(env)
		if err != nil || len(str) == 0 {
			return err
		}
		if _, err = exec.LookPath(str[0]); err != nil {
			return err
		}
		cmd := exec.Command(str[0], str[1:]...)
		cmd.Stderr = os.Stderr
		cs = append(cs, cmd)
	}
	var (
		err error
		out io.Reader
		grp errgroup.Group
	)
	for i := 0; i < len(cs)-1; i++ {
		var (
			curr = cs[i]
			next = cs[i+1]
		)
		if out, err = curr.StdoutPipe(); err != nil {
			return err
		}
		next.Stdin = io.TeeReader(out, New(fmt.Sprintf("stdout-%d", i)))
		grp.Go(curr.Run)
	}
	last := cs[len(cs)-1]
	last.Stdout = os.Stdout
	grp.Go(last.Run)
	return grp.Wait()
}

func execute(ex Expander, env Environment) error {
	str, err := ex.Expand(env)
	if err != nil || len(str) == 0 {
		return err
	}
	fmt.Printf("%d: %q\n", len(str), str)
	if _, err := exec.LookPath(str[0]); err != nil {
		return err
	}
	cmd := exec.Command(str[0], str[1:]...)
	cmd.Stdout = New("stdout")
	cmd.Stderr = New("stderr")
	return cmd.Run()
}

type CommandWriter struct {
	prefix string
}

func New(prefix string) io.Writer {
	return &CommandWriter{
		prefix: fmt.Sprintf("[%s] ", prefix),
	}
}

func (w *CommandWriter) Write(b []byte) (int, error) {
	io.WriteString(os.Stdout, w.prefix)
	return os.Stdout.Write(b)
}
