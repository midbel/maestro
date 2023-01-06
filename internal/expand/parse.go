package expand

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
	"unicode/utf8"
)

func ExpandString(str string, res Resolver) ([]string, error) {
	return Expand(bytes.NewReader([]byte(str)), res)
}

func Expand(str io.Reader, res Resolver) ([]string, error) {
	es, err := parse(readFrom(str))
	if err != nil {
		return nil, err
	}
	var list []string
	for i := range es {
		str, err := es[i].Expand(res)
		if err != nil {
			return nil, err
		}
		list = append(list, str...)
	}
	return list, nil
}

func Parse(r io.Reader) ([]Expander, error) {
	return parse(readFrom(r))
}

func parse(rs *reader) ([]Expander, error) {
	var list []Expander
	for {
		c, err := rs.read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		ex, err := parseExpander(rs, c, isBlank)
		if err != nil {
			return nil, err
		}
		if ex != nil {
			list = append(list, ex)
		}
	}
	return list, nil
}

func parseExpander(rs *reader, char rune, done func(rune) bool) (Expander, error) {
	var (
		ex  Expander
		err error
	)
	switch {
	case isBlank(char):
	case isBrace(char):
		ex, err = parseBrace(rs, nil)
	case isDollar(char):
		ex, err = parseVariable(rs)
	case isQuote(char):
		ex, err = parseQuote(rs, char)
	default:
		ex, err = parseLiteral(rs, done)
	}
	return ex, err
}

func parseLiteral(rs *reader, done func(rune) bool) (Expander, error) {
	rs.unread()
	var (
		list []Expander
		str  bytes.Buffer
	)
	for {
		c, err := rs.read()
		if errors.Is(err, io.EOF) || done(c) {
			if done(c) {
				rs.unread()
			}
			break
		}
		if err != nil {
			return nil, err
		}
		if isQuote(c) || isDollar(c) {
			if str.Len() > 0 {
				list = append(list, createLiteral(str.String()))
				str.Reset()
			}
			var (
				ex  Expander
				err error
			)
			if isQuote(c) {
				ex, err = parseQuote(rs, c)
			} else {
				ex, err = parseVariable(rs)
			}
			if err != nil {
				return nil, err
			}
			list = append(list, ex)
			continue
		} else if isBrace(c) {
			var pre Expander
			if str.Len() > 0 {
				pre = createLiteral(str.String())
				str.Reset()
			}
			ex, err := parseBrace(rs, pre)
			if err != nil {
				return nil, err
			}
			list = append(list, ex)
			continue
		}
		str.WriteRune(c)
	}
	if str.Len() > 0 {
		list = append(list, createLiteral(str.String()))
	}
	return createList(list...), nil
}

func parseQuote(rs *reader, quote rune) (Expander, error) {
	var (
		list []Expander
		str  bytes.Buffer
	)
	for {
		c, err := rs.read()
		if err != nil {
			return nil, err
		}
		if c == quote {
			break
		}
		if isDollar(c) && isDouble(quote) {
			if str.Len() > 0 {
				list = append(list, createLiteral(str.String()))
				str.Reset()
			}
			ex, err := parseVariable(rs)
			if err != nil {
				return nil, err
			}
			list = append(list, ex)
			continue
		}
		str.WriteRune(c)
	}
	if str.Len() > 0 {
		list = append(list, createLiteral(str.String()))
	}
	return createList(list...), nil
}

func parseBrace(rs *reader, pre Expander) (Expander, error) {
	var (
		list []Expander
		done = func(c rune) bool { return c == comma || c == rcurly }
	)

	parse := func() (Expander, error) {
		c, _ := rs.read()
		if isBrace(c) {
			return parseBrace(rs, nil)
		} else if isQuote(c) {
			return parseQuote(rs, c)
		}
		rs.unread()
		return parseLiteral(rs, done)
	}
	for {
		ex, err := parse()
		if err != nil {
			return nil, err
		}
		list = append(list, ex)
		if c, _ := rs.read(); c == rcurly {
			break
		} else if c == comma {
			// do nothing
		} else {
			return nil, fmt.Errorf("braces: unexpected character %c", c)
		}
	}
	if len(list) == 0 {
		return nil, fmt.Errorf("braces: empty list")
	}
	var (
		post Expander
		err  error
	)
	if c, _ := rs.read(); c != comma {
		post, err = parseExpander(rs, c, done)
		if err != nil {
			return nil, err
		}
	} else {
		rs.unread()
	}
	return createBrace(list, pre, post), nil
}

func parseVariable(rs *reader) (Expander, error) {
	switch c, _ := rs.read(); {
	case isLetter(c):
		return parseIdent(rs)
	case isBrace(c):
		return parseExpansion(rs)
	default:
		return nil, fmt.Errorf("unexpected character %c", c)
	}
}

func parseIdent(rs *reader) (Expander, error) {
	rs.unread()
	var str bytes.Buffer
	for {
		c, err := rs.read()
		if errors.Is(err, io.EOF) || !isAlpha(c) {
			if !isAlpha(c) {
				rs.unread()
			}
			break
		}
		if err != nil {
			return nil, err
		}
		str.WriteRune(c)
	}
	if str.Len() == 0 {
		return nil, fmt.Errorf("variable: expansion too short")
	}
	return createVariable(str.String()), nil
}

