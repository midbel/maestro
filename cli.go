package main

func ParseShell(str string) []string {
	const (
		single byte = '\''
		double      = '"'
		space       = ' '
		equal       = '='
	)

	skipN := func(b byte) int {
		var i int
		if b == space || b == single || b == double {
			i++
		}
		return i
	}

	var (
		ps  []string
		j   int
		sep byte = space
	)
	for i := 0; i < len(str); i++ {
		if str[i] == sep || str[i] == equal {
			if i > j {
				j += skipN(str[j])
				ps, j = append(ps, str[j:i]), i+1
				if sep == single || sep == double {
					sep = space
				}
			}
			continue
		}
		if sep == space && (str[i] == single || str[i] == double) {
			sep, j = str[i], i+1
		}
	}
	if str := str[j:]; len(str) > 0 {
		i := skipN(str[0])
		ps = append(ps, str[i:])
	}
	return ps
}
