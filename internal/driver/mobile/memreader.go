//go:build !wasm

package mobile

import (
	"errors"
	"io"
	"sync"
)

// memReader serves a document from memory, reading it whole on first use. It
// is the fallback for a picked iOS document that cannot be opened by its path.
// ReadAt may be called concurrently, so load runs once however many first
// reads arrive at the same moment (#1031). Read and Seek share one offset and,
// as for a file, are not for concurrent use.
type memReader struct {
	load func() []byte

	once   sync.Once
	data   []byte
	offset int
}

func (m *memReader) bytes() []byte {
	m.once.Do(func() {
		m.data = m.load()
		m.load = nil
	})
	return m.data
}

func (m *memReader) Read(p []byte) (int, error) {
	n, err := m.ReadAt(p, int64(m.offset))
	m.offset += n
	return n, err
}

// ReadAt reads len(p) bytes at off, without moving the offset Read reads from.
func (m *memReader) ReadAt(p []byte, off int64) (int, error) {
	data := m.bytes()
	if off < 0 {
		return 0, errors.New("read at a negative offset")
	}
	if off >= int64(len(data)) {
		return 0, io.EOF
	}
	n := copy(p, data[off:])
	if n < len(p) {
		return n, io.EOF
	}
	return n, nil
}

func (m *memReader) Seek(offset int64, whence int) (int64, error) {
	pos, err := seekWithin(int64(m.offset), int64(len(m.bytes())), offset, whence)
	if err != nil {
		return int64(m.offset), err
	}
	m.offset = int(pos)
	return pos, nil
}
