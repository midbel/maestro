package maestro

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"golang.org/x/sync/errgroup"
)

type executer interface {
	Execute(context.Context, io.Writer, io.Writer) error
}

type ctree struct {
	root execmain

	prefix string

	stdout *pipe
	stderr *pipe
}

func createTree(root execmain) (ctree, error) {
	var (
		tree ctree
		err  error
	)
	if tree.stdout, err = createPipe(); err != nil {
		return tree, err
	}
	if tree.stderr, err = createPipe(); err != nil {
		return tree, err
	}
	tree.root = root
	return tree, nil
}

func (c ctree) Execute(ctx context.Context, stdout, stderr io.Writer) error {
	if c.prefix {
		stdout = createPrefix("", stdout)
		stderr = createPrefix("", stderr)
	}

	go io.Copy(stdout, createLine(c.stdout.R))
	go io.Copy(stderr, createLine(c.stderr.R))

	return c.root.Execute(ctx, c.stdout.W, c.stderr.W)
}

type execmain struct {
	Command
	args []string

	list deplist

	ignore bool

	pre     []Command
	post    []Command
	success []Command
	errors  []Command
}

func createMain(cmd Command, args []string, list deplist) execmain {
	return execmain{
		Command: cmd,
		args:    args,
		list:    list,
	}
}

func (e execmain) Execute(ctx context.Context, stdout, stderr io.Writer) error {
	e.executeList(ctx, e.pre, stdout, stderr)
	defer e.executeList(ctx, e.post, stdout, stderr)

	if err := e.list.Execute(ctx, stdout, stderr); err != nil {
		return err
	}
	prepare(e.Command, stdout, stderr)
	var (
		next = e.success
		err  = e.Command.Execute(ctx, e.args)
	)
	if e.ignore && err != nil {
		err = nil
	}
	if err != nil {
		next = e.errors
	}
	e.executeList(ctx, next, stdout, stderr)
	return err
}

func (e execmain) executeList(ctx context.Context, list []Command, stdout, stderr io.Writer) error {
	if len(list) == 0 {
		return nil
	}
	for _, e := range list {
		prepare(e, stdout, stderr)
		err := e.Execute(ctx, nil)
		if errors.Is(err, context.Canceled) {
			return err
		}
	}
	return nil
}

type deplist []execdep

func (el deplist) Execute(ctx context.Context, stdout, stderr io.Writer) error {
	grp, sub := errgroup.WithContext(ctx)
	for i := range el {
		ex := el[i]
		if ex.background {
			grp.Go(func() error {
				return ex.Execute(sub, stdout, stderr)
			})
		} else {
			err := ex.Execute(sub, stdout, stderr)
			_ = err
		}
	}
	return grp.Wait()
}

type execdep struct {
	Command
	args []string

	list       deplist
	background bool
}

func createDep(cmd Command, args []string, list deplist) execdep {
	return execdep{
		Command: cmd,
		args:    args,
		list:    list,
	}
}

func (e execdep) Execute(ctx context.Context, stdout, stderr io.Writer) error {
	if err := e.list.Execute(ctx, stdout, stderr); err != nil {
		return err
	}
	prepare(e.Command, stdout, stderr)
	return e.Command.Execute(ctx, e.args)
}

type exectrace struct {
	inner executer
}

func trace(ex executer) executer {
	return exectrace{
		inner: ex,
	}
}

func (e exectrace) Execute(ctx context.Context, stdout, stderr io.Writer) error {
	var (
		now = time.Now()
		err = e.inner.Execute(ctx, stdout, stderr)
	)
	setPrefix(stderr, "trace")
	if err != nil {
		fmt.Fprintln(stderr, "fail")
	}
	fmt.Fprintf(stderr, "time: %.3fs", time.Since(now))
	fmt.Fprintln(stderr)

	return err
}

func prepare(cmd Command, stdout, stderr io.Writer) {
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	setPrefix(stdout, cmd.Command())
	setPrefix(stderr, cmd.Command())
}

func setPrefix(w io.Writer, name string) {
	p, ok := w.(interface{ SetPrefix(string) })
	if !ok {
		return
	}
	p.SetPrefix(name)
}
