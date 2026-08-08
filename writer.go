package main

import "fmt"

type CapWriter struct {
	Cap       uint64
	NoDiscard bool
	size      uint64
	buffer    []byte
}

func (w *CapWriter) Write(p []byte) (int, error) {
	size := w.size + uint64(len(p))
	if size > w.Cap && w.NoDiscard {
		return 0, fmt.Errorf("could not write body buffer: buffer is full")
	}

	// Always report consuming the whole slice so callers like io.Copy don't treat this as a short write.
	w.size = size

	// Keep only up to Cap bytes; discard the rest.
	if uint64(len(w.buffer)) < w.Cap {
		remain := w.Cap - uint64(len(w.buffer))
		remain = min(remain, uint64(len(p)))
		w.buffer = append(w.buffer, p[:int(remain)]...)
	}

	return len(p), nil
}

func (w *CapWriter) Size() uint64 {
	return w.size
}

func (w *CapWriter) Bytes() []byte {
	return w.buffer
}
