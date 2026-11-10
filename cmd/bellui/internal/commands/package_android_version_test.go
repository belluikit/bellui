package commands

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"

	realUtil "github.com/belluikit/bellui/cmd/bellui/internal/util"
)

// `bellui package -os android --app-build 7 --app-version 2.3.4` of an app
// whose own AndroidManifest.xml says versionCode="1" versionName="1.0.0"
// gives a package that says 7 and 2.3.4, on each of the three ways its
// resources are compiled: binres (no adaptive icon, no android/res), aapt2
// for an adaptive icon, and aapt2 linking android/res. Without the flags (or
// manifest.pxf) the manifest's own numbers stay (#1114).
func TestPackageAndroidOwnManifestTakesTheBuildVersion(t *testing.T) {
	androidNDKOrSkip(t)
	aapt2, err := realUtil.Aapt2Path()
	require.NoError(t, err)

	given := []string{"--app-build", "7", "--app-version", "2.3.4"}
	for name, tt := range map[string]struct {
		extra      func(t *testing.T, dir string)
		flags      []string
		code, name string
	}{
		"binres": {func(*testing.T, string) {}, given, "7", "2.3.4"},
		"adaptive icon": {func(t *testing.T, dir string) {
			writeTestPNG(t, filepath.Join(dir, "Icon-foreground.png"))
		}, given, "7", "2.3.4"},
		"android/res": {func(t *testing.T, dir string) {
			values := filepath.Join(dir, "android", "res", "values")
			require.NoError(t, os.MkdirAll(values, 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(values, "strings.xml"),
				[]byte(`<resources><string name="greeting">hello</string></resources>`), 0o644))
		}, given, "7", "2.3.4"},
		"not given": {func(*testing.T, string) {}, nil, "1", "1.0.0"},
	} {
		t.Run(name, func(t *testing.T) {
			dir := ownManifestApp(t, ownAndroidManifest)
			tt.extra(t, dir)

			app := &cli.App{Name: "bellui", Commands: []*cli.Command{Package()}}
			require.NoError(t, app.Run(append([]string{
				"bellui", "package", "-os", "android/arm64", "--app-id", "com.example.credit", "--name", "Credit",
			}, tt.flags...)))

			badging, err := exec.Command(aapt2, "dump", "badging", filepath.Join(dir, "Credit.apk")).CombinedOutput()
			require.NoError(t, err, string(badging))
			pkg, _, _ := strings.Cut(string(badging), "\n")
			t.Log(pkg)
			assert.Contains(t, pkg, "versionCode='"+tt.code+"'")
			assert.Contains(t, pkg, "versionName='"+tt.name+"'")
		})
	}
}

func writeTestPNG(t *testing.T, path string) {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 108, 108))
	for i := range img.Pix {
		img.Pix[i] = 0xff
	}
	img.Set(54, 54, color.Black)
	f, err := os.Create(path)
	require.NoError(t, err)
	require.NoError(t, png.Encode(f, img))
	require.NoError(t, f.Close())
}
