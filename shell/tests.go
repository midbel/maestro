package shell

import (
	"errors"
	"fmt"
	"os"
	"strconv"
)

// ErrTest is the error value that Tester should returns when the
// tested criteria are not met
var ErrTest = errors.New("test")

var testops = map[string]rune{
	// binary operators
	"-eq": Eq,
	"-ne": Ne,
	"-lt": Lt,
	"-le": Le,
	"-gt": Gt,
	"-ge": Ge,
	"-nt": NewerThan,
	"-ot": OlderThan,
	"-ef": SameFile,
	// unary operators
	"-e": FileExists,
	"-r": FileRead,
	"-h": FileLink,
	"-d": FileDir,
	"-w": FileWrite,
	"-s": FileSize,
	"-f": FileRegular,
	"-x": FileExec,
	"-z": StrNotEmpty,
	"-n": StrEmpty,
}

type Tester interface {
	Expander
	Test(Environment) (bool, error)
}

type SingleTest struct {
	Expander
}

func (t SingleTest) Test(env Environment) (bool, error) {
	str, err := t.Expander.Expand(env, false)
	return len(str) > 0 && err == nil, nil
}

type UnaryTest struct {
	Op    rune
	Right Expander
}

func (t UnaryTest) Expand(env Environment, _ bool) ([]string, error) {
	ok, err := t.Test(env)
	return []string{strconv.FormatBool(ok)}, err
}

func (t UnaryTest) IsQuoted() bool {
	return false
}

func (t UnaryTest) Test(env Environment) (bool, error) {
	switch t.Op {
	case Not:
		ok, err := testExpander(t.Right, env)
		return !ok, err
	case FileExists:
		return t.fileExists(env)
	case FileSize:
		return t.fileSize(env)
	case FileRead:
		return t.fileReadable(env)
	case FileWrite:
		return t.fileWritable(env)
	case FileRegular:
		return t.fileRegular(env)
	case FileLink:
		return t.fileLink(env)
	case FileExec:
		return t.fileExec(env)
	case FileDir:
		return t.fileDirectory(env)
	case StrNotEmpty:
		str, err := expandSingle(t.Right, env)
		if err != nil {
			return false, err
		}
		return str != "", nil
	case StrEmpty:
		str, err := expandSingle(t.Right, env)
		if err != nil {
			return false, err
		}
		return str == "", nil
	default:
		return false, fmt.Errorf("unknown/unsupported unary test operator")
	}
}

func (t UnaryTest) fileExists(env Environment) (bool, error) {
	return statFileWith(t.Right, env, func(_ os.FileInfo) bool {
		return true
	})
}

func (t UnaryTest) fileReadable(env Environment) (bool, error) {
	return statFileWith(t.Right, env, func(fi os.FileInfo) bool {
		var (
			perm  = fi.Mode().Perm()
			owner = perm&0600 != 0
			group = perm&0060 != 0
			other = perm&0006 != 0
		)
		return owner || group || other
	})
}

func (t UnaryTest) fileWritable(env Environment) (bool, error) {
	return statFileWith(t.Right, env, func(fi os.FileInfo) bool {
		var (
			perm  = fi.Mode().Perm()
			owner = perm&0400 != 0
			group = perm&0040 != 0
			other = perm&0004 != 0
		)
		return owner || group || other
	})
}

func (t UnaryTest) fileSize(env Environment) (bool, error) {
	return statFileWith(t.Right, env, func(fi os.FileInfo) bool {
		return fi.Size() > 0
	})
}

func (t UnaryTest) fileRegular(env Environment) (bool, error) {
	return statFileWith(t.Right, env, func(fi os.FileInfo) bool {
		return fi.Mode().IsRegular()
	})
}

func (t UnaryTest) fileDirectory(env Environment) (bool, error) {
	return statFileWith(t.Right, env, func(fi os.FileInfo) bool {
		return fi.IsDir()
	})
}

func (t UnaryTest) fileLink(env Environment) (bool, error) {
	return statFileWith(t.Right, env, func(fi os.FileInfo) bool {
		return fi.Mode()&os.ModeSymlink == os.ModeSymlink
	})
}

