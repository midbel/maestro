package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"sort"
	"strings"

	"github.com/midbel/maestro"
)

type includes []string

func (i *includes) Set(fs string) error {
	vs, err := setValues(fs, *i, isFile)
	if err == nil {
		*i = vs
	}
	return err
}

func (i *includes) String() string {
	return toString(*i)
}

type remotes []string

func (r *remotes) Set(hs string) error {
	vs, err := setValues(hs, *r, isHostPort)
	if err == nil {
		*r = vs
	}
	return err
}

func (r *remotes) String() string {
	return toString(*r)
}

func main() {
	var (
		incl  includes
		hosts remotes
	)
	flag.Var(&incl, "i", "include files")
	flag.Var(&hosts, "r", "remote hosts")

	file := flag.String("f", "maestro.mf", "")
	eta := flag.Bool("eta", false, "eta")
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

	m.Nodeps = *nodeps
	m.Noskip = *noskip
	m.Echo = *echo
	m.Eta = *eta
	m.Hosts = append(m.Hosts, []string(hosts)...)

	switch action, args := flag.Arg(0), arguments(flag.Args()); action {
	case "help", "":
		if act := flag.Arg(1); act == "" {
			err = m.Summary()
		} else {
			err = m.ExecuteHelp(act)
		}
	case "run", "format", "fmt":
		err = fmt.Errorf("%s: action not yet implemented", action)
	case "cat", "debug":
		err = m.ExecuteCat(args)
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

func setValues(str string, set []string, fn func(string) error) ([]string, error) {
	if fn == nil {
		fn = func(_ string) error { return nil }
	}
	sort.Strings(set)
	for _, v := range strings.Split(str, ",") {
		ix := sort.SearchStrings(set, v)
		if ix < len(set) && set[ix] == v {
			continue
		}
		if err := fn(v); err != nil {
			return nil, err
		}
		set = append(set[:ix], append([]string{v}, set[ix:]...)...)
	}
	return set, nil
}

func toString(vs []string) string {
	if len(vs) == 0 {
		return ""
	}
	return strings.Join(vs, ", ")
}

func isHostPort(str string) error {
	_, _, err := net.SplitHostPort(str)
	return err
}

func isFile(str string) error {
	i, err := os.Stat(str)
	if err == nil {
		if !i.Mode().IsRegular() {
			err = fmt.Errorf("%s: not a regular file")
		}
	}
	return err
}
