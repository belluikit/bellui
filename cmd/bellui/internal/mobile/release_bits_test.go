package mobile

import (
	"bytes"
	"testing"

	"github.com/belluikit/bellui/cmd/bellui/internal/templates"
	"github.com/stretchr/testify/assert"
)

// The #144 contract: the manifest's debuggable attribute follows the Debug
// bit, which goAndroidBuild derives from buildRelease — a --release package
// must not ship android:debuggable="true".
func TestAndroidManifestDebuggable(t *testing.T) {
	for _, tc := range []struct {
		debug bool
		want  string
	}{
		{debug: true, want: `android:debuggable="true"`},
		{debug: false, want: `android:debuggable="false"`},
	} {
		buf := &bytes.Buffer{}
		err := templates.ManifestAndroid.Execute(buf, manifestTmplData{
			JavaPkgPath: "com.example.testing",
			Name:        "Testing",
			LibName:     "testing",
			Debug:       tc.debug,
		})
		assert.NoError(t, err)
		assert.Contains(t, buf.String(), tc.want)
	}
}

// AppOutputName keys on distribution, not release: only `bellui release`
// produces the .aab store bundle; plain and --release packages are
// installable .apk files (#144). `bellui install` relies on this to find the
// artifact package actually wrote.
func TestAppOutputName(t *testing.T) {
	assert.Equal(t, "Testing.apk", AppOutputName("android", "Testing", false))
	assert.Equal(t, "Testing.aab", AppOutputName("android", "Testing", true))
	assert.Equal(t, "Testing.apk", AppOutputName("android/arm64", "Testing", false))
	assert.Equal(t, "Testing.app", AppOutputName("ios", "Testing", false))
	assert.Equal(t, "Testing.app", AppOutputName("ios", "Testing", true))
	assert.Equal(t, "", AppOutputName("windows", "Testing", true))
}