func (t UnaryTest) fileExec(env Environment) (bool, error) {
	return statFileWith(t.Right, env, func(fi os.FileInfo) bool {
		var (
			perm  = fi.Mode().Perm()
			owner = perm&0100 != 0
			group = perm&0010 != 0
			other = perm&0001 != 0
		)
		return owner || group || other
	})
}

type BinaryTest struct {
	Op    rune
	Left  Expander
	Right Expander
}

func (t BinaryTest) Expand(env Environment, _ bool) ([]string, error) {
	ok, err := t.Test(env)
	return []string{strconv.FormatBool(ok)}, err
}

func (t BinaryTest) IsQuoted() bool {
	return false
}

func (t BinaryTest) Test(env Environment) (bool, error) {
	switch t.Op {
	case And:
		ok, err := testExpander(t.Left, env)
		if err != nil || !ok {
			return ok, err
		}
		return testExpander(t.Right, env)
	case Or:
		ok, err := testExpander(t.Left, env)
		if err == nil && ok {
			return ok, err
		}
		if err != nil {
			return false, err
		}
		return testExpander(t.Right, env)
	case Eq:
		return t.compare(env, func(left, right string) bool {
			return left == right
		})
	case Ne:
		return t.compare(env, func(left, right string) bool {
			return left != right
		})
	case Lt:
		return t.compare(env, func(left, right string) bool {
			return left < right
		})
	case Le:
		return t.compare(env, func(left, right string) bool {
			return left <= right
		})
	case Gt:
		return t.compare(env, func(left, right string) bool {
			return left > right
		})
	case Ge:
		return t.compare(env, func(left, right string) bool {
			return left >= right
		})
	case SameFile:
		return t.sameFile(env)
	case OlderThan:
		return t.olderThan(env)
	case NewerThan:
		return t.newerThan(env)
	default:
		return false, fmt.Errorf("unknown/unsupported unary test operator")
	}
}

func (t BinaryTest) compare(env Environment, cmp func(left, right string) bool) (bool, error) {
	left, err := expandSingle(t.Left, env)
	if err != nil {
		return false, err
	}
	right, err := expandSingle(t.Right, env)
	if err != nil {
		return false, err
	}
	return cmp(left, right), nil
}

func (t BinaryTest) olderThan(env Environment) (bool, error) {
	left, err := statFile(t.Left, env)
	if err != nil {
		return false, err
	}
	right, err := statFile(t.Right, env)
	if err != nil {
		return false, err
	}
	return left.ModTime().Before(right.ModTime()), nil
}

func (t BinaryTest) newerThan(env Environment) (bool, error) {
	left, err := statFile(t.Left, env)
	if err != nil {
		return false, err
	}
	right, err := statFile(t.Right, env)
	if err != nil {
		return false, err
	}
	return left.ModTime().After(right.ModTime()), nil
}

func (t BinaryTest) sameFile(env Environment) (bool, error) {
	left, err := statFile(t.Left, env)
	if err != nil {
		return false, err
	}
	right, err := statFile(t.Right, env)
	if err != nil {
		return false, err
	}
	return os.SameFile(left, right), nil
}

func statFileWith(ex Expander, env Environment, stat func(os.FileInfo) bool) (bool, error) {
	fi, err := statFile(ex, env)
	if err != nil {
		return false, err
	}
	return stat(fi), nil
}

func statFile(ex Expander, env Environment) (os.FileInfo, error) {
	str, err := expandSingle(ex, env)
	if err != nil {
		return nil, err
	}
	return os.Stat(str)
}

func expandSingle(ex Expander, env Environment) (string, error) {
	str, err := ex.Expand(env, true)
	if err != nil {
		return "", err
	}
	if len(str) != 1 {
		return "", fmt.Errorf("%w: expected only 1 value to be expanded (got %dd)", ErrExpansion, len(str))
	}
	return str[0], nil
}

func testExpander(ex Expander, env Environment) (bool, error) {
	tester, ok := ex.(Tester)
	if !ok {
		return ok, fmt.Errorf("expander is not a tester (%#v)", ex)
	}
	return tester.Test(env)
}
