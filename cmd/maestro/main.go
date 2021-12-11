package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/midbel/maestro"
)

var CmdVersion = "0.1.0"

const MaestroEnv = "MAESTRO_FILE"

const help = `usage: maestro [options] [<command> [options] [<arguments>]]

Options:

  -d, --dry                 dry run
  -e, --echo                echo
  -f FILE, --file FILE      read FILE as a maestro file
  -i, --ignore              ignore all errors from command
  -I DIR, --includes DIR    search DIR for included maestro files
  -k, --skip-dep            skip dependencies
  -r, --remote              execute commands on remote server
  -v, --version             print maestro version and exit

Predefined commands:

default
all
help
version
`

func main() {
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, help)
		os.Exit(2)
	}
	var (
		file = maestro.DefaultFile
		mst  = maestro.New()
		version bool
	)
	if str, ok := os.LookupEnv(MaestroEnv); ok && str != "" {
		file = str
	}

	flag.Var(&mst.Includes, "I", "include directories")
	flag.BoolVar(&mst.MetaExec.Dry, "d", false, "run dry")
	flag.BoolVar(&mst.MetaExec.Ignore, "i", false, "ignore errors from command")
	flag.StringVar(&file, "f", file, "maestro file to use")
	flag.BoolVar(&mst.MetaExec.Echo, "e", false, "echo")
	flag.BoolVar(&mst.NoDeps, "k", false, "skip dependencies")
	flag.BoolVar(&mst.Remote, "r", false, "remote")
	flag.StringVar(&mst.MetaHttp.Addr, "a", mst.MetaHttp.Addr, "address")
	flag.BoolVar(&version, "v", false, "print maestro version and exit")
	flag.Parse()

	if version {
		fmt.Printf("maestro %s", CmdVersion)
		fmt.Println()
		return
	}

	err := mst.Load(file)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	switch cmd, args := arguments(); cmd {
	case maestro.CmdListen, maestro.CmdServe:
		err = mst.ListenAndServe()
	case maestro.CmdHelp:
		if cmd = ""; len(args) > 0 {
			cmd = args[0]
		}
		err = mst.ExecuteHelp(cmd)
	case maestro.CmdVersion:
		err = mst.ExecuteVersion()
	case maestro.CmdAll:
		err = mst.ExecuteAll(args)
	case maestro.CmdDefault:
		err = mst.ExecuteDefault(args)
	default:
		err = mst.Execute(cmd, args)
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
