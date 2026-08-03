package ico

import (
	"bytes"
	"errors"
	"image"
	"image/png"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func readPng(filename string) (image.Image, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return png.Decode(f)
}

func TestWriter(t *testing.T) {
	fn := "testdata/icondata.png"
	m0, err := readPng(fn)
	if err != nil {
		t.Error(fn, err)
	}

	icoimg, _ := os.Create("testdata/new.ico")
	defer icoimg.Close()

	err = Encode(icoimg, m0)
	if err != nil {
		t.Error(err)
	}
}

// failWriter fails after letting n bytes through, standing in for a full disk
// or a broken pipe partway into the file.
type failWriter struct {
	remaining int
	err       error
}

func (w *failWriter) Write(p []byte) (int, error) {
	if w.remaining <= 0 {
		return 0, w.err
	}
	if len(p) <= w.remaining {
		w.remaining -= len(p)
		return len(p), nil
	}
	n := w.remaining
	w.remaining = 0
	return n, w.err
}

// TestEncode_ReportsWriteFailure: Encode used to declare `var e error`, never
// assign it and return it, so it always reported success while discarding the
// errors from both destination writes. A truncated .ico was then packaged, or
// handed to the window manager, with nothing reported to the caller — and both
// call sites check the error, so they were checking a value that could not be
// non-nil. Found by the pre-v1 caliper audit.
func TestEncode_ReportsWriteFailure(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	boom := errors.New("disk full")

	// Failing on the very first write (the header).
	err := Encode(&failWriter{remaining: 0, err: boom}, img)
	assert.ErrorIs(t, err, boom, "a failed header write must be reported")

	// Failing partway, once the header is through and the PNG body starts.
	err = Encode(&failWriter{remaining: 22, err: boom}, img)
	assert.ErrorIs(t, err, boom, "a failed image write must be reported")
}

// A writer that accepts everything still succeeds, so the error plumbing did
// not make the happy path fail.
func TestEncode_SucceedsOnGoodWriter(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, Encode(&buf, image.NewRGBA(image.Rect(0, 0, 16, 16))))
	assert.NotEmpty(t, buf.Bytes())
}
