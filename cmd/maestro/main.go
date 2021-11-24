package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/midbel/maestro"
)

func main() {
	flag.Usage = func() {

	}
	var (
		dry  = flag.Bool("d", false, "run dry")
		file = flag.String("f", "maestro.mf", "maestro file to use")
	)
	flag.Parse()

	mst, err := maestro.Load(*file)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	switch cmd, args := arguments(); cmd {
	case "help":
		err = mst.ExecuteHelp(flag.Arg(1))
	case "version":
		err = mst.ExecuteVersion()
	case "all":
		err = mst.ExecuteAll(args)
	case "default":
		err = mst.ExecuteDefault(args)
	default:
		if *dry {
			err = mst.Dry(cmd, args)
		} else {
			err = mst.Execute(cmd, args)
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
