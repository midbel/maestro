package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/midbel/maestro"
)

type includes []string

func (i *includes) Set(fs string) error {
	files := *i
	sort.Strings(files)
	for _, f := range strings.Split(fs, ",") {
		ix := sort.SearchStrings(files, f)
		if ix < len(files) && files[ix] == f {
			return fmt.Errorf("%s: already include", filepath.Base(f))
		}
		files = append(files[:ix], append([]string{f}, files[ix:]...)...)
	}
	*i = files
	return nil
}

func (i *includes) String() string {
	files := *i
	if len(files) == 0 {
		return ""
	}
	return strings.Join(files, ",")
}

func main() {
	var incl includes
	flag.Var(&incl, "i", "include files")
	file := flag.String("f", "maestro.mf", "")

	debug := flag.Bool("debug", false, "debug")
	echo := flag.Bool("echo", false, "echo")
	bindir := flag.String("bin", "", "scripts directory")
	nodeps := flag.Bool("nodeps", false, "don't execute command dependencies")
	noskip := flag.Bool("noskip", false, "execute an action even if already executed")
	flag.Parse()

	m, err := maestro.Parse(*file, []string(incl)...)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(123)
	}

	m.Debug = *debug
	m.Nodeps = *nodeps
	m.Noskip = *noskip
	m.Echo = *echo

	switch action, args := flag.Arg(0), arguments(flag.Args()); action {
	case "help", "":
		if act := flag.Arg(1); act == "" {
			err = m.Summary()
		} else {
			err = m.ExecuteHelp(act)
		}
	case "run", "format", "fmt":
		err = fmt.Errorf("%s: action not yet implemented", action)
	case "export":
		err = m.ExecuteExport(*bindir, args)
	case "version":
		err = m.ExecuteVersion()
	case "all":
		err = m.ExecuteAll(args)
	case "default":
		err = m.ExecuteDefault(args)
	default:
		err = m.Execute(action, args)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(122)
	}
}

func arguments(args []string) []string {
	if len(args) >= 1 {
		args = args[1:]
	}
	return args
}
