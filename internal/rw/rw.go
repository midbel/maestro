package rw

import (
	"io"
	"sync"
)

type lockWriter struct {
	mu    sync.Mutex
	inner io.Writer
}

func Lock(w io.Writer) io.Writer {
	if _, ok := w.(*lockWriter); ok {
		return w
	}
	return &lockWriter{
		inner: w,
	}
}

func (w *lockWriter) Write(b []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.inner.Write(b)
}
