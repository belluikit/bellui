package mobile

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The generated Xcode project targets the iOS version -iosversion names, and
// that defaults to 15.0: Xcode 27 rejects anything older, and the project used
// to hardcode 14.0 whatever the flag said (#410).
func TestIOSProjectDeploymentTargetFollowsTheFlag(t *testing.T) {
	assert.Equal(t, "15.0", MinIOSVersion)
	assert.Equal(t, MinIOSVersion, cmdBuild.Flag.Lookup("iosversion").DefValue)

	prev := buildIOSVersion
	t.Cleanup(func() { buildIOSVersion = prev })
	for _, v := range []string{MinIOSVersion, "16.4"} {
		buildIOSVersion = v
		var proj bytes.Buffer
		require.NoError(t, projPbxprojTmpl.Execute(&proj, iosProjectData("com.example.app", "", "", "TEAM", true)))
		assert.Contains(t, proj.String(), "IPHONEOS_DEPLOYMENT_TARGET = "+v+";")
		assert.NotContains(t, proj.String(), "IPHONEOS_DEPLOYMENT_TARGET = 14.0;")
	}
}
