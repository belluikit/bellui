package mobile

import (
	"archive/zip"
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every Android build links for 16 KB pages, after the user's own flags
// rather than instead of them (#626).
func TestAndroidLinkEnv(t *testing.T) {
	assert.Equal(t, []string{"CGO_LDFLAGS=-Wl,-z,max-page-size=16384"}, androidLinkEnv(""))
	assert.Equal(t, []string{"CGO_LDFLAGS=-L/opt/lib -lfoo -Wl,-z,max-page-size=16384"}, androidLinkEnv("-L/opt/lib -lfoo"))
}

// Each file in an APK starts on a 4-byte boundary and each native library on
// a 16 KB one, so a device with 16 KB pages can map the library from the
// archive (#626).
func TestAPKWriterAlignsContents(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	var buf bytes.Buffer
	w := NewWriter(&buf, key)
	for _, name := range []string{"AndroidManifest.xml", "classes.dex", "lib/arm64-v8a/libapp.so", "res/a.png", "lib/armeabi-v7a/libapp.so"} {
		fw, err := w.Create(name)
		require.NoError(t, err)
		_, err = fw.Write(bytes.Repeat([]byte{1}, 1001)) // an odd size, so every next start is unaligned
		require.NoError(t, err)
	}
	require.NoError(t, w.Close())

	data := buf.Bytes()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	require.NoError(t, err)
	libs := 0
	for _, f := range zr.File {
		off, err := f.DataOffset()
		require.NoError(t, err)
		align := int64(4)
		if strings.HasSuffix(f.Name, ".so") {
			align = 16384
			libs++
		}
		assert.Zero(t, off%align, "%s starts at %d", f.Name, off)
		assert.Equal(t, zip.Store, f.Method, f.Name)
	}
	assert.Equal(t, 2, libs)
}
