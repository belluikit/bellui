package mobile

import (
	"archive/zip"
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/cmd/bellui/internal/util"
)

// removingAppManifest is an app that takes the advertising ID permission out
// of the SDKs it bundles (belluikit/player#555), and a library's provider.
const removingAppManifest = `<?xml version="1.0" encoding="utf-8"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android"
	xmlns:tools="http://schemas.android.com/tools" package="com.example.app"
	android:versionCode="1" android:versionName="1.0">
	<application android:label="App" android:debuggable="true">
	<activity android:name="org.golang.app.GoNativeActivity" android:exported="true">
		<meta-data android:name="android.app.lib_name" android:value="app" />
	</activity>
	<provider android:name="androidx.startup.InitializationProvider" android:authorities="x">
		<meta-data android:name="androidx.emoji2.text.EmojiCompatInitializer" tools:node="remove" />
	</provider>
	<provider android:name="com.ads.Provider" tools:node="remove" />
	</application>
	<uses-permission android:name="android.permission.INTERNET" />
	<uses-permission android:name="com.google.android.gms.permission.AD_ID" tools:node="remove" />
	<uses-permission android:name="android.permission.VIBRATE" tools:node="merge" />
</manifest>`

// The app's marked elements leave its manifest, and the rest stays as
// written. Only those where libraries' elements merge are keys: a mark nested
// in the app's own provider has nothing of a library's to stop.
func TestWithoutRemovedElements(t *testing.T) {
	out, removed, err := withoutRemovedElements([]byte(removingAppManifest))
	require.NoError(t, err)

	want := removingAppManifest
	for _, line := range []string{
		"\t\t<meta-data android:name=\"androidx.emoji2.text.EmojiCompatInitializer\" tools:node=\"remove\" />\n",
		"\t<provider android:name=\"com.ads.Provider\" tools:node=\"remove\" />\n",
		"\t<uses-permission android:name=\"com.google.android.gms.permission.AD_ID\" tools:node=\"remove\" />\n",
	} {
		require.Contains(t, want, line)
		want = strings.Replace(want, line, "", 1)
	}
	assert.Equal(t, want, string(out))
	assert.Equal(t, map[string]bool{
		"uses-permission com.google.android.gms.permission.AD_ID": true,
		"provider com.ads.Provider":                               true,
	}, removed)

	same, none, err := withoutRemovedElements([]byte(testAppManifest))
	require.NoError(t, err)
	assert.Equal(t, testAppManifest, string(same), "a manifest with nothing marked is unchanged")
	assert.Empty(t, none)
}

// adsAAR is a manifest like play-services-ads-api's and
// play-services-ads-identifier's: both declare the AD_ID permission.
func adsAAR(pkg string, extra ...string) []byte {
	return []byte(`<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="` + pkg + `">
	<uses-permission android:name="com.google.android.gms.permission.AD_ID" />
	` + strings.Join(extra, "\n\t") + `
	<application><provider android:name="com.ads.Provider" android:authorities="${applicationId}.ads" /></application>
</manifest>`)
}

func writeAdsAARs(t *testing.T, dir string) {
	t.Helper()
	writeZip(t, filepath.Join(dir, "android", "libs", "play-services-ads-api.aar"), map[string][]byte{
		"AndroidManifest.xml": adsAAR("com.google.android.gms.ads",
			`<uses-permission android:name="android.permission.ACCESS_ADSERVICES_AD_ID" />`),
	})
	writeZip(t, filepath.Join(dir, "android", "libs", "play-services-ads-identifier.aar"), map[string][]byte{
		"AndroidManifest.xml": adsAAR("com.google.android.gms.ads.identifier"),
	})
}

