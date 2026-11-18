package mobile

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An asset is deflated unless the Android Gradle plugin would leave it stored;
// everything outside assets/ stays stored (#1285).
func TestAPKMethod(t *testing.T) {
	for name, want := range map[string]uint16{
		"assets/THIRD_PARTY_NOTICES.txt": zip.Deflate,
		"assets/data/strings.json":       zip.Deflate,
		"assets/fonts/Inter.ttf":         zip.Deflate, // AGP compresses fonts too
		"assets/noext":                   zip.Deflate,
		"assets/a.png":                   zip.Store,
		"assets/photo.JPG":               zip.Store, // matched ignoring case, as AGP does
		"assets/song.mp3":                zip.Store,
		"assets/sound.ogg":               zip.Store,
		"assets/clip.mp4":                zip.Store,
		"assets/model.tflite":            zip.Store,
		"AndroidManifest.xml":            zip.Store,
		"classes.dex":                    zip.Store,
		"resources.arsc":                 zip.Store,
		"lib/arm64-v8a/libapp.so":        zip.Store,
		"res/mipmap-xxxhdpi-v4/icon.png": zip.Store,
		"res/xml/notices.txt":            zip.Store, // only assets/ changes
	} {
		assert.Equal(t, want, apkMethod(name), name)
	}
}

// Through addAssets, an app's text asset is deflated and its media stay
// stored, both read back byte for byte, and a stored file that follows a
// deflated one still starts on its 4-byte boundary (#1285).
func TestAddAssetsDeflatesTextAssets(t *testing.T) {
	prevTmp := tmpdir
	tmpdir = t.TempDir()
	t.Cleanup(func() { tmpdir = prevTmp })

	app := t.TempDir()
	notices := strings.Repeat("Copyright (c) The Authors. Licensed under the Apache License, Version 2.0.\n", 20000)
	files := map[string]string{
		"THIRD_PARTY_NOTICES.txt": notices,
		"a.png":                   "\x89PNG not really",
		"song.mp3":                "ID3 not really",
		"sound.ogg":               "OggS not really",
	}
	for name, body := range files {
		require.NoError(t, os.MkdirAll(filepath.Join(app, "assets"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(app, "assets", name), []byte(body), 0o644))
	}

	var buf bytes.Buffer
	apkw, err := buildAPK(&buf, nil, nil, nil, nil)
	require.NoError(t, err)
	require.NoError(t, addAssets(apkw, []byte(testAppManifest), app, "", 35, 1, "1", "", "", "", &preparedLibs{libs: &androidLibs{}}))
	require.NoError(t, apkw.Close())

	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	require.NoError(t, err)
	seen := 0
	for _, f := range zr.File {
		body, ok := files[strings.TrimPrefix(f.Name, "assets/")]
		if !ok {
			continue
		}
		seen++
		assert.Equal(t, body, readZipEntry(t, f), f.Name)
		if f.Name == "assets/THIRD_PARTY_NOTICES.txt" {
			assert.Equal(t, zip.Deflate, f.Method, f.Name)
			assert.Less(t, f.CompressedSize64*10, f.UncompressedSize64, "%s: %d of %d bytes", f.Name, f.CompressedSize64, f.UncompressedSize64)
			continue
		}
		assert.Equal(t, zip.Store, f.Method, f.Name)
		off, err := f.DataOffset()
		require.NoError(t, err)
		assert.Zero(t, off%4, "%s starts at %d", f.Name, off)
	}
	assert.Equal(t, len(files), seen)
}
