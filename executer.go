package maestro

import (
	"context"
	"io"
	"os/exec"
	"strings"

	"github.com/midbel/shlex"
	"github.com/midbel/slices"
)

type Executer interface {
	Execute(context.Context, []string, io.Writer, io.Writer) error
}

type command struct {
	pre  Executer
	post Executer
	deps []Executer

	env     []string
	workdir string
	scripts CommandScript
}

func (c command) Execute(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if c.pre != nil {
		c.pre.Execute(ctx, nil, stdout, stderr)
	}
	for _, d := range c.deps {
		if err := d.Execute(ctx, nil, stdout, stderr); err != nil {
			return err
		}
	}
	var err error
	for _, line := range c.scripts {
		parts, err := shlex.Split(strings.NewReader(line))
		if err != nil {
			return err
		}
		cmd := exec.CommandContext(ctx, slices.Fst(parts), slices.Rest(parts)...)
		cmd.Env = c.env
		cmd.Stdout = stdout
		cmd.Stderr = stderr

		if err = cmd.Run(); err != nil {
			break
		}
	}
	if c.post != nil {
		c.post.Execute(ctx, nil, stdout, stderr)
	}
	return err
}
