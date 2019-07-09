package maestro

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
