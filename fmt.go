package maestro

import (
	"fmt"
	"io"
	"os"
)

type formatter struct {
	lex *lexer

	curr Token
	peek Token
}

func Format(r io.Reader, w io.Writer) error {
	x, err := Lex(r)
	if err != nil {
		return err
	}
	f := formatter{lex: x}
	f.nextToken()
	f.nextToken()

	return f.Format()
}

func (f *formatter) Format() error {
	for f.curr.Type != eof {
		fmt.Fprintln(os.Stdout, f.curr, f.peek)
	}
	return nil
}

func (f *formatter) formatActions() error {
	return nil
}

func (f *formatter) formatCommands() error {
	return nil
}

func (f *formatter) formatMeta() error {
	return nil
}

func (f *formatter) formatDeclarations() error {
	return nil
}

func (f *formatter) nextToken() {
	f.curr = f.peek
	f.peek = f.lex.Next()
}
