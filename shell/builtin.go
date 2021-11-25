package shell

import (
	"flag"
	"io"
	"strings"
)

var builtins = map[string]Builtin{
	"help": {},
	"builtins": {
		Usage:   "builtins",
		Short:   "display a list of supported builtins",
		Help:    "",
		Execute: runBuiltins,
	},
	"true":  {},
	"false": {},
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
	"alias":   {},
	"unalias": {},
	"pwd":     {},
	"cd":      {},
	"popd":    {},
	"pushd":   {},
	"dirs":    {},
	"chroot":  {},
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
}

type Builtin struct {
	Usage   string
	Enabled bool
	Help    string
	Execute func(Builtin, []string) error

	shell *Shell
}

func (b Builtin) Name() string {
	i := strings.Index(b.Usage, " ")
	if i <= 0 {
		return b.Usage
	}
	return b.Usage[:i]
}

func (b Builtin) IsEnabled() bool {
	return b.Enabled && b.Execute != nil
}

func (b *Builtin) Start() error {
	return nil
}

func (b *Builtin) Wait() error {
	return nil
}

func (b *Builtin) Run() error {
	return nil
}

func (b *Builtin) StdoutPipe() (io.ReadCloser, error) {
	return nil, nil
}

func (b *Builtin) StderrPipe() (io.ReadCloser, error) {
	return nil, nil
}

func runBuiltins(b Builtin, args []string) error {
	var set flag.FlagSet
	if err := set.Parse(args); err != nil {
		return err
	}
	return nil
}

func runBuiltin(b Builtin, args []string) error {
	var set flag.FlagSet
	if err := set.Parse(args); err != nil {
		return err
	}
	return nil
}

func runCommand(b Builtin, args []string) error {
	var set flag.FlagSet
	if err := set.Parse(args); err != nil {
		return err
	}
	return nil
}

func runScript(b Builtin, args []string) error {
	var set flag.FlagSet
	if err := set.Parse(args); err != nil {
		return err
	}
	return nil
}

func runType(b Builtin, args []string) error {
	var set flag.FlagSet
	if err := set.Parse(args); err != nil {
		return err
	}
	return nil
}

func runSeq(b Builtin, args []string) error {
	var set flag.FlagSet
	if err := set.Parse(args); err != nil {
		return err
	}
	return nil
}

func runEnable(b Builtin, args []string) error {
	var set flag.FlagSet
	if err := set.Parse(args); err != nil {
		return err
	}
	return nil
}

func runReadOnly(b Builtin, args []string) error {
	var set flag.FlagSet
	if err := set.Parse(args); err != nil {
		return err
	}
	return nil
}

func runExport(b Builtin, args []string) error {
	var set flag.FlagSet
	if err := set.Parse(args); err != nil {
		return err
	}
	return nil
}

func runEnv(b Builtin, args []string) error {
	var set flag.FlagSet
	if err := set.Parse(args); err != nil {
		return err
	}
	return nil
}
