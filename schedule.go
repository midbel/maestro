package maestro

import (
	"context"
	"io"
	"os"

	"github.com/midbel/maestro/schedule"
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

type Schedule struct {
	Sched    *schedule.Scheduler
	Args     []string
	Stdout   ScheduleRedirect
	Stderr   ScheduleRedirect
	Notify   []string
	Overlap  bool
}

func (s *Schedule) Run(ctx context.Context, cmd CommandSettings, stdout, stderr io.Writer) error {
	r := s.makeRunner(cmd, stdout, stderr)
	if !s.Overlap {
		r = schedule.SkipRunning(r)
	}
	return s.Sched.Run(ctx, r)
}

func (s *Schedule) makeRunner(cmd CommandSettings, stdout, stderr io.Writer) schedule.Runner {
	return createRunner(cmd, s.Args, stdout, stderr)
}

type runner struct {
	cmd CommandSettings
	args []string
	out io.Writer
	err io.Writer
}

func createRunner(cmd CommandSettings, args []string, stdout, stderr io.Writer) schedule.Runner {
	return runner{
		cmd: cmd,
		args: args,
		out: stdout,
		err: stderr,
	}
}

func (r runner) Run(ctx context.Context) error {
	x, err := r.cmd.Prepare()
	if err != nil {
		return err
	}
	x.SetOut(r.out)
	x.SetErr(r.err)
	return x.Execute(ctx, r.args)
}
