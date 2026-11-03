package mobile

import (
	"archive/zip"
	"bytes"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/belluikit/bellui/cmd/bellui/internal/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An app's own manifest gets the adaptive icon named in it, as the generated
// one does, but an attribute its author set is kept (#920).
func TestWithAdaptiveIcon(t *testing.T) {
	got := string(withAdaptiveIcon([]byte(testAppManifest)))
	assert.Contains(t, got, `<application android:roundIcon="@mipmap/ic_launcher_round" android:icon="@mipmap/ic_launcher" android:label="App"`)

	own := `<manifest><application android:icon="@drawable/mine" android:label="App"></application></manifest>`
	got = string(withAdaptiveIcon([]byte(own)))
	assert.Contains(t, got, `android:icon="@drawable/mine"`)
	assert.NotContains(t, got, "@mipmap/ic_launcher\"")
	assert.Contains(t, got, `android:roundIcon="@mipmap/ic_launcher_round"`)

	both := `<manifest><application android:roundIcon="@drawable/r" android:icon="@drawable/i"></application></manifest>`
	assert.Equal(t, both, string(withAdaptiveIcon([]byte(both))))
}

// The adaptive branch names its resources in the manifest and writes a plain
// PNG under each name for API 21-25, which cannot read the adaptive XML.
func TestWriteAppIconResAdaptive(t *testing.T) {
	src := t.TempDir()
	legacy, fg := writeTestPNG(t, src, "Icon.png"), writeTestPNG(t, src, "Icon-foreground.png")

	res := filepath.Join(t.TempDir(), "res")
	manifest, err := writeAppIconRes(res, []byte(testAppManifest), legacy, fg, "", "")
	require.NoError(t, err)
	assert.Contains(t, string(manifest), `android:icon="@mipmap/ic_launcher"`)
	assert.Contains(t, string(manifest), `android:roundIcon="@mipmap/ic_launcher_round"`)
	for _, f := range []string{
		"mipmap-xxxhdpi/ic_launcher.png", "mipmap-xxxhdpi/ic_launcher_round.png",
		"mipmap-anydpi-v26/ic_launcher.xml", "mipmap-anydpi-v26/ic_launcher_round.xml",
	} {
		assert.FileExists(t, filepath.Join(res, f))
	}
	want, _ := os.ReadFile(legacy)
	got, _ := os.ReadFile(filepath.Join(res, "mipmap-xxxhdpi", "ic_launcher.png"))
	assert.Equal(t, want, got, "the legacy icon is the pre-26 fallback")

	// With no legacy icon the foreground layer stands in.
	res = filepath.Join(t.TempDir(), "res")
	_, err = writeAppIconRes(res, []byte(testAppManifest), "", fg, "", "")
	require.NoError(t, err)
	assert.FileExists(t, filepath.Join(res, "mipmap-xxxhdpi", "ic_launcher_round.png"))
}

// End to end through aapt2: an app with its own manifest and adaptive layers
// ships a manifest that names the icon, and the icon resolves both from API 26
// (the XML) and below it (the PNG).
func TestPackageAndroidOwnManifestAdaptiveIcon(t *testing.T) {
	_, buildTools := sdkTools(t)
	prevTmp := tmpdir
	tmpdir = t.TempDir()
	t.Cleanup(func() { tmpdir = prevTmp })

	app := t.TempDir()
	legacy := writeTestPNG(t, app, "Icon.png")
	writeTestPNG(t, app, "Icon-foreground.png")

	apkPath := filepath.Join(t.TempDir(), "app.apk")
	f, err := os.Create(apkPath)
	require.NoError(t, err)
	apkw, err := buildAPK(f, nil, nil, nil, nil)
	require.NoError(t, err)
	require.NoError(t, addAssets(apkw, []byte(testAppManifest), app, legacy, 35, 1, "1", "", "", "", &preparedLibs{libs: &androidLibs{}}))
	require.NoError(t, apkw.Close())
	require.NoError(t, f.Close())

	aapt2 := filepath.Join(buildTools, util.BuildToolFile("aapt2"))
	xmltree, err := exec.Command(aapt2, "dump", "xmltree", "--file", "AndroidManifest.xml", apkPath).CombinedOutput()
	require.NoError(t, err, string(xmltree))
	assert.Contains(t, string(xmltree), "android:icon(0x01010002)=@0x")
	assert.Contains(t, string(xmltree), "android:roundIcon(0x0101052c)=@0x")

	res, err := exec.Command(aapt2, "dump", "resources", apkPath).CombinedOutput()
	require.NoError(t, err, string(res))
	t.Logf("%s", res)
	for _, want := range []string{
		"(xxxhdpi) (file) res/mipmap-xxxhdpi-v4/ic_launcher.png",
		"(anydpi-v26) (file) res/mipmap-anydpi-v26/ic_launcher.xml",
		"(xxxhdpi) (file) res/mipmap-xxxhdpi-v4/ic_launcher_round.png",
		"(anydpi-v26) (file) res/mipmap-anydpi-v26/ic_launcher_round.xml",
	} {
		assert.Contains(t, string(res), want)
	}
	zr, err := zip.OpenReader(apkPath)
	require.NoError(t, err)
	defer zr.Close()
	var names bytes.Buffer
	for _, e := range zr.File {
		names.WriteString(e.Name + "\n")
	}
	assert.Contains(t, names.String(), "res/mipmap-xxxhdpi-v4/ic_launcher.png\n")
}

func writeTestPNG(t *testing.T, dir, name string) string {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, image.NewNRGBA(image.Rect(0, 0, 4, 4))))
	p := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(p, buf.Bytes(), 0o644))
	return p
}
