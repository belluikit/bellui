package mobile

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every place that states the iOS minimum — the generated Xcode project and
// the clang version-min flags — follows one setting, and that setting is 15.0:
// Xcode 27 rejects anything older, and the project used to hardcode 14.0
// whatever the setting said (#410).
func TestIOSDeploymentTargetIsOneSetting(t *testing.T) {
	assertTarget := func(t *testing.T, want string) {
		t.Helper()
		assert.Equal(t, want, IOSDeploymentTarget())
		var proj bytes.Buffer
		require.NoError(t, projPbxprojTmpl.Execute(&proj, iosProjectData("com.example.app", "", "", "TEAM", true)))
		assert.Contains(t, proj.String(), "IPHONEOS_DEPLOYMENT_TARGET = "+want+";")
		assert.NotContains(t, proj.String(), "IPHONEOS_DEPLOYMENT_TARGET = 14.0;")
		assert.Equal(t, " -miphoneos-version-min="+want, iosVersionMinFlag("iphoneos"))
		assert.Equal(t, " -mios-simulator-version-min="+want, iosVersionMinFlag("iphonesimulator"))
	}

	// What bellui package builds with: RunNewBuild swaps in an empty FlagSet
	// and never parses it, so the value is the one the flag's registration
	// wrote at init. Read it untouched.
	t.Run("default", func(t *testing.T) {
		assert.Equal(t, "15.0", MinIOSVersion)
		assert.Equal(t, MinIOSVersion, cmdBuild.Flag.Lookup("iosversion").DefValue)
		assertTarget(t, MinIOSVersion)
	})

	t.Run("set", func(t *testing.T) {
		prev := buildIOSVersion
		t.Cleanup(func() { buildIOSVersion = prev })
		buildIOSVersion = "16.4"
		assertTarget(t, "16.4")
	})
}
