package shell

import (
	"bufio"
	"io"
)

func Shlex(r io.Reader) ([]string, error) {
	var (
		scan = bufio.NewScanner(r)
		str  []string
	)
	scan.Split(bufio.ScanWords)
	for scan.Scan() {
		w := scan.Text()
		if w == "" {
			continue
		}
		str = append(str, w)
	}
	return str, scan.Err()
}
