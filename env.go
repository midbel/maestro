package maestro

import (
	"fmt"

	"github.com/midbel/slices"
)

type Values map[string][]string

type nameset map[string]string

func (n nameset) All() []string {
	var list []string
	for k, v := range n {
		list = append(list, fmt.Sprintf("%s=%s", k, v))
	}
	return list
}

type cmdEnv struct {
	locals *Env
	values map[string]flag.Value
}

func (e *cmdEnv) Resolve(ident string) ([]string, error) {
	if vs, err := e.resolve(ident); err == nil {
		return vs, nil
	}
	return e.locals.Resolve(ident)
}

func (e *cmdEnv) resolve(ident string) ([]string, error) {
	return nil, nil
}

func (e *cmdEnv) attach(short, long, help, value string) error {
	return nil
}

func (e *cmdEnv) attachFlag(short, long, help string, value bool) error {
	return nil
}

type Env struct {
	parent *Env
	locals Values
}

func EmptyEnv() *Env {
	return EnclosedEnv(nil)
}

func EnclosedEnv(parent *Env) *Env {
	return &Env{
		parent: parent,
		locals: make(Values),
	}
}

func (e *Env) Define(ident string, vs []string) error {
	e.locals[ident] = append(e.locals[ident][:0], vs...)
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
