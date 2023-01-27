package env

import (
	"fmt"
	"strings"

	"github.com/midbel/slices"
)

type Values map[string][]string

type Env struct {
	parent *Env
	locals Values
}

func Empty() *Env {
	return Enclosed(nil)
}

func Enclosed(parent *Env) *Env {
	return &Env{
		parent: parent,
		locals: make(Values),
	}
}

func (e *Env) Define(ident string, vs []string) error {
	e.locals[ident] = append(e.locals[ident][:0], vs...)
	return nil
}

func (e *Env) Append(ident string, vs []string) error {
	xs, ok := e.locals[ident]
	if !ok {
		return fmt.Errorf("%s: identifier not defined", ident)
	}
	e.locals[ident] = append(xs, vs...)
	return nil
}

func (e *Env) Delete(ident string) error {
	delete(e.locals, ident)
	return nil
}

func (e *Env) Resolve(ident string) ([]string, error) {
	vs, ok := e.locals[ident]
	if !ok && e.parent != nil {
		return e.parent.Resolve(ident)
	}
	if !ok {
		return nil, fmt.Errorf("%s: identifier not defined", ident)
	}
	return vs, nil
}

func (e *Env) Join() []string {
	var list []string
	for i, vs := range e.locals {
		if len(vs) == 0 {
			continue
		}
		str := fmt.Sprintf("%s=%s", i, strings.Join(vs, ":"))
		list = append(list, str)
	}
	return list
}

func (e *Env) Unwrap() *Env {
	if e.parent == nil {
		return e
	}
	return e.parent
}

func (e *Env) Copy() *Env {
	x := Env{
		locals: slices.CopyMap(e.locals),
	}
	if e.parent != nil {
		x.parent = e.parent.Copy()
	}
	return &x
}
