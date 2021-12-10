package wrap

import (
	"strings"
	"unicode/utf8"
)

const DefaultLength = 70

const (
	space = ' '
	tab   = '\t'
	nl    = '\n'
)

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
		x := advance(str[ptr:], n)
		if x == 0 {
			break
		}
		ws.WriteString(strings.TrimSpace(str[ptr : ptr+x]))
		ptr += x
	}
	return ws.String()
}

func advance(str string, n int) int {
	if len(str) == 0 {
		return 0
	}
	var (
		curr int
		prev int
	)
	for {
		r, z := utf8.DecodeRuneInString(str[curr:])
		if r == utf8.RuneError {
			break
		}
		curr += z
		if isNL(r) {
			break
		}
		if isBlank(r) {
			if curr == n {
				break
			} else if curr > n {
				curr = prev
				break
			}
			prev = curr
		}
	}
	return curr
}

func isBlank(r rune) bool {
	return r == space || r == tab
}

func isNL(r rune) bool {
	return r == nl
}
