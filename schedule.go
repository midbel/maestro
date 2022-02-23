package maestro

import (
	"context"
	"io"
	"os"
	"time"

	"github.com/midbel/maestro/schedule"
	"golang.org/x/sync/errgroup"
)

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
	Process  bool
	Overlap  bool
	Preserve bool
}

func (s *Schedule) Run(ctx context.Context, ex Executer, stdout, stderr io.Writer) error {
	var err error

	stdout, err = s.Stdout.Writer(stdout)
	if err != nil {
		return err
	}
	if c, ok := stdout.(io.Closer); ok {
		defer c.Close()
	}

	stderr, err = s.Stderr.Writer(stderr)
	if err != nil {
		return err
	}
	if c, ok := stderr.(io.Closer); ok {
		defer c.Close()
	}
	ex.SetOut(stdout)
	ex.SetErr(stderr)
	return s.run(ctx, ex)
}

func (s *Schedule) run(ctx context.Context, ex Executer) error {
	var (
		now     time.Time
		next    time.Time
		grp     errgroup.Group
		running bool
	)
	for {
		now = time.Now()
		next = s.Sched.Next()
		wait := next.Sub(now)
		if wait <= 0 {
			continue
		}
		select {
		case <-ctx.Done():
			err := grp.Wait()
			if err == nil {
				err = ctx.Err()
			}
			return err
		}
		grp.Go(func() error {
			<-time.After(wait)
			if !s.Overlap && running {
				return nil
			}
			return ex.Execute(ctx, s.Args)
		})
	}
	return grp.Wait()
}
