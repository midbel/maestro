package validate

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/midbel/slices"
)

type ValidateFunc func(string) error

const (
	ValidNot  = "not"
	ValidSome = "some"
	ValidAll  = "all"
)

var validations = map[string]func([]string) (ValidateFunc, error){
	"oneof":      validateOneOf,
	"noneof":     validateNoneOf,
	"notempty":   validateNotEmpty,
	"match":      validateMatch,
	"int":        validateInt,
	"float":      validateFloat,
	"eq":         validateEq,
	"ne":         validateNe,
	"gt":         validateGt,
	"ge":         validateGte,
	"lt":         validateLt,
	"le":         validateLte,
	"url":        validateUrl,
	"ip":         validateIp,
	"ipport":     validateIpPort,
	"exists":     validateFileExists,
	"file":       validateIsFile,
	"dir":        validateIsDir,
	"readable":   validateFileIsReadable,
	"writable":   validateFileIsWritable,
	"executable": validateFileIsExecutable,
}

func GetValidateFunc(name string, args []string) (ValidateFunc, error) {
	make, ok := validations[name]
	if !ok {
		return nil, fmt.Errorf("%s: unknown validation function", name)
	}
	return make(args)
}

func Get(name string, valid ...ValidateFunc) (ValidateFunc, error) {
	var (
		fn  ValidateFunc
		err error
	)
	switch name {
	case ValidAll:
		fn = All(valid...)
	case ValidNot:
		fn = Fail(All(valid...))
	case ValidSome:
		fn = Some(valid...)
	default:
		fn, err = GetValidateFunc(name, nil)
	}
	return fn, err
}

func Fail(valid ValidateFunc) ValidateFunc {
	return func(value string) error {
		err := valid(value)
		if err == nil {
			return fmt.Errorf("validation should fail but it pass")
		}
		return nil
	}
}

func Some(valid ...ValidateFunc) ValidateFunc {
	if len(valid) == 1 {
		return slices.Fst(valid)
	}
	return func(value string) error {
		for _, fn := range valid {
			if err := fn(value); err == nil {
				return err
			}
		}
		return fmt.Errorf("%s is not a valid value", value)
	}
}

func All(valid ...ValidateFunc) ValidateFunc {
	if len(valid) == 1 {
		return slices.Fst(valid)
	}
	return func(value string) error {
		for _, fn := range valid {
			if err := fn(value); err != nil {
				return err
			}
		}
		return nil
	}
}

func validateOneOf(args []string) (ValidateFunc, error) {
	if len(args) == 0 {
		return nil, noArg("oneof")
	}
	sort.Strings(args)
	fn := func(value string) error {
		i := sort.SearchStrings(args, value)
		if i >= len(args) || args[i] != value {
			return fmt.Errorf("only %s is accepted", strings.Join(args, ", "))
		}
		return nil
	}
	return fn, nil
}

func validateNoneOf(args []string) (ValidateFunc, error) {
	if len(args) == 0 {
		return nil, noArg("noneof")
	}
	sort.Strings(args)
	fn := func(value string) error {
		i := sort.SearchStrings(args, value)
		if i >= len(args) || args[i] != value {
			return nil
		}
		return fmt.Errorf("value can not be one of %s", strings.Join(args, ", "))
	}
	return fn, nil
}

func validateNotEmpty(args []string) (ValidateFunc, error) {
	if len(args) != 0 {
		return nil, tooManyArg("notempty", 0, len(args))
	}
	fn := func(value string) error {
		if value == "" {
			return fmt.Errorf("not empty value expected")
		}
		return nil
	}
	return fn, nil
}

func validateMatch(args []string) (ValidateFunc, error) {
	if len(args) == 0 {
		return nil, noArg("match")
	}
	r, err := regexp.Compile(args[0])
	if err != nil {
		return nil, err
	}
	fn := func(value string) error {
		ok := r.MatchString(value)
		if !ok {
			return fmt.Errorf("%s does not match pattern %s", value, args[0])
		}
		return nil
	}
	return fn, nil
}

func validateInt(args []string) (ValidateFunc, error) {
	if len(args) != 0 {
		return nil, tooManyArg("int", 0, len(args))
	}
	fn := func(value string) error {
		_, err := strconv.ParseInt(value, 0, 64)
		return err
	}
	return fn, nil
}

func validateFloat(args []string) (ValidateFunc, error) {
	if len(args) != 0 {
		return nil, tooManyArg("float", 0, len(args))
	}
	fn := func(value string) error {
		_, err := strconv.ParseFloat(value, 64)
		return err
	}
	return fn, nil
}

func validateEq(args []string) (ValidateFunc, error) {
	if len(args) == 0 {
		return nil, noArg("eq")
	}
	fn := func(value string) error {
		if value != args[0] {
			return fmt.Errorf("%s expected! got %s", args[0], value)
		}
		return nil
	}
	return fn, nil
}

func validateNe(args []string) (ValidateFunc, error) {
	if len(args) == 0 {
		return nil, noArg("ne")
	}
	fn := func(value string) error {
		if value == args[0] {
			return fmt.Errorf("%s is not accepted", value)
		}
		return nil
	}
	return fn, nil
}

