package main

import (
	"fmt"
  "io"
	"strconv"
	"strings"
)

type Script struct {
	Stdout io.Writer
	Stderr io.Writer
	Stdin  io.Reader

  env Environment
}

type Executer interface {
	Execute(Environment) error
}

type Environment interface {
	Resolve(string) ([]string, error)
	Define(string, []string) error
	Delete(string)
}

type Env struct {
	parent *Env
	values map[string][]string
}

func EmptyEnv() *Env {
	return EnclosedEnv(nil)
}

func EnclosedEnv(parent *Env) *Env {
	return &Env{
		parent: parent,
		values: make(map[string][]string),
	}
}

func (e *Env) Resolve(ident string) ([]string, error) {
	vs, ok := e.values[ident]
	if ok {
		return vs, nil
	}
	if e.parent != nil {
		return e.parent.Resolve(ident)
	}
	return nil, fmt.Errorf("%s: undefined variable", ident)
}

func (e *Env) Define(ident string, vs []string) error {
	e.values[ident] = vs
	return nil
}

func (e *Env) Delete(ident string) {
	delete(e.values, ident)
}

type Expander interface {
	Expand(Environment) ([]string, error)
}

type ExecAnd struct {
	Left  Executer
	Right Executer
}

func (a ExecAnd) Execute(env Environment) error {
	if err := a.Left.Execute(env); err != nil {
		return err
	}
	return a.Right.Execute(env)
}

type ExecOr struct {
	Left  Executer
	Right Executer
}

func (o ExecOr) Execute(env Environment) error {
	err := o.Left.Execute(env)
	if err == nil {
		return err
	}
	return o.Right.Execute(env)
}

type ExecPipe struct {
	List []Executer
}

func (p ExecPipe) Execute(env Environment) error {
	return nil
}

type ExecList struct {
	List []Executer
}

func (i ExecList) Execute(env Environment) error {
  for _, i := range i.List {
    if err := i.Execute(env); err != nil {
      return err
    }
  }
	return nil
}

type ExecSimple struct {
	Words []Expander
	// In    Expander
	// Out   Expander
	// Err   Expander
}

func (s ExecSimple) Execute(env Environment) error {
	var words []string
	for _, w := range s.Words {
		ws, err := w.Expand(env)
		if err != nil {
			return err
		}
		words = append(words, ws...)
	}
	fmt.Println(strings.Join(words, " "))
	return nil
}

type ExecAssign struct {
	Ident string
	Words []Expander
}

func (a ExecAssign) Execute(env Environment) ([]string, error) {
	return nil, nil
}

type ExpandWord struct {
	Literal string
}

func (w ExpandWord) Expand(_ Environment) ([]string, error) {
	return []string{w.Literal}, nil
}

type ExpandMulti struct {
	List []Expander
}

func (m ExpandMulti) Expand(env Environment) ([]string, error) {
	var words []string
	for _, w := range m.List {
		ws, err := w.Expand(env)
		if err != nil {
			return nil, err
		}
		words = append(words, ws...)
	}
	str := strings.Join(words, "")
	return []string{str}, nil
}

type ExpandVariable struct {
	Ident  string
	Quoted bool
}

func (v ExpandVariable) Expand(env Environment) ([]string, error) {
	str, err := env.Resolve(v.Ident)
	if err != nil {
		return nil, err
	}
	if v.Quoted {
		str[0] = strings.Join(str, " ")
		str = str[1:]
	}
	return str, nil
}

type ExpandLength struct {
	Ident string
}

func (v ExpandLength) Expand(env Environment) ([]string, error) {
	var (
		ws, err = env.Resolve(v.Ident)
		sz      int
	)
	if err != nil {
		return nil, err
	}
	for i := range ws {
		sz += len(ws[i])
	}
	s := strconv.Itoa(sz)
	return []string{s}, nil
}

type ExpandReplace struct {
	Ident string
	From  string
	To    string
	What  rune
}

