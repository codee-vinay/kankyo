package platform

import (
	"bytes"
	"sync"
)

type cappedBuffer struct {
	mu        sync.Mutex
	buf       bytes.Buffer
	limit     int
	truncated bool
}

func newCappedBuffer(limit int) *cappedBuffer {
	return &cappedBuffer{limit: limit}
}

func (w *cappedBuffer) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if remaining := w.limit - w.buf.Len(); remaining > 0 {
		if len(p) <= remaining {
			_, _ = w.buf.Write(p)
		} else {
			_, _ = w.buf.Write(p[:remaining])
			w.truncated = true
		}
	} else if len(p) > 0 {
		w.truncated = true
	}

	// Continue draining the pipe after the retention limit.
	return len(p), nil
}

func (w *cappedBuffer) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.String()
}

func (w *cappedBuffer) Truncated() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.truncated
}
