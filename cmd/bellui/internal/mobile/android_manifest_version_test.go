package mobile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An app's own manifest carries the build's version, whatever numbers it
// states itself (#1114).
func TestWithManifestVersion(t *testing.T) {
	givenVersion(t, true, true)
	for name, tt := range map[string]struct{ in, want string }{
		"replaced": {
			`<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="a.b" android:versionCode="1" android:versionName="0.1">`,
			`<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="a.b" android:versionCode="7" android:versionName="2.3.4">`,
		},
		"single quotes and spacing": {
			"<manifest package='a.b'\n\tandroid:versionCode = '1'\n\tandroid:versionName='0.1'>",
			"<manifest package='a.b'\n\tandroid:versionCode = \"7\"\n\tandroid:versionName=\"2.3.4\">",
		},
		"added": {
			`<manifest package="a.b">`,
			`<manifest android:versionName="2.3.4" android:versionCode="7" package="a.b">`,
		},
		"already equal": {
			`<manifest package="a.b" android:versionCode="7" android:versionName="2.3.4">`,
			`<manifest package="a.b" android:versionCode="7" android:versionName="2.3.4">`,
		},
		"only the manifest tag": {
			`<manifest package="a.b"><application android:versionCode="1"/></manifest>`,
			`<manifest android:versionName="2.3.4" android:versionCode="7" package="a.b"><application android:versionCode="1"/></manifest>`,
		},
		"after the prolog and a comment": {
			`<?xml version="1.0"?><!-- app --><manifest package="a.b" android:versionCode="1">`,
			`<?xml version="1.0"?><!-- app --><manifest android:versionName="2.3.4" package="a.b" android:versionCode="7">`,
		},
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tt.want, string(withManifestVersion([]byte(tt.in), 7, "2.3.4")))
		})
	}

	assert.Equal(t, `<manifest android:versionName="1&amp;&lt;2" package="a.b">`,
		string(withManifestAttr([]byte(`<manifest package="a.b">`), "android:versionName", "1&<2", "-app-version")),
		"the value is escaped")
	assert.Equal(t, "no manifest here", string(withManifestVersion([]byte("no manifest here"), 7, "2.3.4")))

	// A defaulted version or build number leaves the manifest's alone.
	own := `<manifest package="a.b" android:versionCode="5" android:versionName="0.1">`
	givenVersion(t, false, false)
	assert.Equal(t, own, string(withManifestVersion([]byte(own), 1, "0.0.1")))
	givenVersion(t, true, false)
	assert.Equal(t, `<manifest package="a.b" android:versionCode="5" android:versionName="2.3.4">`,
		string(withManifestVersion([]byte(own), 1, "2.3.4")))
	givenVersion(t, false, true)
	assert.Equal(t, `<manifest package="a.b" android:versionCode="7" android:versionName="0.1">`,
		string(withManifestVersion([]byte(own), 7, "0.0.1")))
}

// givenVersion sets SetAndroidVersionGiven for the test.
func givenVersion(t *testing.T, name, code bool) {
	t.Helper()
	prevName, prevCode := buildVersionNameGiven, buildVersionCodeGiven
	t.Cleanup(func() { SetAndroidVersionGiven(prevName, prevCode) })
	SetAndroidVersionGiven(name, code)
}

// androidManifestData applies the version to the app's own manifest, which
// every resource path then compiles: aapt2 (adaptive icon, android/libs) and
// binres alike (#1114).
func TestAndroidManifestDataOwnManifestTakesTheBuildVersion(t *testing.T) {
	givenVersion(t, true, true)
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "AndroidManifest.xml"), []byte(`<?xml version="1.0" encoding="utf-8"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.example.v"
	android:versionCode="1" android:versionName="0.1">
	<application android:label="V">
		<activity android:name="org.golang.app.GoNativeActivity">
			<meta-data android:name="android.app.lib_name" android:value="v" />
		</activity>
	</application>
</manifest>
`), 0o644))
	data, lib, err := androidManifestData(dir, "com.example.v", "V", "v", "2.3.4", 7, "", "", "")
	require.NoError(t, err)
	assert.Equal(t, "v", lib)
	assert.Contains(t, string(data), `android:versionCode="7" android:versionName="2.3.4"`)
	assert.NotContains(t, string(data), `"0.1"`)
}
