package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/midbel/maestro/shell"
)

func main() {
	var (
		cwd  = flag.String("c", ".", "set working directory")
		name = flag.String("n", "tish", "script name")
		echo = flag.Bool("e", false, "echo each command before executing")
	)
	flag.Parse()
	if flag.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "no enough argument supplied")
		os.Exit(1)
	}

	options := []shell.ShellOption{
		shell.WithVar("foo", "foo"),
		shell.WithVar("bar", "bar"),
		shell.WithVar("foobar", "foobar"),
		shell.WithCwd(*cwd),
	}
	if *echo {
		options = append(options, shell.WithEcho())
	}

	sh, err := shell.New(options...)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	var args []string
	if flag.NArg() > 1 {
		args = flag.Args()
		args = args[1:]
	}
	if err := sh.Execute(flag.Arg(0), *name, args); err != nil {
		fmt.Fprintf(os.Stderr, "fail to execute command: %s => %s", flag.Arg(0), err)
		fmt.Fprintln(os.Stderr)
	}
}