func (v ExpandReplace) Expand(env Environment) ([]string, error) {
	str, err := env.Resolve(v.Ident)
  if err != nil {
    return nil, err
  }
  switch v.What {
  case Replace:
    str = v.replace(str)
  case ReplaceAll:
    str = v.replaceAll(str)
  case ReplacePrefix:
    str = v.replacePrefix(str)
  case ReplaceSuffix:
    str = v.replaceSuffix(str)
  }
  return str, nil
}

func (v ExpandReplace) replace(str []string) []string {
  for i := range str {
    str[i] = strings.Replace(str[i], v.From, v.To, 1)
  }
  return str
}

func (v ExpandReplace) replaceAll(str []string) []string {
  for i := range str {
    str[i] = strings.ReplaceAll(str[i], v.From, v.To)
  }
  return str
}

func (v ExpandReplace) replacePrefix(str []string) []string {
  return v.replace(str)
}

func (v ExpandReplace) replaceSuffix(str []string) []string {
  return v.replace(str)
}

type ExpandTrim struct {
	Ident string
	Trim  string
	What  rune
}

func (v ExpandTrim) Expand(env Environment) ([]string, error) {
	str, err := env.Resolve(v.Ident)
  if err != nil {
    return nil, err
  }
  switch v.What {
  case TrimSuffix:
    str = v.trimSuffix(str)
  case TrimSuffixLong:
    str = v.trimSuffixLong(str)
  case TrimPrefix:
    str = v.trimPrefix(str)
  case TrimPrefixLong:
    str = v.trimPrefixLong(str)
  }
  return str, nil
}

func (v ExpandTrim) trimSuffix(str []string) []string {
  for i := range str {
    str[i] = strings.TrimSuffix(str[i], v.Trim)
  }
  return str
}

func (v ExpandTrim) trimSuffixLong(str []string) []string {
  for i := range str {
    for strings.HasSuffix(str[i], v.Trim) {
      str[i] = strings.TrimSuffix(str[i], v.Trim)
    }
  }
  return str
}

func (v ExpandTrim) trimPrefix(str []string) []string {
  for i := range str {
    str[i] = strings.TrimPrefix(str[i], v.Trim)
  }
  return str
}

func (v ExpandTrim) trimPrefixLong(str []string) []string {
  for i := range str {
    for strings.HasPrefix(str[i], v.Trim) {
      str[i] = strings.TrimPrefix(str[i], v.Trim)
    }
  }
  return str
}

type ExpandSlice struct {
	Ident string
	From  int
	To    int
}

func (v ExpandSlice) Expand(env Environment) ([]string, error) {
	return env.Resolve(v.Ident)
}

type ExpandLower struct {
	Ident string
	All   bool
}

func (v ExpandLower) Expand(env Environment) ([]string, error) {
	return env.Resolve(v.Ident)
}

type ExpandUpper struct {
	Ident string
	All   bool
}

func (v ExpandUpper) Expand(env Environment) ([]string, error) {
	return env.Resolve(v.Ident)
}

type ExpandValIfUnset struct {
	Ident string
	Str   string
}

func (v ExpandValIfUnset) Expand(env Environment) ([]string, error) {
	str, err := env.Resolve(v.Ident)
	if err == nil {
		return str, nil
	}
	return []string{v.Str}, nil
}

type ExpandSetValIfUnset struct {
	Ident string
	Str   string
}

func (v ExpandSetValIfUnset) Expand(env Environment) ([]string, error) {
	str, err := env.Resolve(v.Ident)
	if err != nil {
		str = []string{v.Str}
		env.Define(v.Ident, str)
	}
	return str, nil
}

type ExpandValIfSet struct {
	Ident string
	Str   string
}

func (v ExpandValIfSet) Expand(env Environment) ([]string, error) {
	str, err := env.Resolve(v.Ident)
	if err == nil {
		str = []string{v.Str}
	}
	return str, nil
}

type ExpandExitIfUnset struct {
	Ident string
	Str   string
}

func (v ExpandExitIfUnset) Expand(env Environment) ([]string, error) {
	return nil, nil
}
