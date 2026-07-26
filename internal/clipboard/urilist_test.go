package clipboard

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEncodeURIList(t *testing.T) {
	data := EncodeURIList([]string{"/tmp/plain.txt", "/tmp/with space/ü.png"})
	assert.Equal(t,
		"file:///tmp/plain.txt\r\nfile:///tmp/with%20space/%C3%BC.png\r\n",
		string(data))

	assert.Empty(t, EncodeURIList(nil))
}

func TestDecodeURIList(t *testing.T) {
	payload := "# comment line\r\n" +
		"file:///tmp/plain.txt\r\n" +
		"file:///tmp/with%20space/%C3%BC.png\r\n" +
		"https://example.com/not-a-file\r\n" +
		"\r\n" +
		"file://localhost/tmp/host.txt\r\n" +
		"file://otherhost/tmp/remote.txt\r\n"

	assert.Equal(t,
		[]string{"/tmp/plain.txt", "/tmp/with space/ü.png", "/tmp/host.txt"},
		DecodeURIList([]byte(payload)))
}

func TestDecodeURIList_LFOnly(t *testing.T) {
	// Some writers separate with bare LF; be liberal on read.
	assert.Equal(t,
		[]string{"/a", "/b"},
		DecodeURIList([]byte("file:///a\nfile:///b")))
}

func TestGnomeCopiedFilesRoundTrip(t *testing.T) {
	paths := []string{"/tmp/a.txt", "/tmp/dir with space/b.png"}
	data := EncodeGnomeCopiedFiles("copy", paths)
	assert.Equal(t,
		"copy\nfile:///tmp/a.txt\nfile:///tmp/dir%20with%20space/b.png",
		string(data))

	op, back := DecodeGnomeCopiedFiles(data)
	assert.Equal(t, "copy", op)
	assert.Equal(t, paths, back)
}

func TestDecodeGnomeCopiedFiles_Cut(t *testing.T) {
	op, paths := DecodeGnomeCopiedFiles([]byte("cut\nfile:///x"))
	assert.Equal(t, "cut", op)
	assert.Equal(t, []string{"/x"}, paths)
}

func TestDecodeGnomeCopiedFiles_Empty(t *testing.T) {
	op, paths := DecodeGnomeCopiedFiles(nil)
	assert.Equal(t, "", op)
	assert.Nil(t, paths)
}

func TestDecodeGnomeCopiedFiles_RejectsForeignPayload(t *testing.T) {
	// Promiscuous owners (xclip) serve their text/uri-list bytes for any
	// requested target; the first URI must not be eaten as the operation.
	op, paths := DecodeGnomeCopiedFiles([]byte("file:///tmp/a.txt\r\nfile:///tmp/b.txt\r\n"))
	assert.Equal(t, "", op)
	assert.Nil(t, paths)
}
