package expand

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/midbel/slices"
)

type Resolver interface {
	Resolve(string) ([]string, error)
}

type Expander interface {
	Expand(Resolver) ([]string, error)
}

type literal string

func createLiteral(str string) Expander {
	return literal(str)
}

func (e literal) Expand(_ Resolver) ([]string, error) {
	return toArray(string(e)), nil
}

type variable string

func createVariable(str string) Expander {
	return variable(str)
}

func (e variable) Expand(r Resolver) ([]string, error) {
	return r.Resolve(string(e))
}

type combined []Expander

func createList(ex ...Expander) Expander {
	if len(ex) == 1 {
		return ex[0]
	}
	return combined(ex)
}

func (e combined) Expand(r Resolver) ([]string, error) {
	var str strings.Builder
	for _, x := range e {
		s, err := x.Expand(r)
		if err != nil {
			return nil, err
		}
		str.WriteString(strings.Join(s, ""))
	}
	return toArray(str.String()), nil
}

type brace struct {
	list []Expander
	pre  Expander
	post Expander
}

func createBrace(list []Expander, pre, post Expander) Expander {
	return brace{
		list: list,
		pre:  pre,
		post: post,
	}
}

func (e brace) Expand(r Resolver) ([]string, error) {
	var (
		list []string
		all  [][]string
	)
	if e.pre != nil {
		pre, err := e.pre.Expand(r)
		if err != nil {
			return nil, err
		}
		if len(pre) > 0 {
			all = append(all, pre)
		}
	}
	for _, i := range e.list {
		str, err := i.Expand(r)
		if err != nil {
			return nil, err
		}
		list = append(list, str...)
	}
	all = append(all, list)
	if e.post != nil {
		post, err := e.post.Expand(r)
		if err != nil {
			return nil, err
		}
		if len(post) > 0 {
			all = append(all, post)
		}
	}
	if len(all) == 1 {
		return slices.Fst(all), nil
	}
	var ret []string
	for _, r := range slices.Combine(all...) {
		ret = append(ret, strings.Join(r, ""))
	}
	return ret, nil
}

type length struct {
	Expander
}

func createLength(ex Expander) Expander {
	return length{
		Expander: ex,
	}
}

func (e length) Expand(r Resolver) ([]string, error) {
	str, err := resolveOne(e.Expander, r)
	if err != nil {
		return nil, err
	}
	n := len(str)
	return toArray(strconv.Itoa(n)), nil
}

type stripPrefix struct {
	Expander
	strip string
	long  bool
}

func createPrefix(ex Expander, str string, long bool) Expander {
	return stripPrefix{
		Expander: ex,
		strip:    str,
		long:     long,
	}
}

func (e stripPrefix) Expand(r Resolver) ([]string, error) {
	str, err := resolveOne(e.Expander, r)
	if err != nil {
		return nil, err
	}
	str = strings.TrimPrefix(str, e.strip)
	for strings.HasPrefix(str, e.strip) && e.long {
		str = strings.TrimPrefix(str, e.strip)
	}
	return toArray(str), nil
}

type stripSuffix struct {
	Expander
	strip string
	long  bool
}

func createSuffix(ex Expander, str string, long bool) Expander {
	return stripSuffix{
		Expander: ex,
		strip:    str,
		long:     long,
	}
}

func (e stripSuffix) Expand(r Resolver) ([]string, error) {
	str, err := resolveOne(e.Expander, r)
	if err != nil {
		return nil, err
	}
	str = strings.TrimSuffix(str, e.strip)
	for strings.HasSuffix(str, e.strip) && e.long {
		str = strings.TrimSuffix(str, e.strip)
	}
	return toArray(str), nil
}

type substring struct {
	Expander
	offset int
	length int
}

func createSubstring(ex Expander, from, size int) Expander {
	return substring{
		Expander: ex,
		offset:   from,
		length:   size,
	}
}

func (e substring) Expand(r Resolver) ([]string, error) {
	str, err := resolveOne(e.Expander, r)
	if err != nil {
		return nil, err
	}
	if e.offset < len(str) {
		str = str[e.offset:]
	}
	if e.length > 0 && e.length < len(str) {
		str = str[:e.length]
	}
	return toArray(str), nil
}

type replaceFirst struct {
	Expander
	src string
	dst string
}

func createReplaceFirst(ex Expander, src, dst string) Expander {
	return replaceFirst{
		Expander: ex,
		src:      src,
		dst:      dst,
	}
}

func (e replaceFirst) Expand(r Resolver) ([]string, error) {
	str, err := resolveOne(e.Expander, r)
	if err != nil {
		return nil, err
	}
	str = strings.Replace(str, e.src, e.dst, 1)
	return toArray(str), nil
}

type replaceAll struct {
	Expander
	src string
	dst string
}

func createReplaceAll(ex Expander, src, dst string) Expander {
	return replaceAll{
		Expander: ex,
		src:      src,
		dst:      dst,
	}
}

func (e replaceAll) Expand(r Resolver) ([]string, error) {
	str, err := resolveOne(e.Expander, r)
	if err != nil {
		return nil, err
	}
	str = strings.ReplaceAll(str, e.src, e.dst)
	return toArray(str), nil
}

type replacePrefix struct {
	Expander
	src string
	dst string
}

func createReplacePrefix(ex Expander, src, dst string) Expander {
	return replacePrefix{
		Expander: ex,
		src:      src,
		dst:      dst,
	}
}

func (e replacePrefix) Expand(r Resolver) ([]string, error) {
	str, err := resolveOne(e.Expander, r)
	if err != nil {
		return nil, err
	}
	if strings.HasPrefix(str, e.src) {
		str = e.dst + str[len(e.src):]
	}
	return toArray(str), nil
}

type replaceSuffix struct {
	Expander
	src string
	dst string
}

func createReplaceSuffix(ex Expander, src, dst string) Expander {
	return replaceSuffix{
		Expander: ex,
		src:      src,
		dst:      dst,
	}
}

func (e replaceSuffix) Expand(r Resolver) ([]string, error) {
	str, err := resolveOne(e.Expander, r)
	if err != nil {
		return nil, err
	}
	if strings.HasSuffix(str, e.src) {
		str = str[:len(str)-len(e.src)] + e.dst
	}
	return toArray(str), nil
}

func toArray(str ...string) []string {
	return str
}

func resolveOne(e Expander, r Resolver) (string, error) {
	str, err := e.Expand(r)
	if err != nil {
		return "", err
	}
	if len(str) != 1 {
		return "", fmt.Errorf("too many values expanded")
	}
	return str[0], nil
}
