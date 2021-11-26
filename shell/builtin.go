package shell

import (
	"flag"
	"fmt"
	"io"
	"os"
	"plugin"
	"strconv"
	"strings"
)

var builtins = map[string]Builtin{
	"help": {
		Usage:   "help",
		Short:   "display information about a builtin command",
		Help:    "",
		Execute: runHelp,
	},
	"builtins": {
		Usage:   "builtins",
		Short:   "display a list of supported builtins",
		Help:    "",
		Execute: runBuiltins,
	},
	"true":  {
		Usage: "true",
		Short: "always return a successfull result",
		Help: "",
		Execute: runTrue,
	},
	"false":  {
		Usage: "false",
		Short: "always return an unsuccessfull result",
		Help: "",
		Execute: runFalse,
	},
	"builtin": {
		Usage:   "builtin",
		Short:   "execute a simple builtin or display information about builtins",
		Help:    "",
		Execute: runBuiltin,
	},
	"command": {
		Usage:   "command",
		Short:   "execute a simple command or display information about commands",
		Help:    "",
		Execute: runCommand,
	},
	"script": {
		Usage:   "script",
		Short:   "execute a simple script or display information about scripts",
		Help:    "",
		Execute: runScript,
	},
	"seq": {
		Usage:   "seq",
		Short:   "print a sequence of number to stdout",
		Help:    "",
		Execute: runSeq,
	},
	"type": {
		Usage:   "type",
		Short:   "display information about command type",
		Help:    "",
		Execute: runType,
	},
	"env": {
		Usage:   "env",
		Short:   "display list of variables exported to environment of commands to be executed",
		Help:    "",
		Execute: runEnv,
	},
	"enable": {
		Usage:   "enable",
		Short:   "enable and disable builtins",
		Help:    "",
		Execute: runEnable,
	},
	"alias":   {
		Usage: "alias",
		Short: "",
		Help: "",
	},
	"unalias": {
		Usage: "unalias",
		Short: "",
		Help: "",
	},
	"cd":      {
		Usage: "cd",
		Short: "change the shell working directory",
		Help: "",
		Execute: runChdir,
	},
	"pwd": {
		Usage: "pwd",
		Short: "print the name of the current shell working directory",
		Help: "",
		Execute: runPwd,
	},
	"popd":    {
		Usage: "popd",
		Short: "",
		Help: "",
	},
	"pushd":   {
		Usage: "pushd",
		Short: "",
		Help: "",
	},
	"dirs":    {
		Usage: "dirs",
		Short: "",
		Help: "",
	},
	"chroot":  {
		Usage: "chroot",
		Short: "",
		Help: "",
	},
	"readonly": {
		Usage:   "readonly",
		Short:   "mark and unmark shell variables as readonly",
		Help:    "",
		Execute: runReadOnly,
	},
	"export": {
		Usage:   "export",
		Short:   "mark variables to export in environment of commands to be executed",
		Help:    "",
		Execute: runExport,
	},
	"exit": {
		Usage:   "exit",
		Short:   "exit the shell",
		Help:    "",
		Execute: runExit,
	},
}

type Builtin struct {
	Usage    string
	Short    string
	Help     string
	Disabled bool
	Execute  func(Builtin) error

	args     []string
	shell    *Shell
	finished bool
	done     chan error

	stdout io.Writer
	stderr io.Writer
	stdin  io.Reader

	closes []io.Closer
}

func (b *Builtin) Start() error {
	if !b.IsEnabled() {
		return fmt.Errorf("builtin is disabled")
	}
	if b.finished {
		return fmt.Errorf("builtin already executed")
	}
	setupfd := []func() error{
		b.setStdin,
		b.setStdout,
		b.setStderr,
	}
	for _, set := range setupfd {
		err := set()
		if err != nil {
			b.closeDescriptors()
			return err
		}
	}
	b.done = make(chan error, 1)
	go func() {
		b.done <- b.Execute(*b)
	}()
	return nil
}

func (b *Builtin) Wait() error {
	if !b.IsEnabled() {
		return fmt.Errorf("builtin is disabled")
	}
	if b.finished {
		return fmt.Errorf("builtin already finished")
	}
	defer func() {
		close(b.done)
		b.closeDescriptors()
	}()
	b.finished = true
	return <-b.done
}

func (b *Builtin) Run() error {
	if err := b.Start(); err != nil {
		return err
	}
	return b.Wait()
}

func (b *Builtin) StdoutPipe() (io.ReadCloser, error) {
	if b.stdout != nil {
		return nil, fmt.Errorf("stdout already set")
	}
	if b.shell != nil {
		return nil, fmt.Errorf("stdout after builtin started")
	}
	pr, pw, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	b.closes = append(b.closes, pr, pw)
	b.stdout = pw
	return pr, nil
}

