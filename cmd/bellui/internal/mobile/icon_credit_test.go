package mobile

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/cmd/bellui/internal/templates"
)

// The default icon's attribution reaches the generated Android manifest and
// iOS Info.plist, and nothing is added for an app with its own icon (#362).
func TestMobileManifestsCarryTheIconCredit(t *testing.T) {
	for _, credit := range []string{templates.GopherCredit, ""} {
		var manifest bytes.Buffer
		require.NoError(t, templates.ManifestAndroid.Execute(&manifest, manifestTmplData{
			JavaPkgPath: "com.example.notes", Name: "Notes", LibName: "notes", Credit: credit,
		}))
		var plist bytes.Buffer
		require.NoError(t, infoplistTmpl.Execute(&plist, infoplistTmplData{
			BundleID: "com.example.notes", Name: "Notes", Version: "1.0.0", Build: 1, Credit: credit,
		}))

		if credit == "" {
			assert.NotContains(t, manifest.String(), "bellui.icon_credit")
			assert.NotContains(t, plist.String(), "NSHumanReadableCopyright")
			continue
		}
		assert.Contains(t, manifest.String(), `<meta-data android:name="bellui.icon_credit" android:value="`+credit+`" />`)
		assert.Contains(t, plist.String(), "<key>NSHumanReadableCopyright</key>\n  <string>"+credit+"</string>")
	}
}

// The generated manifest takes the credit from the build settings that
// RunNewBuild fills, not only from a hand-built template value.
func TestGeneratedAndroidManifestCarriesTheIconCredit(t *testing.T) {
	prev := buildIconCredit
	buildIconCredit = templates.GopherCredit
	t.Cleanup(func() { buildIconCredit = prev })

	data, _, err := androidManifestData(t.TempDir(), "com.example.notes", "Notes", "notes", "1.0.0", 1, "", "", "")
	require.NoError(t, err)
	assert.Contains(t, string(data), `android:name="bellui.icon_credit"`)
}

// The generated iOS Info.plist takes the credit from the build settings.
func TestGeneratedIOSInfoPlistCarriesTheIconCredit(t *testing.T) {
	prev := buildIconCredit
	buildIconCredit = templates.GopherCredit
	t.Cleanup(func() { buildIconCredit = prev })

	var plist bytes.Buffer
	require.NoError(t, infoplistTmpl.Execute(&plist, iosInfoPlistData("com.example.notes", "Notes", "1.0.0", 1, false, nil)))
	assert.Contains(t, plist.String(), "<string>"+templates.GopherCredit+"</string>")
}
