package maestro

import (
	"fmt"

	"github.com/midbel/distance"
	"github.com/midbel/maestro/internal/scan"
)

type SuggestionError struct {
	Others []string
	Err    error
}

func Suggest(err error, name string, names []string) error {
	names = distance.Levenshtein(name, names)
	if len(names) == 0 {
		return err
	}
	return SuggestionError{
		Err:    err,
		Others: names,
	}
}

func (s SuggestionError) Error() string {
	return s.Err.Error()
}

type UnexpectedError struct {
	Line     string
	Invalid  scan.Token
	Expected []string
}

func unexpected(token scan.Token, line string) error {
	return UnexpectedError{
		Line:    line,
		Invalid: token,
	}
}

func (e UnexpectedError) Error() string {
	str := e.Invalid.Literal
	if str == "" {
		str = e.Invalid.String()
	}
	return fmt.Sprintf("%s %q at %d:%d", errUnexpected, str, e.Invalid.Line, e.Invalid.Column)
}

func hasError(errs ...error) error {
	for _, e := range errs {
		if e != nil {
			return e
		}
	}
	return nil
}
