package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"github.com/midbel/maestro/shell"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Kill, os.Interrupt)
		<-sig
		cancel()
		close(sig)
	}()
	var (
		cwd  = flag.String("c", ".", "set working directory")
		name = flag.String("n", "tish", "script name")
		echo = flag.Bool("e", false, "echo each command before executing")
		scan = flag.Bool("s", false, "scan script")
	)
	flag.Parse()
	if flag.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "no enough argument supplied")
		os.Exit(1)
	}

	if *scan {
		scanLine(flag.Arg(0))
		return
	}

	options := []shell.ShellOption{
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
	if err := sh.Execute(ctx, flag.Arg(0), *name, args); err != nil {
		fmt.Fprintf(os.Stderr, "fail to execute command: %s => %s", flag.Arg(0), err)
		fmt.Fprintln(os.Stderr)
	}
	sh.Exit()
}

func scanLine(line string) {
	scan := shell.Scan(strings.NewReader(line))
	for i := 1; ; i++ {
		tok := scan.Scan()
		fmt.Fprintf(os.Stdout, "%3d: %s", i, tok)
		fmt.Fprintln(os.Stdout)
		if tok.Type == shell.EOF || tok.Type == shell.Invalid {
			break
		}
	}
}
