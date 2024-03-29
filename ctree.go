package maestro

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"golang.org/x/sync/errgroup"
)

type executer interface {
	Execute(context.Context, io.Writer, io.Writer) error
}

type ctreeOption struct {
	Ignore bool
	Prefix bool
	Trace  bool
	NoDeps bool
}

type ctree struct {
	root executer

	prefix bool

	stdout *pipe
	stderr *pipe
}

func createTree(root executer) (ctree, error) {
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

func (c *ctree) Execute(ctx context.Context, stdout, stderr io.Writer) error {
	go io.Copy(stdout, c.stdout)
	go io.Copy(stderr, c.stderr)

	return c.root.Execute(ctx, c.Stdout(), c.Stderr())
}

func (c *ctree) Stdout() io.Writer {
	return createWriter(c.stdout, c.prefix)
}

func (c *ctree) Stderr() io.Writer {
	return createWriter(c.stderr, c.prefix)
}

func (c *ctree) Close() error {
	c.stdout.Close()
	return c.stderr.Close()
}

type execmain struct {
	Executer
	args []string

	list deplist

	ignore bool

	pre     []Executer
	post    []Executer
	success []Executer
	errors  []Executer
}

func createMain(cmd Executer, args []string, list deplist) execmain {
	return execmain{
		Executer: cmd,
		args:     args,
		list:     list,
	}
}

func (e execmain) Execute(ctx context.Context, stdout, stderr io.Writer) error {
	e.executeList(ctx, e.pre, stdout, stderr)
	defer e.executeList(ctx, e.post, stdout, stderr)

	if err := e.list.Execute(ctx, stdout, stderr); err != nil {
		return err
	}
	prepare(e.Executer, stdout, stderr)
	var (
		next = e.success
		err  = e.Executer.Execute(ctx, e.args)
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

func (e execmain) executeList(ctx context.Context, list []Executer, stdout, stderr io.Writer) error {
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

type deplist []executer

func (el deplist) Execute(ctx context.Context, stdout, stderr io.Writer) error {
	inBackground := func(e executer) bool {
		b, ok := e.(interface{ Bg() bool })
		if !ok {
			return ok
		}
		return b.Bg()
	}
	grp, sub := errgroup.WithContext(ctx)
	for i := range el {
		ex := el[i]
		if inBackground(ex) {
			grp.Go(func() error {
				return ex.Execute(sub, stdout, stderr)
			})
		} else {
			err := ex.Execute(sub, stdout, stderr)
			if err != nil {
				grp.Wait()
				return err
			}
		}
	}
	return grp.Wait()
}

type execdep struct {
	Executer
	args []string

	list       deplist
	background bool
}

func createDep(cmd Executer, args []string, list deplist) execdep {
	return execdep{
		Executer: cmd,
		args:     args,
		list:     list,
	}
}

func (e execdep) Execute(ctx context.Context, stdout, stderr io.Writer) error {
	if err := e.list.Execute(ctx, stdout, stderr); err != nil {
		return err
	}
	prepare(e.Executer, stdout, stderr)
	return e.Executer.Execute(ctx, e.args)
}

func (e execdep) Bg() bool {
	return e.background
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
		now     = time.Now()
		err     = e.inner.Execute(ctx, stdout, stderr)
		elapsed = time.Since(now)
	)
	setPrefix(stderr, "trace")
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
	}
	fmt.Fprintf(stderr, "time: %.3fs", elapsed.Seconds())
	fmt.Fprintln(stderr)

	return err
}

type pipe struct {
	R *os.File
	W *os.File

	scan   *bufio.Scanner
	prefix string
}

func createPipe() (*pipe, error) {
	var (
		p   pipe
		err error
	)
	p.R, p.W, err = os.Pipe()
	if err == nil {
		p.scan = bufio.NewScanner(p.R)
	}
	return &p, err
}

func (p *pipe) SetPrefix(prefix string) {
	p.prefix = ""
	if prefix != "" {
		p.prefix = fmt.Sprintf("[%s] ", prefix)
	}
}

func (p *pipe) Close() error {
	p.R.Close()
	return p.W.Close()
}

func (p *pipe) Write(b []byte) (int, error) {
	return p.W.Write(b)
}

func (p *pipe) Read(b []byte) (int, error) {
	if !p.scan.Scan() {
		err := p.scan.Err()
		if err == nil {
			err = io.EOF
		}
		return 0, io.EOF
	}
	var n int
	if p.prefix != "" {
		n = copy(b, p.prefix)
	}
	x := p.scan.Bytes()
	n += copy(b[n:], append(x, '\n'))
	return n, p.scan.Err()
}

func prepare(cmd Executer, stdout, stderr io.Writer) {
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

func createWriter(w io.Writer, prefix bool) io.Writer {
	if prefix {
		return w
	}
	return createNoopPrefix(w)
}

type noopPrefix struct {
	io.Writer
}

func createNoopPrefix(w io.Writer) io.Writer {
	return noopPrefix{
		Writer: w,
	}
}

func (_ noopPrefix) SetPrefix(_ string) {
	// noop
}
