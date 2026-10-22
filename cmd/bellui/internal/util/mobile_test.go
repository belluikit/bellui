package util

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_IsMobile(t *testing.T) {
	osList := []string{"darwin", "linux", "windows", "android/arm", "android/arm64", "android/amd64", "android/386", "android", "ios"}
	osIsMobile := []bool{false, false, false, true, true, true, true, true, true}

	assert.Equal(t, IsMobile(""), false)

	for i, os := range osList {
		assert.Equal(t, IsMobile(os), osIsMobile[i])
	}

	assert.Equal(t, IsMobile("amiga"), false)
}

func Test_IsAndroid(t *testing.T) {
	osList := []string{"darwin", "linux", "windows", "android/arm", "android/arm64", "android/amd64", "android/386", "android", "ios"}
	osIsMobile := []bool{false, false, false, true, true, true, true, true, false}

	assert.Equal(t, IsAndroid(""), false)

	for i, os := range osList {
		assert.Equal(t, IsAndroid(os), osIsMobile[i])
	}

	assert.Equal(t, IsAndroid("amiga"), false)
}

// The newest build-tools wins, compared numerically: an old d8 cannot read
// the newest platform's android.jar (#672 found 30.0.3 picked over 35.0.0).
func TestAndroidBuildToolsPathPicksNewest(t *testing.T) {
	home := t.TempDir()
	for _, v := range []string{"30.0.3", "35.0.0", "35.0.0-rc1", "9.0.0", "34.0.0"} {
		if err := os.MkdirAll(filepath.Join(home, "build-tools", v), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("ANDROID_HOME", home)
	if got, want := AndroidBuildToolsPath(), filepath.Join(home, "build-tools", "35.0.0"); got != want {
		t.Errorf("AndroidBuildToolsPath() = %s, want %s", got, want)
	}
}
