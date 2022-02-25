package schedule

import (
  "context"
	"errors"
	"sync"
	"time"
)

var (
	ErrDone = errors.New("done")
)

type Runner interface {
	Run(context.Context) error
}

func DoBefore(r Runner, do func() error) Runner {
  return nil
}

func DoAfter(r Runner, do func() error) Runner {
  return nil
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
	return r.Runner.Run(ctx)
}
