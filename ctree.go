package maestro

import (
	"context"
	"errors"
	"io"
)

type Executer interface {
	Execute(context.Context, []string) error
}

type execmain struct {
  Command
  args []string

  ignore bool
  prefix bool
  trace  bool
}

func (e execmain) Execute(ctx context.Context) error {
  return nil
}

type execdep struct {
	Command

	background bool
	optional   bool
	args       []string
}

func (e execdep) Execute(ctx context.Context) error {
  return nil
}

type exectree struct {
	main Executer
	args []string
	deps []Executer

	pre     []Executer
	post    []Executer
	success []Executer
	errors  []Executer

	stdout io.Writer
	stderr io.Writer
}

func (e exectree) Execute(ctx context.Context, args []string) error {
	if err := e.executeList(ctx, e.pre); err != nil {
		return err
	}
	defer e.executeList(ctx, e.post)

	if err := e.executeDependencies(ctx); err != nil {
		return err
	}
	var (
		err  = e.main.Execute(ctx, args)
		next = e.success
	)
	if err != nil {
		next = e.errors
	}
	e.executeList(ctx, next)
	return err
}

func (e exectree) executeDependencies(ctx context.Context) error {
	for _, e := range e.deps {
		if err := e.Execute(ctx, nil); err != nil {
			return err
		}
	}
	// wait for all dependencies to complete before returning
	return nil
}

func (e exectree) executeList(ctx context.Context, list []Executer) error {
	if len(list) == 0 {
		return nil
	}
	for _, e := range list {
		err := e.Execute(ctx, nil)
		if errors.Is(err, context.Canceled) {
			return err
		}
	}
	return nil
}
