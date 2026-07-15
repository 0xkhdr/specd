package worker

import (
	"bytes"
	"fmt"
	"io"
	"sync"
)

var outputMu sync.Mutex

type lineWriter struct {
	prefix string
	dst    io.Writer
	buf    []byte
}

func newLineWriter(prefix string, dst io.Writer) *lineWriter {
	return &lineWriter{prefix: prefix, dst: dst}
}

func (w *lineWriter) Write(p []byte) (int, error) {
	outputMu.Lock()
	defer outputMu.Unlock()
	w.buf = append(w.buf, p...)
	for {
		i := bytes.IndexByte(w.buf, '\n')
		if i < 0 {
			break
		}
		if _, err := fmt.Fprint(w.dst, w.prefix); err != nil {
			return 0, err
		}
		if _, err := w.dst.Write(w.buf[:i+1]); err != nil {
			return 0, err
		}
		w.buf = w.buf[i+1:]
	}
	return len(p), nil
}

// Flush emits any trailing partial line (no newline) so nothing is dropped when
// the worker exits.
func (w *lineWriter) Flush() {
	outputMu.Lock()
	defer outputMu.Unlock()
	if len(w.buf) > 0 {
		_, _ = fmt.Fprint(w.dst, w.prefix)
		_, _ = w.dst.Write(w.buf)
		_, _ = fmt.Fprintln(w.dst)
		w.buf = nil
	}
}
