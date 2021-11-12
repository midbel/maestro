package maestro

import (
  "fmt"
)

type env struct {
	parent *env
	locals map[string][]string
}

func emptyEnv() *env {
	return enclosedEnv(nil)
}

func enclosedEnv(parent *env) *env {
	return &env{
		parent: parent,
		locals: make(map[string][]string),
	}
}

func (e *env) Define(key string, vs []string) {
	e.locals[key] = append(e.locals[key], vs...)
}

func (e *env) Delete(key string) {
	delete(e.locals, key)
}

func (e *env) Resolve(key string) ([]string, error) {
	var err error
	vs, ok := e.locals[key]
	if !ok {
		if e.parent == nil {
			return nil, fmt.Errorf("%s: %w", key, errUndefined)
		}
		vs, err = e.parent.Resolve(key)
	}
	return vs, err
}

func (e *env) Unwrap() *env {
	if e.parent == nil {
		return e
	}
	return e.parent
}

func (e *env) Values() map[string][]string {
	locals := make(map[string][]string)
	locals = copyLocals(locals, e.locals)
	if e.parent != nil {
		locals = copyLocals(locals, e.parent.Values())
	}
	return locals
}

func copyLocals(locals, others map[string][]string) map[string][]string {
	for k, vs := range others {
		locals[k] = append(locals[k], vs...)
	}
	return locals
}
