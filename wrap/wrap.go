package wrap

import (
	"strings"
	"unicode/utf8"
)

const (
	DefaultLength = 70
	DefaultOffset = 5
)

const (
	space = ' '
	tab   = '\t'
	nl    = '\n'
)

// type WrapperOption func(*Wrapper)
//
// type SplitFunc func(rune) bool
//
// func ReplaceTab() WrapperOption {
// 	return func(w *Wrapper) {
// 		w.replaceTab = true
// 	}
// }
//
// func MergeBlanks() WrapperOption {
// 	return func(w *Wrapper) {
// 		w.mergeBlank = true
// 	}
// }
//
// func MergeNL() WrapperOption {
// 	return func(w *Wrapper) {
// 		w.mergeNL = true
// 	}
// }
//
// func Split(split SplitFunc) WrapperOption {
// 	return func(w *Wrapper) {
//     if split == nil {
//       return
//     }
// 		w.split = split
// 	}
// }
//
// type Wrapper struct {
// 	replaceTab bool
// 	mergeBlank bool
// 	mergeNL    bool
// 	split      SplitFunc
// 	size       int
// }
//
// func New(size int, options ...WrapperOption) Wrapper {
// 	w := Wrapper{
//     size: size,
//     split: isBlank,
//   }
// 	for _, o := range options {
// 		o(&w)
// 	}
// 	return &w
// }
//
// func (w Wrapper) Wrap(str string) string {
// 	return str
// }

func Wrap(str string) string {
	return WrapN(str, DefaultLength)
}

func WrapN(str string, n int) string {
	if len(str) < n {
		return str
	}
	var (
		ws  strings.Builder
		ptr int
	)
	for i := 0; ptr < len(str); i++ {
		if i > 0 {
			ws.WriteRune(nl)
		}
		_, x := advance(str[ptr:], n)
		if x == 0 {
			break
		}
		ws.WriteString(strings.TrimSpace(str[ptr : ptr+x]))
		ptr += x
	}
	return ws.String()
}

func advance(str string, n int) (string, int) {
	if len(str) == 0 {
		return "", 0
	}
	var (
		curr int
		prev int
		ws   strings.Builder
	)
	for {
		r, z := utf8.DecodeRuneInString(str[curr:])
		if r == utf8.RuneError {
			break
		}
		curr += z
		if isNL(r) {
			// skip(str[curr:], isNL)
			break
		}
		if isBlank(r) {
			// skip(str[curr:], isBlank)
			if curr == n || (curr > n && curr-n <= DefaultOffset) {
				break
			} else if curr > n && curr-n > DefaultOffset {
				curr = prev
				break
			}
			prev = curr
		}
		ws.WriteRune(r)
	}
	return ws.String(), curr
}

func skip(str string, fn func(rune) bool) int {
	var n int
	for {
		r, z := utf8.DecodeRuneInString(str[n:])
		if !fn(r) {
			break
		}
		n += z
	}
	return n
}

func isBlank(r rune) bool {
	return r == space || r == tab
}

func isNL(r rune) bool {
	return r == nl
}