func (b *Builtin) StderrPipe() (io.ReadCloser, error) {
	if b.stderr != nil {
		return nil, fmt.Errorf("stderr already set")
	}
	if b.shell != nil {
		return nil, fmt.Errorf("stderr after builtin started")
	}
	pr, pw, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	b.closes = append(b.closes, pr, pw)
	b.stderr = pw
	return pr, nil
}

func (b *Builtin) StdinPipe() (io.WriteCloser, error) {
	if b.stdin != nil {
		return nil, fmt.Errorf("stdin already set")
	}
	if b.shell != nil {
		return nil, fmt.Errorf("stdin after builtin started")
	}
	pr, pw, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	b.closes = append(b.closes, pr, pw)
	b.stdin = pr
	return pw, nil
}

func (b Builtin) Name() string {
	i := strings.Index(b.Usage, " ")
	if i <= 0 {
		return b.Usage
	}
	return b.Usage[:i]
}

func (b Builtin) IsEnabled() bool {
	return !b.Disabled && b.Execute != nil
}

func (b *Builtin) setStdin() error {
	if b.stdin != nil {
		return nil
	}
	f, err := os.Open(os.DevNull)
	if err != nil {
		return err
	}
	b.stdin = f
	b.closes = append(b.closes, f)
	return nil
}

func (b *Builtin) setStdout() error {
	if b.stdout != nil {
		return nil
	}
	out, err := b.openFile()
	if err == nil {
		b.stdout = out
	}
	return err
}

func (b *Builtin) setStderr() error {
	if b.stderr != nil {
		return nil
	}
	out, err := b.openFile()
	if err == nil {
		b.stderr = out
	}
	return err
}

func (b *Builtin) openFile() (*os.File, error) {
	f, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		return nil, err
	}
	b.closes = append(b.closes, f)
	return f, nil
}

func (b *Builtin) closeDescriptors() {
	for _, c := range b.closes {
		c.Close()
	}
	b.closes = b.closes[:0]
}

func runTrue(_ Builtin) error {
	return nil
}

func runFalse(_ Builtin) error {
	return Failure
}

func runBuiltins(b Builtin) error {
	var set flag.FlagSet
	if err := set.Parse(b.args); err != nil {
		return err
	}
	for n, i := range b.shell.builtins {
		if i.Name() != "" {
			n = i.Name()
		}
		fmt.Fprintf(b.stdout, "%-12s: %s", n, i.Short)
		fmt.Fprintln(b.stdout)
	}
	return nil
}

func runHelp(b Builtin) error {
	var set flag.FlagSet
	if err := set.Parse(b.args); err != nil {
		return err
	}
	other, ok := b.shell.builtins[set.Arg(0)]
	if !ok {
		fmt.Fprintf(b.stderr, "no help match %s! try builtins to get the list of available builtins", set.Arg(0))
		fmt.Fprintln(b.stderr)
		return nil
	}
	fmt.Fprintln(b.stdout, other.Name())
	fmt.Fprintln(b.stdout, other.Short)
	fmt.Fprintln(b.stdout)
	if len(other.Help) > 0 {
		fmt.Fprintln(b.stdout, other.Help)
	}
	fmt.Fprintln(b.stdout)
	return nil
}

func runBuiltin(b Builtin) error {
	var set flag.FlagSet
	if err := set.Parse(b.args); err != nil {
		return err
	}
	return nil
}

func runCommand(b Builtin) error {
	var set flag.FlagSet
	if err := set.Parse(b.args); err != nil {
		return err
	}
	return nil
}

func runScript(b Builtin) error {
	var set flag.FlagSet
	if err := set.Parse(b.args); err != nil {
		return err
	}
	return nil
}

func runType(b Builtin) error {
	var set flag.FlagSet
	if err := set.Parse(b.args); err != nil {
		return err
	}
	for _, a := range set.Args() {
		var kind string
		if _, ok := b.shell.builtins[a]; ok {
			kind = "builtin"
		} else if _, ok := b.shell.commands[a]; ok {
			kind = "user command"
		} else if _, ok := b.shell.alias[a]; ok {
			kind = "alias"
		} else if vs, err := b.shell.Resolve(a); err == nil && len(vs) > 0 {
			kind = "shell variable"
		} else {
			kind = "command"
		}
		fmt.Fprintf(b.stdout, "%s: %s", a, kind)
		fmt.Fprintln(b.stdout)
	}
	return nil
}

