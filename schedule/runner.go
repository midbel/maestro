package schedule

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"
)

var (
	ErrDone = errors.New("done")
)

type Runner interface {
	Run(context.Context) error
}

func Trace(r Runner, name string) Runner {
	return &traceRunner{
		name:   name,
		Runner: r,
	}
}

func DoBefore(r Runner, do func() error) Runner {
	return &doRunner{
		before: do,
		Runner: r,
	}
}

func DoAfter(r Runner, do func(error) error) Runner {
	return &doRunner{
		after:  do,
		Runner: r,
	}
}

func LimitRunning(r Runner, max int) Runner {
	return &limitRunner{
		limit:  max,
		Runner: r,
	}
}

func SkipRunning(r Runner) Runner {
	return &skipRunner{
		Runner: r,
	}
}

func DelayRunner(r Runner, wait time.Duration) Runner {
	return &delayRunner{
		wait:   wait,
		Runner: r,
	}
}

type runFunc func(context.Context) error

func (r runFunc) Run(ctx context.Context) error {
	return r(ctx)
}

type limitRunner struct {
	mu    sync.Mutex
	limit int
	curr  int
	Runner
}

func (r *limitRunner) Run(ctx context.Context) error {
	if !r.can() {
		return nil
	}
	r.inc()
	defer r.dec()
	return r.Runner.Run(ctx)
}

func (r *limitRunner) can() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.curr <= r.limit
}

func (r *limitRunner) inc() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.curr++
}

func (r *limitRunner) dec() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.curr--
}

type skipRunner struct {
	mu      sync.Mutex
	running bool
	Runner
}

func (r *skipRunner) Run(ctx context.Context) error {
	if r.isRunning() {
		return nil
	}
	r.toggle()
	defer r.toggle()
	return r.Runner.Run(ctx)
}

func (r *skipRunner) isRunning() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.running
}

func (r *skipRunner) toggle() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.running = !r.running
}

type delayRunner struct {
	wait time.Duration
	Runner
}

func (r *delayRunner) Run(ctx context.Context) error {
	<-time.After(r.wait)
	return r.Runner.Run(ctx)
}

type timeoutRunner struct {
	timeout time.Duration
	Runner
}

func (r *timeoutRunner) Run(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	err := r.Runner.Run(ctx)
	if err == nil {
		err = ctx.Err()
	}
	return err
}

type doRunner struct {
	before func() error
	after  func(error) error
	Runner
}

func (r *doRunner) Run(ctx context.Context) error {
	var err error
	if r.before != nil {
		err = r.before()
	}
	if err != nil {
		return err
	}
	err = r.Runner.Run(ctx)
	if r.after != nil {
		err = r.after(err)
	}
	return err
}

type traceRunner struct {
	name string
	Runner
}

func (r *traceRunner) Run(ctx context.Context) error {
	log.Printf("[%s] start", r.name)
	var (
		now = time.Now()
		err = r.Runner.Run(ctx)
	)
	if err != nil {
		log.Printf("[%s] error: %s", r.name, err)
	}
	log.Printf("[%s] done (elapsed: %s)", r.name, time.Since(now))
	return err
}
