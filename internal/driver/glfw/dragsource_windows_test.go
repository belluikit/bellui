//go:build windows

package glfw

import (
	"encoding/binary"
	"image"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/storage"
)

// Compile-time ABI assertions for the COM structs shared verbatim with the OS:
// each index must be 0, else the package fails to compile (#837).
var (
	_ = [1]struct{}{}[unsafe.Sizeof(formatEtc{})-32]
	_ = [1]struct{}{}[unsafe.Offsetof(formatEtc{}.ptd)-8]
	_ = [1]struct{}{}[unsafe.Offsetof(formatEtc{}.dwAspect)-16]
	_ = [1]struct{}{}[unsafe.Offsetof(formatEtc{}.lindex)-20]
	_ = [1]struct{}{}[unsafe.Offsetof(formatEtc{}.tymed)-24]
	_ = [1]struct{}{}[unsafe.Sizeof(stgMedium{})-24]
	_ = [1]struct{}{}[unsafe.Offsetof(stgMedium{}.hGlobal)-8]
	_ = [1]struct{}{}[unsafe.Offsetof(stgMedium{}.pUnkForRelease)-16]
)

// decodeWideList splits a CF_HDROP body (past the DROPFILES header) into its
// NUL-terminated UTF-16LE entries, dropping the empty final terminator.
func decodeWideList(t *testing.T, body []byte) []string {
	t.Helper()
	if len(body)%2 != 0 {
		t.Fatalf("odd-length UTF-16 body: %d bytes", len(body))
	}
	units := make([]uint16, len(body)/2)
	for i := range units {
		units[i] = binary.LittleEndian.Uint16(body[i*2:])
	}
	var out []string
	cur := make([]uint16, 0, 16)
	for _, u := range units {
		if u == 0 {
			if len(cur) > 0 {
				out = append(out, string(utf16Decode(cur)))
			}
			cur = cur[:0]
			continue
		}
		cur = append(cur, u)
	}
	return out
}

func utf16Decode(u []uint16) []rune {
	// The test paths are BMP-only, so a 1:1 unit→rune mapping is sufficient.
	r := make([]rune, len(u))
	for i, v := range u {
		r[i] = rune(v)
	}
	return r
}

func TestBuildDropFiles(t *testing.T) {
	buf := buildDropFiles([]string{"C:/Users/a.txt", "D:/b"})
	if assert.Greater(t, len(buf), dropFilesHeaderSize) {
		// Header: pFiles offset == header size, fWide == TRUE (Unicode).
		assert.Equal(t, uint32(dropFilesHeaderSize), binary.LittleEndian.Uint32(buf[0:]))
		assert.Equal(t, uint32(1), binary.LittleEndian.Uint32(buf[16:]))

		// The list decodes to native-separator paths, in order.
		paths := decodeWideList(t, buf[dropFilesHeaderSize:])
		assert.Equal(t, []string{`C:\Users\a.txt`, `D:\b`}, paths)

		// The whole list is double-NUL terminated (the last entry's NUL plus the
		// list NUL) — the final four bytes are two UTF-16 NULs.
		assert.Equal(t, []byte{0, 0, 0, 0}, buf[len(buf)-4:])
	}
}

func TestBuildDropFiles_EmptyWhenNoPaths(t *testing.T) {
	assert.Nil(t, buildDropFiles(nil))
	assert.Nil(t, buildDropFiles([]string{}))
}

func TestUTF16Bytes(t *testing.T) {
	// "hi" -> 'h',0x00,'i',0x00, then the terminating NUL 0x00,0x00.
	assert.Equal(t, []byte{'h', 0, 'i', 0, 0, 0}, utf16Bytes("hi"))
}

func TestFilePaths_SkipsNonFile(t *testing.T) {
	remote, err := storage.ParseURI("https://example.com/x")
	assert.NoError(t, err)
	got := filePaths([]bellui.URI{
		storage.NewFileURI("/tmp/a.txt"),
		remote,
		nil,
	})
	assert.Equal(t, []string{"/tmp/a.txt"}, got)
}

func TestBuildDragEntries_Flavors(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))

	formats, payloads := buildDragEntries(desktop.DragData{
		Text:  "hello",
		URIs:  []bellui.URI{storage.NewFileURI("/tmp/a.txt")},
		Image: img,
	})
	assert.Equal(t, len(formats), len(payloads), "formats and payloads stay index-parallel")

	haveHDrop, haveDIB, haveText := false, false, false
	for i, f := range formats {
		assert.Equal(t, dvAspectContent, f.dwAspect)
		assert.Equal(t, tymedHGlobal, f.tymed)
		assert.NotEmpty(t, payloads[i], "every declared flavor carries bytes")
		switch f.cfFormat {
		case cfHDrop:
			haveHDrop = true
		case cfDIB:
			haveDIB = true
		case cfUnicodeText:
			haveText = true
		}
	}
	assert.True(t, haveHDrop, "file list flavor present")
	assert.True(t, haveDIB, "CF_DIB image flavor present")
	assert.True(t, haveText, "CF_UNICODETEXT flavor present")
}

func TestBuildDragEntries_EmptyPayload(t *testing.T) {
	formats, payloads := buildDragEntries(desktop.DragData{})
	assert.Empty(t, formats)
	assert.Empty(t, payloads)
}
