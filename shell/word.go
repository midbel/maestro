package shell

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/midbel/maestro/shlex"
)

type Environment interface {
	Resolve(string) ([]string, error)
	Define(string, []string) error
	Delete(string) error
	// SetReadOnly(string)
}

type Env struct {
	parent Environment
	values map[string][]string
}

func EmptyEnv() Environment {
	return EnclosedEnv(nil)
}

func EnclosedEnv(parent Environment) Environment {
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

func (e *Env) Delete(ident string) error {
	delete(e.values, ident)
	return nil
}

type Executer interface{}

type Expander interface {
	Expand(Environment) ([]string, error)
}

type ExecAssign struct {
	Ident string
	Expander
}

func createAssign(ident string, ex Expander) ExecAssign {
	return ExecAssign{
		Ident:    ident,
		Expander: ex,
	}
}

type ExecAnd struct {
	Left  Executer
	Right Executer
}

func createAnd(left, right Executer) ExecAnd {
	return ExecAnd{
		Left:  left,
		Right: right,
	}
}

type ExecOr struct {
	Left  Executer
	Right Executer
}

func createOr(left, right Executer) ExecOr {
	return ExecOr{
		Left:  left,
		Right: right,
	}
}

type ExecPipe struct {
	List []pipeitem
}

func createPipe(list []pipeitem) ExecPipe {
	return ExecPipe{
		List: list,
	}
}

type pipeitem struct {
	Executer
	Both bool
}

func createPipeItem(ex Executer, both bool) pipeitem {
	return pipeitem{
		Executer: ex,
		Both:     both,
	}
}

type ExecSimple struct {
	Expander
	In  Expander
	Out Expander
	Err Expander
}

func createSimple(ex Expander) ExecSimple {
	return ExecSimple{
		Expander: ex,
	}
}

type ExpandSub struct {
	List   []Executer
	Quoted bool
}

func (e ExpandSub) Expand(env Environment) ([]string, error) {
	sh, ok := env.(*Shell)
	if !ok {
		return nil, fmt.Errorf("substitution can not expanded")
	}
	var (
		err error
		buf bytes.Buffer
	)
	sh, _ = sh.Subshell()
	sh.SetStdout(&buf)

	for i := range e.List {
		if err = sh.execute(e.List[i]); err != nil {
			return nil, err
		}
	}
	return shlex.Split(&buf)
}

type ExpandList struct {
	List   []Expander
	Quoted bool
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

func (e *ExpandList) Pop() Expander {
	n := len(e.List)
	if n == 0 {
		return nil
	}
	n--
	x := e.List[n]
	e.List = e.List[:n]
	return x
}

type ExpandWord struct {
	Literal string
	Quoted  bool
}

func createWord(str string, quoted bool) ExpandWord {
	return ExpandWord{
		Literal: str,
		Quoted:  quoted,
	}
}

func (w ExpandWord) Expand(env Environment) ([]string, error) {
	if w.Quoted {
		return []string{w.Literal}, nil
	}
	return w.expand(env)
}

func (w ExpandWord) expand(env Environment) ([]string, error) {
	if strings.HasPrefix(w.Literal, "~") {
		return w.expandTilde(env)
	}
	if strings.ContainsAny(w.Literal, "[?*") {
		return w.expandPattern()
	}
	return []string{w.Literal}, nil
}

func (w ExpandWord) expandTilde(env Environment) ([]string, error) {
	return []string{w.Literal}, nil
}

func (w ExpandWord) expandPattern() ([]string, error) {
	list, err := filepath.Glob(w.Literal)
	if err != nil {
		list = append(list, w.Literal)
	}
	return list, nil
}

type ExpandListBrace struct {
	Prefix Expander
	Suffix Expander
	Words  []Expander
}

func (b ExpandListBrace) Expand(env Environment) ([]string, error) {
	var (
		prefix []string
		suffix []string
		words  []string
		err    error
	)
	if b.Prefix != nil {
		if prefix, err = b.Prefix.Expand(env); err != nil {
			return nil, err
		}
	}
	if b.Suffix != nil {
		if suffix, err = b.Suffix.Expand(env); err != nil {
			return nil, err
		}
	}
	for i := range b.Words {
		str, err := b.Words[i].Expand(env)
		if err != nil {
			return nil, err
		}
		words = append(words, str...)
	}
	return combineStrings(words, prefix, suffix), nil
}

type ExpandRangeBrace struct {
	Prefix Expander
	Suffix Expander
	Pad    int
	From   int
	To     int
	Step   int
}

func (b ExpandRangeBrace) Expand(env Environment) ([]string, error) {
	var (
		prefix []string
		suffix []string
		words  []string
		err    error
	)
	if b.Prefix != nil {
		if prefix, err = b.Prefix.Expand(env); err != nil {
			return nil, err
		}
	}
	if b.Suffix != nil {
		if suffix, err = b.Suffix.Expand(env); err != nil {
			return nil, err
		}
	}
	if b.Step == 0 {
		b.Step = 1
	}
	cmp := func(from, to int) bool {
		return from <= to
	}
	if b.From > b.To {
		cmp = func(from, to int) bool {
			return from >= to
		}
		if b.Step > 0 {
			b.Step = -b.Step
		}
	}
	for cmp(b.From, b.To) {
		str := strconv.Itoa(b.From)
		if z := len(str); b.Pad > 0 && z < b.Pad {
			str = fmt.Sprintf("%s%s", strings.Repeat("0", b.Pad-z), str)
		}
		words = append(words, str)
		b.From += b.Step
	}
	return combineStrings(words, prefix, suffix), nil
}

type ExpandMulti struct {
	List   []Expander
	Quoted bool
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

func (m *ExpandMulti) Pop() Expander {
	n := len(m.List)
	if n == 0 {
		return nil
	}
	n--
	x := m.List[n]
	m.List = m.List[:n]
	return x
}

type ExpandVar struct {
	Ident  string
	Quoted bool
}

func createVariable(ident string, quoted bool) ExpandVar {
	return ExpandVar{
		Ident:  ident,
		Quoted: quoted,
	}
}

func (v ExpandVar) Expand(env Environment) ([]string, error) {
	str, err := env.Resolve(v.Ident)
	if err != nil {
		return nil, err
	}
	if v.Quoted && len(str) > 0 {
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
	Ident  string
	From   string
	To     string
	What   rune
	Quoted bool
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
	Ident  string
	Trim   string
	What   rune
	Quoted bool
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
	Ident  string
	From   int
	To     int
	Quoted bool
}

func (v ExpandSlice) Expand(env Environment) ([]string, error) {
	return env.Resolve(v.Ident)
}

type ExpandPad struct {
	Ident string
	With  string
	Len   int
	What  rune
}

func (v ExpandPad) Expand(env Environment) ([]string, error) {
	str, err := env.Resolve(v.Ident)
	if err != nil || len(str) >= v.Len {
		return str, err
	}
	for i := range str {
		var (
			diff = v.Len - len(str[i])
			fill = strings.Repeat(v.With, diff)
			ori  = str[i]
		)
		if v.What == PadRight {
			fill, ori = ori, fill
		}
		str[i] = fmt.Sprintf("%s%s", fill, ori)
	}
	return str, nil
}

var (
	lowerA  byte = 'a'
	lowerZ  byte = 'z'
	upperA  byte = 'A'
	upperZ  byte = 'Z'
	deltaLU byte = 32
)

type ExpandLower struct {
	Ident  string
	All    bool
	Quoted bool
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
	Ident  string
	All    bool
	Quoted bool
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
	Ident  string
	Value  string
	Quoted bool
}

func createValIfUnset(ident, value string, quoted bool) ExpandValIfUnset {
	return ExpandValIfUnset{
		Ident:  ident,
		Value:  value,
		Quoted: quoted,
	}
}

func (v ExpandValIfUnset) Expand(env Environment) ([]string, error) {
	str, err := env.Resolve(v.Ident)
	if err == nil {
		return str, nil
	}
	return []string{v.Value}, nil
}

type ExpandSetValIfUnset struct {
	Ident  string
	Value  string
	Quoted bool
}

func createSetValIfUnset(ident, value string, quoted bool) ExpandSetValIfUnset {
	return ExpandSetValIfUnset{
		Ident:  ident,
		Value:  value,
		Quoted: quoted,
	}
}

func (v ExpandSetValIfUnset) Expand(env Environment) ([]string, error) {
	str, err := env.Resolve(v.Ident)
	if err != nil {
		str = []string{v.Value}
		env.Define(v.Ident, str)
	}
	return str, nil
}

type ExpandValIfSet struct {
	Ident  string
	Value  string
	Quoted bool
}

func createExpandValIfSet(ident, value string, quoted bool) ExpandValIfSet {
	return ExpandValIfSet{
		Ident:  ident,
		Value:  value,
		Quoted: quoted,
	}
}

func (v ExpandValIfSet) Expand(env Environment) ([]string, error) {
	str, err := env.Resolve(v.Ident)
	if err == nil {
		str = []string{v.Value}
	}
	return str, nil
}

type ExpandExitIfUnset struct {
	Ident  string
	Value  string
	Quoted bool
}

func createExpandExitIfUnset(ident, value string, quoted bool) ExpandExitIfUnset {
	return ExpandExitIfUnset{
		Ident:  ident,
		Value:  value,
		Quoted: quoted,
	}
}

func (v ExpandExitIfUnset) Expand(env Environment) ([]string, error) {
	return nil, nil
}

func combineStrings(words, prefix, suffix []string) []string {
	if len(prefix) == 0 && len(suffix) == 0 {
		return words
	}
	var (
		tmp strings.Builder
		str = combineStringsWith(&tmp, words, prefix)
	)
	return combineStringsWith(&tmp, suffix, str)
}

func combineStringsWith(ws *strings.Builder, all, with []string) []string {
	if len(with) == 0 {
		return all
	}
	if len(all) == 0 {
		return with
	}
	var str []string
	for i := range with {
		for j := range all {
			ws.WriteString(with[i])
			ws.WriteString(all[j])
			str = append(str, ws.String())
			ws.Reset()
		}
	}
	return str
}
