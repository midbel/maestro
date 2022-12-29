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

func (e *Env) Set(str string) error {
	if len(str) == 0 {
		return fmt.Errorf("no identifier provided")
	}
	x := strings.Index(str, "=")
	if x < 0 {
		e.Define(str, nil)
	} else {
		e.Define(str[:x], []string{str[x+1:]})
	}
	return nil
}

func (e *Env) Define(key string, vs []string) error {
	e.locals[key] = append(e.locals[key][:0], vs...)
	return nil
}

func (e *Env) Delete(key string) error {
	delete(e.locals, key)
	return nil
}

func (e *Env) Resolve(key string) ([]string, error) {
	vs, ok := e.locals[key]
	if !ok && e.parent != nil {
		return e.parent.Resolve(key)
	}
	if !ok {
		return nil, fmt.Errorf("%s: identifier not defined", key)
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
