package commands

import (
	"archive/tar"
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/cmd/bellui/internal/templates"
)

func writeTarZst(t *testing.T, entries map[string]string) string {
	t.Helper()
	var buf bytes.Buffer
	zw, err := zstd.NewWriter(&buf)
	require.NoError(t, err)
	tw := tar.NewWriter(zw)
	for name, content := range entries {
		require.NoError(t, tw.WriteHeader(&tar.Header{
			Name: name,
			Mode: 0o755,
			Size: int64(len(content)),
		}))
		_, err := tw.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())
	require.NoError(t, zw.Close())

	path := filepath.Join(t.TempDir(), "payload.tar.zst")
	require.NoError(t, os.WriteFile(path, buf.Bytes(), 0o644))
	return path
}

func TestExtractTarZst(t *testing.T) {
	archive := writeTarZst(t, map[string]string{
		"libcef.so":         "elf!",
		"locales/en-US.pak": "pak",
	})
	dest := filepath.Join(t.TempDir(), "cef")
	require.NoError(t, extractTarZst(archive, dest))

	got, err := os.ReadFile(filepath.Join(dest, "libcef.so"))
	require.NoError(t, err)
	assert.Equal(t, "elf!", string(got))
	_, err = os.Stat(filepath.Join(dest, "locales", "en-US.pak"))
	assert.NoError(t, err)
}

func TestExtractTarZst_RefusesEscape(t *testing.T) {
	archive := writeTarZst(t, map[string]string{"../evil": "x"})
	dest := filepath.Join(t.TempDir(), "cef")
	err := extractTarZst(archive, dest)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "escapes the payload directory")
	_, statErr := os.Stat(filepath.Join(filepath.Dir(dest), "evil"))
	assert.True(t, os.IsNotExist(statErr), "nothing may be written outside dest")
}

func TestNeedsCEFPayload_SelfBinary(t *testing.T) {
	// The test binary is a real Go binary that does not link gocef: the
	// detection must answer no without error, and still report an arch.
	exe, err := os.Executable()
	require.NoError(t, err)
	needs, arch := needsCEFPayload(exe)
	assert.False(t, needs)
	assert.NotEmpty(t, arch)
}

func TestMakefileTemplate_CEFBlocks(t *testing.T) {
	render := func(bundle bool) string {
		var buf bytes.Buffer
		require.NoError(t, templates.MakefileUNIX.Execute(&buf, unixData{
			Name: "App", AppID: "com.example.app", Exec: "app", Icon: "app.png",
			BundleCEF: bundle,
		}))
		return buf.String()
	}

	with := render(true)
	assert.Contains(t, with, "bin/cef")
	assert.Contains(t, with, "chmod 4755", "install must set up the sandbox helper")

	without := render(false)
	assert.NotContains(t, without, "bin/cef")
	assert.NotContains(t, without, "chrome-sandbox")
}