func parseExpansion(rs *reader) (Expander, error) {
	var (
		c, _ = rs.read()
		ex   Expander
		err  error
	)
	switch {
	case isPound(c):
		ex, err = parseLength(rs)
	case isLetter(c):
		ex, err = parseTransform(rs)
		if err == nil {
			rs.unread()
		}
	default:
		err = fmt.Errorf("expansion: unexpected character %c", c)
	}
	if err != nil {
		return nil, err
	}
	if c, _ = rs.read(); c != rcurly {
		return nil, fmt.Errorf("expansion: unexpected character %c", c)
	}
	return ex, nil
}

func parseTransform(rs *reader) (Expander, error) {
	ex, err := parseIdent(rs)
	if err != nil {
		return nil, err
	}
	switch c, _ := rs.read(); {
	case c == slash:
		ex, err = parseReplace(rs, ex)
	case c == percent || c == pound:
		ex, err = parseStrip(rs, ex, c)
	case c == colon:
		ex, err = parseSubstring(rs, ex)
	case c == rcurly:
	default:
		return nil, fmt.Errorf("transform: unexpected character %c", c)
	}
	return ex, nil
}

func parseReplace(rs *reader, ex Expander) (Expander, error) {
	kind, _ := rs.read()
	switch kind {
	case percent, pound, slash:
	default:
		rs.unread()
	}
	src, err := parseString(rs, slash)
	if err != nil {
		return nil, err
	}
	dst, err := parseString(rs, rcurly)
	if err != nil {
		return nil, err
	}
	switch kind {
	case percent:
		ex = createReplaceSuffix(ex, src, dst)
	case pound:
		ex = createReplacePrefix(ex, src, dst)
	case slash:
		ex = createReplaceAll(ex, src, dst)
	default:
		ex = createReplaceFirst(ex, src, dst)
	}
	return ex, nil
}

func parseSubstring(rs *reader, ex Expander) (Expander, error) {
	var (
		off int
		siz int
		err error
	)
	if off, err = readNumber(rs); err != nil {
		return nil, err
	}
	switch c, _ := rs.read(); c {
	case rcurly:
		return createSubstring(ex, off, siz), nil
	case colon:
	default:
		return nil, fmt.Errorf("substring: unexpected character %c", c)
	}
	if siz, err = readNumber(rs); err != nil {
		return nil, err
	}
	return createSubstring(ex, off, siz), nil
}

func parseStrip(rs *reader, ex Expander, char rune) (Expander, error) {
	var longest bool
	if c, _ := rs.read(); c == char {
		longest = true
	} else {
		rs.unread()
	}
	var str bytes.Buffer
	for {
		c, err := rs.read()
		if err != nil {
			return nil, err
		}
		if c == rcurly {
			break
		}
		str.WriteRune(c)
	}
	if char == percent {
		ex = createSuffix(ex, str.String(), longest)
	} else {
		ex = createPrefix(ex, str.String(), longest)
	}
	return ex, nil
}

func parseLength(rs *reader) (Expander, error) {
	c, _ := rs.read()
	if !isLetter(c) {
		return nil, fmt.Errorf("length: unexpected character %c", c)
	}
	ex, err := parseIdent(rs)
	if err == nil {
		ex = createLength(ex)
	}
	return ex, err
}

func parseString(rs *reader, delim rune) (string, error) {
	str, err := parseUntil(rs, func(c rune) bool { return delim == c })
	if err == nil {
		rs.read()
	}
	return str, err
}

func parseUntil(rs *reader, done func(rune) bool) (string, error) {
	defer rs.unread()
	var str bytes.Buffer
	for {
		c, err := rs.read()
		if err != nil {
			return "", err
		}
		if done(c) {
			break
		}
		str.WriteRune(c)
	}
	return str.String(), nil
}

func readNumber(rs *reader) (int, error) {
	defer rs.unread()

	var str bytes.Buffer
	if c, _ := rs.read(); c == '-' {
		str.WriteRune(c)
	} else {
		rs.unread()
	}
	for {
		c, _ := rs.read()
		if !isDigit(c) {
			break
		}
		str.WriteRune(c)
	}
	return strconv.Atoi(str.String())
}

type reader struct {
	inner io.RuneScanner
}

func readFrom(r io.Reader) *reader {
	return &reader{
		inner: bufio.NewReader(r),
	}
}

func (r *reader) read() (rune, error) {
	c, _, err := r.inner.ReadRune()
	if errors.Is(err, io.EOF) {
		c = utf8.RuneError
	}
	return c, err
}

func (r *reader) unread() {
	r.inner.UnreadRune()
}

const (
	space     = ' '
	tab       = '\t'
	squote    = '\''
	dquote    = '"'
	backslash = '\\'
	nl        = '\n'
	cr        = '\r'
	dollar    = '$'
	lcurly    = '{'
	rcurly    = '}'
	pound     = '#'
	slash     = '/'
	percent   = '%'
	colon     = ':'
	comma     = ','
)

func isBrace(r rune) bool {
	return r == lcurly
}

func isAlpha(r rune) bool {
	return isLetter(r) || isDigit(r)
}

func isLetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_'
}

func isDigit(r rune) bool {
	return r >= '0' && r <= '9'
}

func isPound(r rune) bool {
	return r == pound
}

func isDollar(r rune) bool {
	return r == dollar
}

func isBlank(r rune) bool {
	return isSpace(r) || isNL(r) || r == utf8.RuneError
}

func isSpace(r rune) bool {
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
