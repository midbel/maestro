package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/midbel/maestro"
	// "github.com/midbel/maestro/wrap"
)

const MaestroEnv = "MAESTRO_FILE"

const help = `maestro

Options:

  -d
	-f
	-e
	-i
	-k
	-r
	-I

Predefined commands

usage: maestro [options] [<command> [comand options...] [command arguments]]
`

func main() {
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, help)
		os.Exit(2)
	}
	var (
		file = maestro.DefaultFile
		mst  = maestro.New()
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
	flag.Parse()

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
