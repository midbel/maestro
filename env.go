package maestro

import (
	"fmt"
	"strings"
)

type Env struct {
	parent *Env
	locals map[string][]string
}

func EmptyEnv() *Env {
	return EnclosedEnv(nil)
}

func EnclosedEnv(parent *Env) *Env {
	return &Env{
		parent: parent,
		locals: make(map[string][]string),
	}
}

func (e *Env) Set(str string) error {
	if len(str) == 0 {
		return fmt.Errorf("no ident provided")
	}
	x := strings.Index(str, "=")
	if x < 0 {
		e.Define(str, nil)
	} else {
		e.Define(str[:x], []string{str[x+1:]})
	}
	return nil
}

func (e *Env) String() string {
	return "locals"
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
	return vs, nil
}

func (e *Env) Unwrap() *Env {
	if e.parent == nil {
		return e
	}
	return e.parent
}

func (e *Env) Copy() *Env {
	locals := make(map[string][]string)
	locals = copyLocals(locals, e.locals)
	if e.parent != nil {

	}
	return &Env{locals: locals}
}

func copyLocals(locals, others map[string][]string) map[string][]string {
	for k, vs := range others {
		locals[k] = append(locals[k], vs...)
	}
	return locals
}