// One app line removes a permission two libraries declare (#1168). What the
// app did not mark still merges, and no tools: attribute survives.
func TestMergeLibraryManifestsHonoursAppRemovals(t *testing.T) {
	dir := t.TempDir()
	writeAdsAARs(t, dir)
	libs, err := collectAndroidLibs(dir, t.TempDir())
	require.NoError(t, err)
	require.Len(t, libs.aars, 2)

	app, removed, err := withoutRemovedElements([]byte(removingAppManifest))
	require.NoError(t, err)
	merged, err := mergeLibraryManifests(app, libs, removed)
	require.NoError(t, err)
	m := string(merged)

	assert.NotContains(t, m, "permission.AD_ID\"", "neither library's AD_ID, nor the app's marker")
	assert.NotContains(t, m, "com.ads.Provider", "a marked <application> child is not merged either")
	assert.Contains(t, m, `<uses-permission android:name="android.permission.ACCESS_ADSERVICES_AD_ID"></uses-permission>`)
	assert.Contains(t, m, `"androidx.startup.InitializationProvider"`)
	assert.NotContains(t, m, "EmojiCompatInitializer")
	assert.Contains(t, m, `<uses-permission android:name="android.permission.VIBRATE" tools:node="merge" />`,
		"another directive is left as the app wrote it, as before")
	assert.Equal(t, 1, strings.Count(m, "tools:node"), "only that one: nothing merged from a library carries tools:")

	// Without the removals, the same libraries do bring AD_ID: the test
	// above measures the removal, not a library that lacks it.
	unmarked, err := mergeLibraryManifests([]byte(testAppManifest), libs, nil)
	require.NoError(t, err)
	assert.Equal(t, 1, strings.Count(string(unmarked), `"com.google.android.gms.permission.AD_ID"`))
}

// The packaged manifest, on both resource paths, has neither the removed
// permission nor its marker: aapt2 links it when there are AARs, binres
// compiles it when there are none.
func TestPackageAndroidRemovesMarkedPermission(t *testing.T) {
	_, buildTools := sdkTools(t)
	aapt2 := filepath.Join(buildTools, util.BuildToolFile("aapt2"))
	prevTmp := tmpdir
	tmpdir = t.TempDir()
	t.Cleanup(func() { tmpdir = prevTmp })

	app, removed, err := withoutRemovedElements([]byte(removingAppManifest))
	require.NoError(t, err)

	packaged := func(dir string, l *preparedLibs) string {
		apkPath := filepath.Join(t.TempDir(), "app.apk")
		var apk bytes.Buffer
		apkw, err := buildAPK(&apk, nil, nil, nil, nil)
		require.NoError(t, err)
		require.NoError(t, addAssets(apkw, app, dir, "", 35, 1, "1", "", "", "", l))
		require.NoError(t, apkw.Close())
		zr, err := zip.NewReader(bytes.NewReader(apk.Bytes()), int64(apk.Len()))
		require.NoError(t, err)
		require.NotEmpty(t, zr.File)
		require.NoError(t, os.WriteFile(apkPath, apk.Bytes(), 0o644))
		out, err := exec.Command(aapt2, "dump", "xmltree", "--file", "AndroidManifest.xml", apkPath).CombinedOutput()
		require.NoError(t, err, string(out))
		return string(out)
	}

	withAARs := t.TempDir()
	writeAdsAARs(t, withAARs)
	libs, err := prepareAndroidLibs(withAARs, app, removed, []string{"arm64"}, "app", "", "", "", "", 35, 1, "1")
	require.NoError(t, err)
	require.NotNil(t, libs.linked, "the aapt2 path")
	linked := packaged(withAARs, libs)
	assert.Contains(t, linked, `"android.permission.ACCESS_ADSERVICES_AD_ID"`, "the merge ran")
	assert.NotContains(t, linked, "permission.AD_ID\"")

	plain := t.TempDir()
	libs, err = prepareAndroidLibs(plain, app, removed, []string{"arm64"}, "app", "", "", "", "", 35, 1, "1")
	require.NoError(t, err)
	require.Nil(t, libs.linked, "the binres path")
	compiled := packaged(plain, libs)
	assert.Contains(t, compiled, `"android.permission.INTERNET"`, "the manifest was read")
	assert.NotContains(t, compiled, "permission.AD_ID\"")
}
