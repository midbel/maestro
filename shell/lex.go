package shell

import (
	"bufio"
	"errors"
	"io"
	"strings"
)

func Shlex(r io.Reader) ([]string, error) {
	var (
		rs  = bufio.NewReader(r)
		str []string
	)
	for {
		r, _, err := rs.ReadRune()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		var word string
		switch {
		case isNL(r) || isBlank(r):
			readBlank(rs)
			continue
		case isQuote(r):
			word = readQuote(rs, r)
		default:
			word = readWord(rs, r)
		}
		str = append(str, word)
	}
	return str, nil
}

func readWord(rs io.RuneScanner, r rune) string {
	var str strings.Builder
	str.WriteRune(r)
	for {
		r, _, err := rs.ReadRune()
		if isBlank(r) || isNL(r) || isQuote(r) || err != nil {
			break
		}
		str.WriteRune(r)
	}
	rs.UnreadRune()
	return str.String()
}

func readQuote(rs io.RuneReader, quote rune) string {
	var (
		str  strings.Builder
		prev rune
	)
	for {
		r, _, err := rs.ReadRune()
		if (r == quote && prev != backslash) || err != nil {
			break
		}
		prev = r
		str.WriteRune(r)
	}
	return str.String()
}

func readBlank(rs io.RuneScanner) {
	for {
		r, _, _ := rs.ReadRune()
		if !isNL(r) && !isBlank(r) {
			break
		}
	}
	rs.UnreadRune()
}
