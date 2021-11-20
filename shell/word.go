package shell

import (
	"fmt"
	"strconv"
	"strings"
)

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

type Executer interface{}

type Expander interface {
	Expand(Environment) ([]string, error)
}

type ExecAssign struct {
	Ident string
	Expander
}

type ExecAnd struct {
	Left  Executer
	Right Executer
}

type ExecOr struct {
	Left  Executer
	Right Executer
}

type ExecPipe struct {
	List []Executer
}

type ExecSimple struct {
	Expander
	// In    Expander
	// Out   Expander
	// Err   Expander
}

type ExpandList struct {
	List []Expander
}

func (e ExpandList) Expand(env Environment) ([]string, error) {
	var str []string
	for i := range e.List {
		ws, err := e.List[i].Expand(env)
		if err != nil {
			return nil, err
		}
		str = append(str, ws...)
	}
	return str, nil
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
		str = str[:1]
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

var (
	lowerA  byte = 'a'
	lowerZ  byte = 'z'
	upperA  byte = 'A'
	upperZ  byte = 'Z'
	deltaLU byte = 32
)

type ExpandLower struct {
	Ident string
	All   bool
}

func (v ExpandLower) Expand(env Environment) ([]string, error) {
	str, err := env.Resolve(v.Ident)
	if err != nil {
		return nil, err
	}
	if v.All {
		str = v.lowerAll(str)
	} else {
		str = v.lowerFirst(str)
	}
	return str, nil
}

func (v ExpandLower) lowerFirst(str []string) []string {
	for i := range str {
		if len(str) == 0 {
			continue
		}
		b := []byte(str[i])
		if b[0] >= upperA && b[0] <= upperZ {
			b[0] += deltaLU
		}
		str[i] = string(b)
	}
	return str
}

func (v ExpandLower) lowerAll(str []string) []string {
	for i := range str {
		str[i] = strings.ToLower(str[i])
	}
	return str
}

type ExpandUpper struct {
	Ident string
	All   bool
}

func (v ExpandUpper) Expand(env Environment) ([]string, error) {
	str, err := env.Resolve(v.Ident)
	if err != nil {
		return nil, err
	}
	if v.All {
		str = v.upperAll(str)
	} else {
		str = v.upperFirst(str)
	}
	return str, nil
}

func (v ExpandUpper) upperFirst(str []string) []string {
	for i := range str {
		if len(str) == 0 {
			continue
		}
		b := []byte(str[i])
		if b[0] >= lowerA && b[0] <= lowerZ {
			b[0] -= deltaLU
		}
		str[i] = string(b)
	}
	return str
}

func (v ExpandUpper) upperAll(str []string) []string {
	for i := range str {
		str[i] = strings.ToUpper(str[i])
	}
	return str
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
