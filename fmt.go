package maestro

// import (
// 	"io"
// )

type formatter struct {
	frames []*frame
}

// func Format(w io.Writer, file string, is []string) error {
// 	return nil
// }

func (f *formatter) Format(file string) error {
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
