// Serves the image from an in-process httptest server, which cannot listen
// inside a browser; the file/reader/URI-file image tests stay on the leg.
// (#77's split convention; #79 Phase B.)
//go:build !wasm

package canvas_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/storage"
	"github.com/stretchr/testify/assert"
)

func TestNewImageFromURI_HTTP(t *testing.T) {
	pwd, _ := os.Getwd()
	path := filepath.Join(filepath.Dir(pwd), "theme", "testdata", "bellui.png")
	f, _ := os.ReadFile(path)

	// start a test server to test http calls
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f, err := os.ReadFile(path)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Write(f)
	}))
	defer ts.Close()

	// http is mounted in Bellui test handlers by default
	url, _ := storage.ParseURI(ts.URL)
	img := canvas.NewImageFromURI(url)
	assert.NotNil(t, img)
	assert.Equal(t, "", img.File)
	assert.NotNil(t, img.Resource)
	assert.Equal(t, url.Authority(), img.Resource.Name())
	assert.Equal(t, f, img.Resource.Content())

	img.FillMode = canvas.ImageFillOriginal

	size := img.MinSize()
	assert.Equal(t, float32(512), size.Width)
	assert.Equal(t, float32(512), size.Height)
}
