package maestro

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/midbel/maestro/internal/env"
	"github.com/midbel/maestro/internal/expand"
	"github.com/midbel/slices"
	"github.com/midbel/try"
	"golang.org/x/crypto/ssh"
	"golang.org/x/sync/errgroup"
)

type Executer interface {
	Execute(context.Context, []string, io.Writer, io.Writer) error
}

type local struct {
	deps []Executer

	name    string
	env     []string
	workdir string
	scripts []CommandScript
	ctx     *env.Context
}

func (c local) Execute(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	err := c.ctx.ParseArgs(args)
	if err != nil {
		return err
	}
	for _, d := range c.deps {
		if err = d.Execute(ctx, nil, stdout, stderr); err != nil {
			return err
		}
	}
	outr, outw := io.Pipe()
	errr, errw := io.Pipe()
	defer func() {
		outr.Close()
		outw.Close()
		errr.Close()
		errw.Close()
	}()
	go writeLines(c.name, stdout, outr)
	go writeLines(c.name, stderr, errr)
	for _, line := range c.scripts {
		if err = c.execute(ctx, line.Line, args, outw, errw); err != nil {
			break
		}
	}
	return err
}

func (c local) execute(ctx context.Context, line string, args []string, stdout, stderr io.Writer) error {
	parts, err := expand.ExpandString(line, c.ctx)
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, slices.Fst(parts), slices.Rest(parts)...)
	cmd.Dir = c.workdir
	cmd.Env = c.env
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	return cmd.Run()
}

type remote struct {
	name    string
	host    string
	scripts []CommandScript
	config  *ssh.ClientConfig
	ctx     *env.Context
}

func (c remote) Execute(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	conn, err := ssh.Dial("tcp", c.host, c.config)
	if err != nil {
		return err
	}
	defer conn.Close()

	exec := func(line string, outw, errw io.Writer) error {
		parts, err := expand.ExpandString(line, c.ctx)
		if err != nil {
			return err
		}
		sess, err := conn.NewSession()
		if err != nil {
			return err
		}
		defer sess.Close()
		sess.Stdout = outw
		sess.Stderr = errw

		return sess.Run(strings.Join(parts, " "))
	}

	prefix := fmt.Sprintf("%s(%s)", c.name, c.host)
	outr, outw := io.Pipe()
	errr, errw := io.Pipe()
	defer func() {
		outr.Close()
		outw.Close()
		errr.Close()
		errw.Close()
	}()
	go writeLines(prefix, stdout, outr)
	go writeLines(prefix, stderr, errr)

	for _, line := range c.scripts {
		if err := exec(line.Line, outw, errw); err != nil {
			return err
		}
	}
	return nil
}

type execset struct {
	parallel int
	list     []Executer
}

func (c execset) Execute(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	grp, ctx := errgroup.WithContext(ctx)
	if c.parallel > 0 {
		grp.SetLimit(c.parallel)
	}
	for i := range c.list {
		e := c.list[i]
		grp.Go(func() error {
			return e.Execute(ctx, args, stdout, stderr)
		})
	}
	return grp.Wait()
}

type retry struct {
	limit int
	Executer
}

func Retry(limit int64, exec Executer) Executer {
	if limit <= 1 {
		return exec
	}
	return retry{
		limit:    int(limit),
		Executer: exec,
	}
}

func (c retry) Execute(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	return try.TryContext(ctx, c.limit, func(_ int) error {
		return c.Executer.Execute(ctx, args, stdout, stderr)
	})
}

type tracer struct {
	name string
	Executer
}

func Trace(exec Executer) Executer {
	t := tracer{
		name:     "tracer",
		Executer: exec,
	}
	if c, ok := exec.(local); ok {
		t.name = c.name
	}
	return t
}

func (c tracer) Execute(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	fmt.Fprintf(stderr, "[%s] start command", c.name)
	fmt.Fprintln(stderr)
	var (
		now = time.Now()
		err = c.Executer.Execute(ctx, args, stdout, stderr)
	)
	fmt.Fprintf(stderr, "[%s] command done in %s", c.name, time.Since(now))
	fmt.Fprintln(stderr)
	if err != nil {
		fmt.Fprintf(stderr, "[%s] execution failed: %s", c.name, err)
		fmt.Fprintln(stderr)
	}
	return err
}

func writeLines(name string, w io.Writer, r io.Reader) {
	scan := bufio.NewScanner(r)
	for scan.Scan() {
		fmt.Fprintf(w, "[%s] %s", name, scan.Text())
		fmt.Fprintln(w)
	}
}
