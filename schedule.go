package maestro

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"

	"github.com/midbel/maestro/schedule"
	"github.com/midbel/tish"
)

const maxParallelJob = 120

type ScheduleRedirect struct {
	File      string
	Compress  bool
	Duplicate bool
	Overwrite bool
}

func (s ScheduleRedirect) Writer(w io.Writer) (io.Writer, error) {
	if s.File == "" {
		return w, nil
	}
	std, err := os.OpenFile(s.File, s.Option(), 0644)
	if err != nil {
		return nil, err
	}
	if s.Duplicate {
		w = io.MultiWriter(w, std)
	} else {
		w = std
	}
	return w, nil
}

func (s ScheduleRedirect) Option() int {
	base := os.O_CREATE | os.O_WRONLY
	if !s.Overwrite {
		base |= os.O_APPEND
	}
	return base
}

type ScheduleContext struct {
	CommandSettings
	Prefix bool
	Trace  bool
}

func scheduleContext(cmd CommandSettings, prefix, trace bool) ScheduleContext {
	return ScheduleContext{
		CommandSettings: cmd,
		Prefix:          prefix,
		Trace:           trace,
	}
}

type Schedule struct {
	Sched   *schedule.Scheduler
	Args    []string
	Stdout  ScheduleRedirect
	Stderr  ScheduleRedirect
	Notify  []string
	Overlap bool
}

func (s *Schedule) Run(ctx context.Context, reg Registry, cmd ScheduleContext, stdout, stderr io.Writer) error {
	r, err := s.makeRunner(reg, cmd, stdout, stderr)
	if err != nil {
		return err
	}
	if c, ok := r.(io.Closer); ok {
		defer c.Close()
	}
	return s.Sched.Run(ctx, r)
}

func (s *Schedule) makeRunner(reg Registry, cmd ScheduleContext, stdout, stderr io.Writer) (schedule.Runner, error) {
	var err error
	stdout, err = s.Stdout.Writer(stdout)
	if err != nil {
		return nil, err
	}
	if cmd.Prefix {
		stdout = writePrefix(stdout, cmd.Name)
	}
	stderr, err = s.Stderr.Writer(stderr)
	if err != nil {
		return nil, err
	}
	if cmd.Prefix {
		stderr = writePrefix(stderr, cmd.Name)
	}
	r := createRunner(reg, cmd.CommandSettings, s.Args, stdout, stderr)
	if !s.Overlap {
		r = schedule.SkipRunning(r)
	}
	return r, nil
}

type runner struct {
	reg  Registry
	cmd  CommandSettings
	args []string
	out  io.Writer
	err  io.Writer
}

func createRunner(reg Registry, cmd CommandSettings, args []string, stdout, stderr io.Writer) schedule.Runner {
	return runner{
		reg:  reg,
		cmd:  cmd,
		args: args,
		out:  stdout,
		err:  stderr,
	}
}

func (r runner) Find(ctx context.Context, name string) (tish.Command, error) {
	cmd, err := r.reg.Lookup(name)
	if err != nil {
		return nil, err
	}
	x, err := cmd.Prepare()
	if err != nil {
		return nil, err
	}
	return makeShellCommand(ctx, x), nil
}

func (r runner) Run(ctx context.Context) error {
	x, err := r.cmd.Prepare(tish.WithFinder(r))
	if err != nil {
		return nil
	}
	x.SetOut(r.out)
	x.SetErr(r.err)
	err = x.Execute(ctx, r.args)
	if err != nil {
		fmt.Fprintf(r.err, "[%s] %s", r.cmd.Command(), err)
		fmt.Fprintln(r.err)
	}
	return nil
}

func (r runner) Close() error {
	if c, ok := r.err.(io.Closer); ok {
		c.Close()
	}
	if c, ok := r.out.(io.Closer); ok {
		c.Close()
	}
	return nil
}

func writePrefix(w io.Writer, prefix string) io.Writer {
	pr, pw, _ := os.Pipe()
	go func() {
		defer pr.Close()

		scan := bufio.NewScanner(pr)
		for scan.Scan() {
			line := scan.Text()
			if line == "" {
				continue
			}
			fmt.Fprintf(w, "[%s] %s", prefix, line)
			fmt.Fprintln(w)
		}
	}()
	return pw
}
