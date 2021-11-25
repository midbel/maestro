package shlex

import (
	"bufio"
	"bytes"
	"errors"
	"io"
)

type runeWriter interface {
	WriteRune(rune) (int, error)
}

func Split(r io.Reader) ([]string, error) {
	var (
		rs  = bufio.NewReader(r)
		buf bytes.Buffer
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
		switch {
		case isNL(r) || isBlank(r):
			readBlank(rs)
			continue
		case isDollar(r):
			readDollar(&buf, rs)
		case isQuote(r):
			readQuote(&buf, rs, r)
		case isDelimiter(r):
			readDelimiter(&buf, rs, r)
		default:
			readWord(&buf, rs, r)
		}
		str = append(str, buf.String())
		buf.Reset()
	}
	return str, nil
}

func readDollar(str runeWriter, rs io.RuneScanner) {
	if r, _, _ := rs.ReadRune(); r != lparen {
		rs.UnreadRune()
		readWord(str, rs, dollar)
		return
	}
	if r, _, _ := rs.ReadRune(); r == lparen {
		readArithmetic(str, rs)
		return
	}
	rs.UnreadRune()
	readSubstitution(str, rs)
}

func readSubstitution(str runeWriter, rs io.RuneScanner) {
	str.WriteRune(dollar)
	str.WriteRune(lparen)

	var (
		count int
		prev  rune
	)
	for {
		r, _, err := rs.ReadRune()
		if err != nil {
			return
		}
		if r == rparen {
			if count == 0 {
				break
			}
			count--
		}
		if prev == dollar && r == lparen {
			count++
		}
		prev = r
		str.WriteRune(r)
	}
	str.WriteRune(rparen)
}

func readArithmetic(str runeWriter, rs io.RuneScanner) {
	str.WriteRune(dollar)
	str.WriteRune(lparen)
	str.WriteRune(lparen)

	var count int
	for {
		r, _, err := rs.ReadRune()
		if err != nil {
			return
		}
		if r == rparen {
			if r, _, _ := rs.ReadRune(); r == rparen && count == 0 {
				break
			}
			count--
			rs.UnreadRune()
		}
		if r == lparen {
			count++
		}
		str.WriteRune(r)
	}
	str.WriteRune(rparen)
	str.WriteRune(rparen)
}

func readWord(str runeWriter, rs io.RuneScanner, r rune) {
	str.WriteRune(r)
	for {
		r, _, err := rs.ReadRune()
		if eow(r) || err != nil {
			break
		}
		str.WriteRune(r)
	}
	rs.UnreadRune()
}

func readDelimiter(str runeWriter, rs io.RuneScanner, r rune) {
	str.WriteRune(r)
	for {
		r, _, err := rs.ReadRune()
		if !isDelimiter(r) || err != nil {
			break
		}
		str.WriteRune(r)
	}
	rs.UnreadRune()
}

func readQuote(str runeWriter, rs io.RuneReader, quote rune) {
	var prev rune
	for {
		r, _, err := rs.ReadRune()
		if (r == quote && prev != backslash) || err != nil {
			break
		}
		prev = r
		str.WriteRune(r)
	}
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

const (
	ampersand = '&'
	pipe      = '|'
	semicolon = ';'
	space     = ' '
	tab       = '\t'
	squote    = '\''
	dquote    = '"'
	backslash = '\\'
	nl        = '\n'
	cr        = '\r'
	dollar    = '$'
	lparen    = '('
	rparen    = ')'
)

func eow(r rune) bool {
	return isDelimiter(r) || isQuote(r) || isBlank(r) || isNL(r)
}

func isDollar(r rune) bool {
	return r == dollar
}

func isDelimiter(r rune) bool {
	return r == ampersand || r == pipe || r == semicolon
}

func isBlank(r rune) bool {
	return r == space || r == tab
}

func isDouble(r rune) bool {
	return r == dquote
}

func isSingle(r rune) bool {
	return r == squote
}

func isQuote(r rune) bool {
	return isDouble(r) || isSingle(r)
}

func isNL(r rune) bool {
	return r == cr || r == nl
}
