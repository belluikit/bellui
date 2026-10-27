//go:build !wasm

package mobile

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/belluikit/bellui/storage"
)

// TestPickedFileSeeksWhenItsReaderCan pins #848: a picked document read
// through a file that can seek is offered to the caller as an io.Seeker, as
// on the desktop, and one read forward-only is not, since callers discover
// seeking by type assertion.
func TestPickedFileSeeksWhenItsReaderCan(t *testing.T) {
	path := filepath.Join(t.TempDir(), "song.mp3")
	if err := os.WriteFile(path, []byte("0123456789"), 0o600); err != nil {
		t.Fatal(err)
	}
	uri := storage.NewFileURI(path)

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	r := withSeek(&fileOpen{uri: uri}, f)
	defer r.Close()
	s, ok := r.(io.Seeker)
	if !ok {
		t.Fatal("a reader over a seekable file is not an io.Seeker")
	}
	if _, err := s.Seek(4, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	b := make([]byte, 3)
	if _, err := io.ReadFull(r, b); err != nil || string(b) != "456" {
		t.Errorf("read %q, %v after seeking to 4; want \"456\"", b, err)
	}
	if r.URI().String() != uri.String() {
		t.Errorf("URI %v; want %v", r.URI(), uri)
	}

	fw := withSeek(&fileOpen{uri: uri}, io.NopCloser(io.MultiReader()))
	if _, ok := fw.(io.Seeker); ok {
		t.Error("a forward-only reader is offered as an io.Seeker")
	}
}

// TestSeekWithin covers the in-memory seek the iOS picker's reader uses.
func TestSeekWithin(t *testing.T) {
	for _, tc := range []struct {
		cur, offset int64
		whence      int
		want        int64
		wantErr     bool
	}{
		{3, 5, io.SeekStart, 5, false},
		{3, 2, io.SeekCurrent, 5, false},
		{3, -4, io.SeekEnd, 6, false},
		{3, 0, io.SeekEnd, 10, false},
		{3, 50, io.SeekStart, 10, false}, // clamped to the end
		{3, -4, io.SeekCurrent, 3, true},
		{3, 0, 7, 3, true},
	} {
		got, err := seekWithin(tc.cur, 10, tc.offset, tc.whence)
		if got != tc.want || (err != nil) != tc.wantErr {
			t.Errorf("seekWithin(cur %d, size 10, %d, whence %d) = %d, %v; want %d, error %v",
				tc.cur, tc.offset, tc.whence, got, err, tc.want, tc.wantErr)
		}
	}
}