func validateGt(args []string) (ValidateFunc, error) {
	if len(args) == 0 {
		return nil, noArg("gt")
	}
	fn := func(value string) error {
		if value <= args[0] {
			return fmt.Errorf("%s should be greater than %s", value, args[0])
		}
		return nil
	}
	return fn, nil
}

func validateGte(args []string) (ValidateFunc, error) {
	if len(args) == 0 {
		return nil, noArg("ge")
	}
	fn := func(value string) error {
		if value < args[0] {
			return fmt.Errorf("%s should be greater or equal than %s", value, args[0])
		}
		return nil
	}
	return fn, nil
}

func validateLt(args []string) (ValidateFunc, error) {
	if len(args) == 0 {
		return nil, noArg("lt")
	}
	fn := func(value string) error {
		if value >= args[0] {
			return fmt.Errorf("%s should be lesser than %s", value, args[0])
		}
		return nil
	}
	return fn, nil
}

func validateLte(args []string) (ValidateFunc, error) {
	if len(args) == 0 {
		return nil, noArg("le")
	}
	fn := func(value string) error {
		if value > args[0] {
			return fmt.Errorf("%s should be lesser or equal than %s", value, args[0])
		}
		return nil
	}
	return fn, nil
}

func validateUrl(args []string) (ValidateFunc, error) {
	if len(args) != 0 {
		return nil, tooManyArg("url", 0, len(args))
	}
	fn := func(value string) error {
		_, err := url.Parse(value)
		return err
	}
	return fn, nil
}

func validateIp(args []string) (ValidateFunc, error) {
	if len(args) != 0 {
		return nil, tooManyArg("ip", 0, len(args))
	}
	fn := func(value string) error {
		ip := net.ParseIP(value)
		if len(ip) == 0 {
			return fmt.Errorf("%s is not a valid IP address", value)
		}
		return nil
	}
	return fn, nil
}

func validateIpPort(args []string) (ValidateFunc, error) {
	if len(args) != 0 {
		return nil, tooManyArg("ipport", 0, len(args))
	}
	fn := func(value string) error {
		_, _, err := net.SplitHostPort(value)
		return err
	}
	return fn, nil
}

func validateFileExists(args []string) (ValidateFunc, error) {
	if len(args) != 0 {
		return nil, tooManyArg("exists", 0, len(args))
	}
	fn := func(value string) error {
		_, err := os.Stat(value)
		return err
	}
	return fn, nil
}

func validateIsFile(args []string) (ValidateFunc, error) {
	if len(args) != 0 {
		return nil, tooManyArg("file", 0, len(args))
	}
	fn := func(value string) error {
		i, err := os.Stat(value)
		if err != nil {
			return err
		}
		if !i.Mode().IsRegular() {
			return fmt.Errorf("%s is not a regular file", value)
		}
		return nil
	}
	return fn, nil
}

func validateIsDir(args []string) (ValidateFunc, error) {
	if len(args) != 0 {
		return nil, tooManyArg("dir", 0, len(args))
	}
	fn := func(value string) error {
		i, err := os.Stat(value)
		if err != nil {
			return err
		}
		if !i.Mode().IsDir() {
			return fmt.Errorf("%s is not a directory", value)
		}
		return nil
	}
	return fn, nil
}

func validateFileIsReadable(args []string) (ValidateFunc, error) {
	if len(args) != 0 {
		return nil, tooManyArg("readable", 0, len(args))
	}
	fn := func(value string) error {
		i, err := os.Stat(value)
		if err != nil {
			return err
		}
		var (
			perm  = i.Mode().Perm()
			owner = perm&0600 != 0
			group = perm&0060 != 0
			other = perm&0006 != 0
		)
		if owner || group || other {
			return nil
		}
		return fmt.Errorf("%s is not readable", value)
	}
	return fn, nil
}

func validateFileIsWritable(args []string) (ValidateFunc, error) {
	if len(args) != 0 {
		return nil, tooManyArg("writable", 0, len(args))
	}
	fn := func(value string) error {
		i, err := os.Stat(value)
		if err != nil {
			return err
		}
		var (
			perm  = i.Mode().Perm()
			owner = perm&0400 != 0
			group = perm&0040 != 0
			other = perm&0004 != 0
		)
		if owner || group || other {
			return nil
		}
		return fmt.Errorf("%s is not writable", value)
	}
	return fn, nil
}

func validateFileIsExecutable(args []string) (ValidateFunc, error) {
	if len(args) != 0 {
		return nil, tooManyArg("executable", 0, len(args))
	}
	fn := func(value string) error {
		i, err := os.Stat(value)
		if err != nil {
			return err
		}
		var (
			perm  = i.Mode().Perm()
			owner = perm&0100 != 0
			group = perm&0010 != 0
			other = perm&0001 != 0
		)
		if owner || group || other {
			return nil
		}
		return fmt.Errorf("%s is not executable", value)
	}
	return fn, nil
}

func noArg(name string) error {
	return fmt.Errorf("%s takes at least 1 argument! none were given", name)
}

func tooManyArg(name string, narg, given int) error {
	return fmt.Errorf("%s take %d argument(s)! %d was/were given", name, narg, given)
}
