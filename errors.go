package maestro

import (
	"github.com/midbel/distance"
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

func hasError(errs ...error) error {
	for _, e := range errs {
		if e != nil {
			return e
		}
	}
	return nil
}
