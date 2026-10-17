package mobile

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func withBuildTarget(t *testing.T, target string, tags ...string) {
	t.Helper()
	prevTarget, prevTags, prevRelease := buildTarget, buildTags, buildRelease
	buildTarget, buildTags, buildRelease = target, tags, false
	t.Cleanup(func() { buildTarget, buildTags, buildRelease = prevTarget, prevTags, prevRelease })
}

// A Simulator build must carry the iossimulator tag, or the wgpu-native
// libraries hand the linker the device archive (#10).
func TestGoBuildTags_Simulator(t *testing.T) {
	withBuildTarget(t, "iossimulator", "mine")
	tags, err := goBuildTags([]string{"GOOS=ios", "CGO_CFLAGS=-isysroot sim -mios-simulator-version-min=13.0 -arch arm64"})
	require.NoError(t, err)
	assert.Equal(t, []string{"mine", "ios", "iossimulator"}, tags)
	assert.Equal(t, stringsFlag{"mine"}, buildTags, "the flag's own slice is not appended to")
}

func TestGoBuildTags_DeviceAndAndroid(t *testing.T) {
	withBuildTarget(t, "ios")
	tags, err := goBuildTags([]string{"GOOS=ios", "CGO_CFLAGS=-isysroot dev -miphoneos-version-min=13.0 -arch arm64"})
	require.NoError(t, err)
	assert.Equal(t, []string{"ios"}, tags)

	withBuildTarget(t, "android")
	tags, err = goBuildTags([]string{"GOOS=android"})
	require.NoError(t, err)
	assert.Empty(t, tags)
}

// The tag follows the SDK env.go actually picks for each arch.
func TestGoBuildTags_FollowEnvInit(t *testing.T) {
	if !xcodeAvailable() {
		t.Skip("needs Xcode")
	}
	prevTmp := tmpdir
	tmpdir = t.TempDir() // envInit builds a small stub library there
	t.Cleanup(func() { tmpdir = prevTmp })
	for target, want := range map[string]bool{"iossimulator": true, "ios": false} {
		withBuildTarget(t, target)
		prevArchs := allArchs[target]
		require.NoError(t, envInit())
		for _, arch := range allArchs[target] {
			tags, err := goBuildTags(darwinEnv[arch])
			require.NoError(t, err)
			assert.Equal(t, want, contains(tags, "iossimulator"), "%s/%s: %v", target, arch, tags)
		}
		allArchs[target] = prevArchs
	}
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
