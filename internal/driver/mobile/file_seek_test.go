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
