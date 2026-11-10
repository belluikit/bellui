package mobile

import (
	"archive/zip"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The bundle states its native-library alignment rather than leaving it to
// the store's bundletool default (#1115).
func TestBundleConfigStates16KAlignment(t *testing.T) {
	var cfg struct {
		Optimizations struct {
			UncompressNativeLibraries struct {
				Enabled   bool   `json:"enabled"`
				Alignment string `json:"alignment"`
			} `json:"uncompressNativeLibraries"`
		} `json:"optimizations"`
	}
	require.NoError(t, json.Unmarshal([]byte(bundleConfig), &cfg))
	assert.True(t, cfg.Optimizations.UncompressNativeLibraries.Enabled)
	assert.Equal(t, "PAGE_ALIGNMENT_16K", cfg.Optimizations.UncompressNativeLibraries.Alignment)
}

// TestConvertAPKToAABAlignment runs the store-bundle step with the real
// bundletool: the bundle's config carries PAGE_ALIGNMENT_16K, and the APKs
// bundletool generates from it put the native library on a 16 KB boundary
// (#1115).
func TestConvertAPKToAABAlignment(t *testing.T) {
	sdkTools(t)
	if _, err := exec.LookPath("bundletool"); err != nil {
		t.Skip("bundletool not on PATH")
	}
	prevTmp, prevO := tmpdir, buildO
	tmpdir = spacedDir(t)
	t.Cleanup(func() { tmpdir, buildO = prevTmp, prevO })

	app := spacedDir(t)
	libs, err := prepareAndroidLibs(app, []byte(testAppManifest), []string{"arm64"}, "app", "", "", "", "", 35, 7, "1.2")
	require.NoError(t, err)
	goLib := "lib/arm64-v8a/libapp.so"
	require.NoError(t, os.MkdirAll(filepath.Join(tmpdir, "lib", "arm64-v8a"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpdir, goLib), []byte("go"), 0o644))

	out := spacedDir(t)
	buildO = filepath.Join(out, "app.aab")
	f, err := os.Create(filepath.Join(out, "app.apk"))
	require.NoError(t, err)
	apkw, err := buildAPK(f, nil, []string{goLib}, []string{"arm64"}, nil)
	require.NoError(t, err)
	require.NoError(t, addAssets(apkw, []byte(testAppManifest), app, "", 35, 7, "1.2", "", "", "", libs))
	require.NoError(t, apkw.Close())
	require.NoError(t, f.Close())

	require.NoError(t, convertAPKToAAB(buildO))
	for _, left := range []string{"base.zip", "BundleConfig.json", "tmpbundle"} {
		assert.NoFileExists(t, filepath.Join(out, left))
	}

	dump, err := exec.Command("bundletool", "dump", "config", "--bundle", buildO).CombinedOutput()
	require.NoError(t, err, string(dump))
	assert.Contains(t, string(dump), "PAGE_ALIGNMENT_16K")
	t.Logf("bundletool dump config:\n%s", dump)

	apks := filepath.Join(t.TempDir(), "app.apks")
	b, err := exec.Command("bundletool", "build-apks", "--bundle", buildO, "--output", apks).CombinedOutput()
	require.NoError(t, err, string(b))
	set, err := zip.OpenReader(apks)
	require.NoError(t, err)
	defer set.Close()
	var checked int
	for _, e := range set.File {
		if filepath.Ext(e.Name) != ".apk" {
			continue
		}
		p := filepath.Join(t.TempDir(), filepath.Base(e.Name))
		require.NoError(t, os.WriteFile(p, []byte(readZipEntry(t, e)), 0o644))
		apk, err := zip.OpenReader(p)
		require.NoError(t, err)
		for _, lib := range apk.File {
			if lib.Name != goLib {
				continue
			}
			off, err := lib.DataOffset()
			require.NoError(t, err)
			assert.Equal(t, zip.Store, lib.Method, "%s: %s is compressed", e.Name, goLib)
			assert.Zero(t, off%16384, "%s: %s at offset %d", e.Name, goLib, off)
			t.Logf("%s: %s at offset %d", e.Name, goLib, off)
			checked++
		}
		_ = apk.Close()
	}
	assert.NotZero(t, checked, "no generated APK holds %s", goLib)
}
