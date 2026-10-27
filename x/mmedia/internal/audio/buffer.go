package audio

import (
	"errors"
	"io"
	"sync"
)

// RingBuffer is a thread-safe circular buffer for PCM audio samples.
// It implements io.Reader for consumption by oto.Player.
type RingBuffer struct {
	mu   sync.Mutex
	buf  []byte
	r, w int
	full bool

	// totalRead tracks total bytes consumed, used for audio clock.
	totalRead int64
}

func NewRingBuffer(size int) *RingBuffer {
	return &RingBuffer{
		buf: make([]byte, size),
	}
}

// Write adds data to the buffer. Returns the number of bytes written.
// If the buffer is full, it drops the oldest data to make room.
func (rb *RingBuffer) Write(p []byte) (int, error) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	n := len(p)
	for i := 0; i < n; i++ {
		rb.buf[rb.w] = p[i]
		rb.w = (rb.w + 1) % len(rb.buf)
		if rb.full {
			rb.r = (rb.r + 1) % len(rb.buf) // overwrite oldest
		}
		rb.full = rb.w == rb.r
	}
	return n, nil
}

// Read reads data from the buffer into p.
// Returns 0 if the buffer is empty (non-blocking).
func (rb *RingBuffer) Read(p []byte) (int, error) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	avail := rb.available()
	if avail == 0 {
		// Return silence to keep audio output running smoothly.
		for i := range p {
			p[i] = 0
		}
		return len(p), nil
	}

	n := len(p)
	if n > avail {
		n = avail
	}

	for i := 0; i < n; i++ {
		p[i] = rb.buf[rb.r]
		rb.r = (rb.r + 1) % len(rb.buf)
	}
	rb.full = false
	rb.totalRead += int64(n)

	// Fill remainder with silence if we didn't have enough data.
	for i := n; i < len(p); i++ {
		p[i] = 0
	}

	return len(p), nil
}

// Available returns the number of bytes available to read.
func (rb *RingBuffer) Available() int {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	return rb.available()
}

func (rb *RingBuffer) available() int {
	if rb.full {
		return len(rb.buf)
	}
	if rb.w >= rb.r {
		return rb.w - rb.r
	}
	return len(rb.buf) - rb.r + rb.w
}

// TotalRead returns the total number of bytes that have been read from this buffer.
func (rb *RingBuffer) TotalRead() int64 {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	return rb.totalRead
}

// Reset clears the buffer.
func (rb *RingBuffer) Reset() {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.r = 0
	rb.w = 0
	rb.full = false
	rb.totalRead = 0
}

// Cap returns the total capacity of the buffer in bytes.
func (rb *RingBuffer) Cap() int {
	return len(rb.buf)
}

// Clear discards all buffered data without resetting the totalRead counter.
func (rb *RingBuffer) Clear() {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.r = 0
	rb.w = 0
	rb.full = false
}

// Seek lets oto's Player clear what it has pulled: Player.Seek empties its own
// buffer and then seeks its source, which needs the source to be an
// io.Seeker. Only Seek(0, io.SeekCurrent) is meaningful, and it moves
// nothing; it reports the bytes read so far. Clear is what discards the
// ring's own contents (#855).
func (rb *RingBuffer) Seek(offset int64, whence int) (int64, error) {
	if offset != 0 || whence != io.SeekCurrent {
		return 0, errors.New("audio: RingBuffer seeks only to where it is")
	}
	return rb.TotalRead(), nil
}
