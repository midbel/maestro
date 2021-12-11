package wrap

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	DefaultLength    = 70
	DefaultThreshold = 2
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

func Shorten(str string, n int) string {
	if n <= 0 || n >= len(str) {
		return str
	}
	str, _ = advance(str, n)
	return fmt.Sprintf("%s...", str)
}

// func Indent(str string) string {
//   return str
// }
//
// func Dedent(str string) string {
//   return str
// }

func Wrap(str string) string {
	return WrapN(str, DefaultLength)
}

func WrapN(str string, n int) string {
	if n <= 0 {
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
		next, x := advance(str[ptr:], n)
		if x == 0 {
			break
		}
		ws.WriteString(strings.TrimSpace(next))
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
		if r != utf8.RuneError {
			curr += z
		}
		if isNL(r) {
			ws.WriteRune(nl)
			curr += skip(str[curr:], isNL)
			break
		}
		if isBlank(r) || r == utf8.RuneError {
			curr += skip(str[curr:], isBlank)
			if z := ws.Len(); z == n || (z > n && z-n < DefaultThreshold) {
				break
			} else if z > n && z-n > DefaultThreshold {
				curr = prev
				break
			}
			prev = curr
		}
		// if r != utf8.RuneError {
		// }
		ws.WriteRune(r)
	}
	str = ws.String()
	if z := len(str); z > curr {
		str = str[:curr]
	}
	return str, curr
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
