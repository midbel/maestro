package shell

import (
	"io"
)

type ShellOption func(*Shell) error

func WithStdout(w ...io.Writer) ShellOption {
	return func(s *Shell) error {
		s.SetStdout(io.MultiWriter(w...))
		return nil
	}
}

func WithStderr(w ...io.Writer) ShellOption {
	return func(s *Shell) error {
		s.SetStderr(io.MultiWriter(w...))
		return nil
	}
}

func WithEcho() ShellOption {
	return func(s *Shell) error {
		s.echo = true
		return nil
	}
}

func WithVar(ident string, values ...string) ShellOption {
	return func(s *Shell) error {
		if s.locals == nil {
			s.locals = EmptyEnv()
		}
		return s.locals.Define(ident, values)
	}
}

func WithAlias(ident, script string) ShellOption {
	return func(s *Shell) error {
		return s.Alias(ident, script)
	}
}

func WithCwd(dir string) ShellOption {
	return func(s *Shell) error {
		return s.Chdir(dir)
	}
}

func WithEnv(e Environment) ShellOption {
	return func(s *Shell) error {
		s.locals = EnclosedEnv(e)
		return nil
	}
}
