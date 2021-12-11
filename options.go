package maestro

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Dirs struct {
	List []string
}

func (d *Dirs) Set(str string) error {
	if i, err := os.Stat(str); err != nil || !i.IsDir() {
		return fmt.Errorf("%s is not a directory", str)
	}
	d.List = append(d.List, str)
	return nil
}

func (d *Dirs) String() string {
	if len(d.List) == 0 {
		return "directories"
	}
	return strings.Join(d.List, ", ")
}

func (d *Dirs) Exists(file string) (string, bool) {
	for i := range d.List {
		f := filepath.Join(d.List[i], file)
		if i, err := os.Stat(f); err == nil && i.Mode().IsRegular() {
			return f, true
		}
	}
	i, err := os.Stat(file)
	return file, err == nil && i.Mode().IsRegular()
}
