package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/midbel/maestro"
)

var (
	CmdVersion = "0.2.0"
	CmdBuild   = ""
	CmdHash    = ""
)

const MaestroEnv = "MAESTRO_FILE"

const help = `usage: maestro [options] [<command> [options] [<arguments>]]

maestro helps to organize all the tasks and/or commands that need to be
performed regularly in a project whatever its nature. It could be the
development of a program, administration of a single server or a set of
virtual machines,...

To do that, maestro needs only a single file, by default called maestro.mf,
and make all the commands available in the file as sub commands of itself.

Moreover, to make the file and its commands easier to use, maestro creates
a help message for the input maestro file and foreach of commands defined
in it.

maestro makes available some default sub commands:

default:  same as calling maestro without arguments, it will call the command
          configured with the meta DEFAULT
all:      call all the commands defined in the meta ALL in order
help:     without arguments, maestro will print a help message generated from
          the information in the maestro file. Otherwise print help of the
				  command
version:  print the version of the maestro file defined via the meta VERSION
          and exit
listen:   run a HTTP server and execute command from the name available in the
          last element of the URL
schedule: run commands that have a schedule property set properly at the given
          interval of time

Options:

  -D NAME[=VALUE], --define NAME[=VALUE]  define NAME with optional value
  -f FILE, --file FILE                    read FILE as a maestro file
  -i, --ignore                            ignore all errors from command
  -k, --skip                              don't execute command's dependencies
  -p, --with-prefix                       prefix each output line with the name of the command
  -r, --remote                            execute commands on remote server
  -t, --trace                             add tracing information with command execution
  -v, --version                           print maestro version and exit
`

func main() {
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, help)
		os.Exit(2)
	}
	var (
		file    = maestro.DefaultFile
		mst     = maestro.New()
		version bool
	)
	if str, ok := os.LookupEnv(MaestroEnv); ok && str != "" {
		file = str
	}

	options := []Option{
		// {Short: "I", Long: "includes", Desc: "search include files in directories", Ptr: &mst.Includes},
		{Short: "d", Long: "dry", Desc: "only print commands that will be executed", Ptr: &mst.MetaExec.Dry},
		{Short: "i", Long: "ignore", Desc: "ignore errors from command", Ptr: &mst.MetaExec.Ignore},
		{Short: "f", Long: "file", Desc: "read file as maestro file", Ptr: &file},
		{Short: "k", Long: "skip", Desc: "skip command dependencies", Ptr: &mst.NoDeps},
		{Short: "r", Long: "remote", Desc: "execute command on remote server(s)", Ptr: &mst.Remote},
		{Short: "t", Long: "trace", Desc: "add tracing information command execution", Ptr: &mst.MetaExec.Trace},
		{Short: "v", Long: "version", Desc: "print maestro version and exit", Ptr: &version},
		// {Short: "D", Long: "define", Desc: "set variables", Ptr: &mst.Locals},
		{Short: "p", Long: "with-prefix", Desc: "add a prefix to each output line", Ptr: &mst.WithPrefix},
	}

	parseArgs(options)

	if version {
		fmt.Printf("maestro %s (build date: %s)", CmdVersion, CmdBuild)
		fmt.Println()
		return
	}

	err := mst.Load(file)
	if err != nil {
		exit(err, file)
	}
	switch cmd, args := arguments(); cmd {
	case maestro.CmdListen, maestro.CmdServe:
		// err = mst.ListenAndServe(args)
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
	case maestro.CmdSchedule:
		// err = mst.Schedule(args)
	case maestro.CmdGraph:
		if len(args) > 0 {
			cmd = args[0]
		}
		err = mst.ShowGraph(cmd)
	default:
		err = mst.Execute(cmd, args)
	}
	exit(err, file)
}

func exit(err error, file string) {
	if err == nil {
		return
	}
	switch err := err.(type) {
	// case maestro.UnexpectedError:
	// 	printUnexpected(err, file)
	case maestro.SuggestionError:
		printSuggestion(err)
	default:
		fmt.Fprintln(os.Stderr, err)
	}
	os.Exit(1)
}

func printSuggestion(err maestro.SuggestionError) {
	sort.Strings(err.Others)
	fmt.Fprintln(os.Stderr, err)
	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "similar command(s): %s", strings.Join(err.Others, ", "))
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "see maestro help to get the list of commands")
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

type Option struct {
	Short string
	Long  string
	Desc  string
	Ptr   interface{}
}

func parseArgs(options []Option) {
	for _, o := range options {
		switch v := o.Ptr.(type) {
		case flag.Value:
			if o.Short != "" {
				flag.Var(v, o.Short, o.Desc)
			}
			if o.Long != "" {
				flag.Var(v, o.Long, o.Desc)
			}
		case *string:
			if o.Short != "" {
				flag.StringVar(v, o.Short, *v, o.Desc)
			}
			if o.Long != "" {
				flag.StringVar(v, o.Long, *v, o.Desc)
			}
		case *bool:
			if o.Short != "" {
				flag.BoolVar(v, o.Short, *v, o.Desc)
			}
			if o.Long != "" {
				flag.BoolVar(v, o.Long, *v, o.Desc)
			}
		default:
		}
	}
	flag.Parse()
}
