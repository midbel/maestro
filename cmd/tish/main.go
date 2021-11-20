package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/midbel/maestro/shell"
)

func main() {
	flag.Parse()

	options := []shell.ShellOption{
		shell.WithVar("foo", "foo"),
		shell.WithVar("bar", "bar"),
		shell.WithVar("foobar", "foobar"),
	}

	sh, err := shell.New(options...)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for _, a := range flag.Args() {
		if err := sh.Execute(a); err != nil {
			fmt.Fprintf(os.Stderr, "fail to execute command: %s => %s", a, err)
			fmt.Fprintln(os.Stderr)
		}
	}
}
