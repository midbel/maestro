package shell

import (
	"io"
)

var builtins = map[string]Builtin{
	"help":     {},
	"builtins": {},
	"true":     {},
	"false":    {},
	"builtin":  {},
	"command":  {},
	"seq":      {},
	"type":     {},
	"env":      {},
	"enable":   {},
	"alias":    {},
	"unalias":  {},
	"pwd":      {},
	"cd":       {},
	"popd":     {},
	"pushd":    {},
	"dirs":     {},
	"chroot":   {},
	"readonly": {},
}

type Builtin struct {
	Name    string
	Enabled bool
	Help    string
	Exec    func(args []string) error
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
