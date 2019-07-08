package main

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

type includes []string

func (i *includes) Set(fs string) error {
	files := *i
	sort.Strings(files)
	for _, f := range strings.Split(fs, ",") {
		ix := sort.SearchStrings(files, f)
		if ix < len(files) && files[ix] == f {
			return fmt.Errorf("%s: already include", filepath.Base(f))
		}
		files = append(files[:ix], append([]string{f}, files[ix:]...)...)
	}
	*i = files
	return nil
}

func (i *includes) String() string {
	files := *i
	if len(files) == 0 {
		return ""
	}
	return strings.Join(files, ",")
}

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
