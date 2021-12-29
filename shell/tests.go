package shell

import (
	"errors"
	"fmt"
	"os"
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

// [[ -z file && (file -nt other || file -ef other) ]]
// [[ ! -z file && !(file -nt other || ! file -ef other) ]]
// -z file => unary tester
// file -nt other => binary tester
// file -ef other => binary tester
// ( x || y) => binary tester x.Test() || y.Test()
// z && (x || y) => z.Test() && (x.Test() || y.Test())

type Tester interface {
	Test(Environment) (bool, error)
}

type UnaryTest struct {
	Op    rune
	Right Expander
}

func (t UnaryTest) Expand(env Environment, _ bool) ([]string, error) {
	return nil, nil
}

func (t UnaryTest) IsQuoted() bool {
	return false
}

func (t UnaryTest) Test(env Environment) (bool, error) {
	switch t.Op {
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
	case StrEmpty:
	default:
	}
	return false, ErrTest
}

func (t UnaryTest) fileExists(env Environment) (bool, error) {
	return statFileWith(t.Right, env, func(_ os.FileInfo) (bool, error) {
		return true, nil
	})
}

func (t UnaryTest) fileReadable(env Environment) (bool, error) {
	return statFileWith(t.Right, env, func(fi os.FileInfo) (bool, error) {
		var (
			perm  = fi.Mode().Perm()
			owner = perm&0600 != 0
			group = perm&0060 != 0
			other = perm&0006 != 0
		)
		return owner || group || other, nil
	})
}

func (t UnaryTest) fileWritable(env Environment) (bool, error) {
	return statFileWith(t.Right, env, func(fi os.FileInfo) (bool, error) {
		var (
			perm  = fi.Mode().Perm()
			owner = perm&0400 != 0
			group = perm&0040 != 0
			other = perm&0004 != 0
		)
		return owner || group || other, nil
	})
}

func (t UnaryTest) fileSize(env Environment) (bool, error) {
	return statFileWith(t.Right, env, func(fi os.FileInfo) (bool, error) {
		return fi.Size() > 0, nil
	})
}

func (t UnaryTest) fileRegular(env Environment) (bool, error) {
	return statFileWith(t.Right, env, func(fi os.FileInfo) (bool, error) {
		return fi.Mode().IsRegular(), nil
	})
}

func (t UnaryTest) fileDirectory(env Environment) (bool, error) {
	return statFileWith(t.Right, env, func(fi os.FileInfo) (bool, error) {
		return fi.IsDir(), nil
	})
}

func (t UnaryTest) fileLink(env Environment) (bool, error) {
	return statFileWith(t.Right, env, func(fi os.FileInfo) (bool, error) {
		return fi.Mode()&os.ModeSymlink == os.ModeSymlink, nil
	})
}

func (t UnaryTest) fileExec(env Environment) (bool, error) {
	return statFileWith(t.Right, env, func(fi os.FileInfo) (bool, error) {
		var (
			perm  = fi.Mode().Perm()
			owner = perm&0100 != 0
			group = perm&0010 != 0
			other = perm&0001 != 0
		)
		return owner || group || other, nil
	})
}

type BinaryTest struct {
	Op    rune
	Left  Expander
	Right Expander
}

func (t BinaryTest) Expand(env Environment, _ bool) ([]string, error) {
	return nil, nil
}

func (t BinaryTest) IsQuoted() bool {
	return false
}

func (t BinaryTest) Test(env Environment) (bool, error) {
	switch t.Op {
	case Eq:
	case Ne:
	case Lt:
	case Le:
	case Gt:
	case Ge:
	case SameFile:
		return t.sameFile(env)
	case OlderThan:
		return t.olderThan(env)
	case NewerThan:
		return t.newerThan(env)
	default:
	}
	return false, ErrTest
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

func statFileWith(ex Expander, env Environment, check func(os.FileInfo) (bool, error)) (bool, error) {
	fi, err := statFile(ex, env)
	if err != nil {
		return false, err
	}
	return check(fi)
}

func statFile(ex Expander, env Environment) (os.FileInfo, error) {
	str, err := ex.Expand(env, true)
	if err != nil {
		return nil, err
	}
	if len(str) != 1 {
		return nil, fmt.Errorf("%w: expected only 1 value to be expanded (got %dd)", ErrExpansion, len(str))
	}
	return os.Stat(str[0])
}
