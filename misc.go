package maestro

import (
	"fmt"
	"strings"
)

func expandVariableInString(literal string, locals map[string][]string) (string, error) {
	var (
		b strings.Builder
		i int
	)

	for {
		ix := strings.Index(literal[i:], "%(")
		if ix < 0 {
			b.WriteString(literal[i:])
			break
		}
		i = ix + len("%(")
		if ix := strings.Index(literal[i:], ")"); ix < 0 {
			return "", fmt.Errorf("invalid syntax")
		} else {
			vs, ok := locals[literal[i:i+ix]]
			if !ok {
				return "", fmt.Errorf("%s: variable not set", literal[i:i+ix])
			}
			if len(vs) >= 1 {
				b.WriteString(vs[0])
			}
			i += ix + 1
		}
	}
	return b.String(), nil
}

func strUsage(u string) string {
	if u == "" {
		u = "no description available"
	}
	return u
}

func flatten(xs [][]string) []string {
	vs := make([]string, 0, len(xs)*4)
	for i := 0; i < len(xs); i++ {
		vs = append(vs, xs[i]...)
	}
	return vs
}
