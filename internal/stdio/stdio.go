package stdio

import (
	"io"
	"os"
	"sync"
)

var (
	Stdout = Lock(os.Stdout)
	Stderr = Lock(os.Stderr)
)

type lockedWriter struct {
	mu sync.Mutex
	io.Writer
}

func Lock(w io.Writer) io.Writer {
	return createLock(w)
}

func createLock(w io.Writer) io.Writer {
	return &lockedWriter{
		Writer: w,
	}
}

func (w *lockedWriter) Write(b []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.Writer.Write(b)
}

type nopWriterCloser struct {
	io.Writer
}

func NopCloser(w io.Writer) io.WriteCloser {
	return &nopWriterCloser{
		Writer: w,
	}
}

func (w *nopWriterCloser) Close() error {
	return nil
}