func runSeq(b Builtin) error {
	var (
		set flag.FlagSet
		sep = set.String("s", " ", "print separator between each number")
		fst = 1
		lst = 1
		inc = 1
		err error
	)
	if err := set.Parse(b.args); err != nil {
		return err
	}
	switch set.NArg() {
	case 1:
		if lst, err = strconv.Atoi(set.Arg(0)); err != nil {
			fmt.Fprintf(b.stderr, "%s: invalid number", flag.Arg(0))
			fmt.Fprintln(b.stderr)
		}
	case 2:
		if fst, err = strconv.Atoi(set.Arg(0)); err != nil {
			fmt.Fprintf(b.stderr, "%s: invalid number", flag.Arg(0))
			fmt.Fprintln(b.stderr)
			break
		}
		if lst, err = strconv.Atoi(set.Arg(1)); err != nil {
			fmt.Fprintf(b.stderr, "%s: invalid number", flag.Arg(1))
			fmt.Fprintln(b.stderr)
			break
		}
	case 3:
		if fst, err = strconv.Atoi(set.Arg(0)); err != nil {
			fmt.Fprintf(b.stderr, "%s: invalid number", flag.Arg(0))
			fmt.Fprintln(b.stderr)
			break
		}
		if inc, err = strconv.Atoi(set.Arg(1)); err != nil {
			fmt.Fprintf(b.stderr, "%s: invalid number", flag.Arg(1))
			fmt.Fprintln(b.stderr)
			break
		}
		if lst, err = strconv.Atoi(set.Arg(2)); err != nil {
			fmt.Fprintf(b.stderr, "%s: invalid number", flag.Arg(2))
			fmt.Fprintln(b.stderr)
			break
		}
	default:
		fmt.Fprintf(b.stderr, "seq: missing operand")
		fmt.Fprintln(b.stderr)
		return nil
	}
	if err != nil {
		return nil
	}
	if inc == 0 {
		inc++
	}
	cmp := func(f, t int) bool { return f <= t }
	if fst > lst {
		cmp = func(f, t int) bool { return f >= t }
		if inc > 0 {
			inc = -inc
		}
	}
	for i := 0; cmp(fst, lst); i++ {
		if i > 0 {
			fmt.Fprint(b.stdout, *sep)
		}
		fmt.Fprintf(b.stdout, strconv.Itoa(fst))
		fst += inc
	}
	fmt.Fprintln(b.stdout)
	return nil
}

func runEnable(b Builtin) error {
	var set flag.FlagSet
	var (
		print   = set.Bool("p", false, "print the list of builtins with their status")
		load    = set.Bool("f", false, "load new builtin(s) from list of given object file(s)")
		disable = set.Bool("d", false, "disable builtin(s) given in the list")
	)
	if err := set.Parse(b.args); err != nil {
		return err
	}
	if *load {
		return loadExternalBuiltins(b, set.Args())
	}
	if *print {
		printEnableBuiltins(b)
		return nil
	}
	for _, n := range set.Args() {
		other, ok := b.shell.builtins[n]
		if !ok {
			fmt.Fprintf(b.stderr, "builtin %s not found", n)
			fmt.Fprintln(b.stderr)
			continue
		}
		other.Disabled = *disable
		b.shell.builtins[n] = other
	}
	return nil
}

func loadExternalBuiltins(b Builtin, files []string) error {
	for _, f := range files {
		plug, err := plugin.Open(f)
		if err != nil {
			return err
		}
		sym, err := plug.Lookup("Load")
		if err != nil {
			return err
		}
		load, ok := sym.(func() Builtin)
		if !ok {
			return fmt.Errorf("invalid signature")
		}
		e := load()
		b.shell.builtins[b.Name()] = e
	}
	return nil
}

func printEnableBuiltins(b Builtin) {
	for _, x := range b.shell.builtins {
		state := "enabled"
		if x.Disabled {
			state = "disabled"
		}
		fmt.Fprintf(b.stdout, "%-12s: %s", x.Name(), state)
		fmt.Fprintln(b.stdout)
	}
}

func runReadOnly(b Builtin) error {
	var set flag.FlagSet
	if err := set.Parse(b.args); err != nil {
		return err
	}
	return nil
}

func runExport(b Builtin) error {
	var set flag.FlagSet
	if err := set.Parse(b.args); err != nil {
		return err
	}
	return nil
}

func runEnv(b Builtin) error {
	var set flag.FlagSet
	if err := set.Parse(b.args); err != nil {
		return err
	}
	return nil
}

func runExit(b Builtin) error {
	var set flag.FlagSet
	if err := set.Parse(b.args); err != nil {
		return err
	}
	code := ExitCode(b.shell.context.code)
	if c, err := strconv.Atoi(set.Arg(0)); err == nil {
		code = ExitCode(c)
	}
	if code.Failure() {
		return fmt.Errorf("%w: %s", ErrExit, code)
	}
	return nil
}

func runChdir(b Builtin) error {
	var set flag.FlagSet
	if err := set.Parse(b.args); err != nil {
		return err
	}
	if err := b.shell.Chdir(set.Arg(0)); err != nil {
		fmt.Fprintf(b.stderr, err.Error())
		fmt.Fprintln(b.stderr)
	}
	return nil
}

func runPwd(b Builtin) error {
	var set flag.FlagSet
	if err := set.Parse(b.args); err != nil {
		return err
	}
	fmt.Fprintln(b.stdout, b.shell.cwd)
	return nil
}
