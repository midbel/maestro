package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/midbel/maestro"
)

const help = "maestro command help"

func main() {
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, help)
		os.Exit(2)
	}
	var (
		skip   = flag.Bool("k", false, "skip dependencies")
		remote = flag.Bool("r", false, "remote")
		dry    = flag.Bool("d", false, "run dry")
		file   = flag.String("f", "maestro.mf", "maestro file to use")
	)
	flag.Parse()

	mst, err := maestro.Load(*file)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	switch cmd, args := arguments(); cmd {
	case "help":
		if cmd = ""; len(args) > 0 {
			cmd = args[0]
		}
		err = mst.ExecuteHelp(cmd)
	case "version":
		err = mst.ExecuteVersion()
	case "all":
		err = mst.ExecuteAll(args, *remote, *skip)
	case "default":
		err = mst.ExecuteDefault(args, *remote, *skip)
	default:
		if *dry {
			err = mst.Dry(cmd, args)
		} else {
			err = mst.Execute(cmd, args, *remote, *skip)
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func arguments() (string, []string) {
	var (
		cmd  = flag.Arg(0)
		args = flag.Args()
	)
	if flag.NArg() >= 1 {
		args = args[1:]
	}
	return cmd, args
}
