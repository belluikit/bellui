package mobile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An app built with the iOS 27 SDK traps in UIApplicationMain unless its
// Info.plist declares the scene life cycle (#1117), so the generated plist
// must carry a UIApplicationSceneManifest naming the runtime's scene delegate.
func TestInfoPlistSceneManifest(t *testing.T) {
	gen := generatedInfoPlist(t, iosInfoPlistData("com.example.t", "T", "1.0", 1, false, nil))
	plutilLint(t, gen)
	info, err := parsePlist(gen)
	require.NoError(t, err)

	manifest := info.get("UIApplicationSceneManifest")
	require.NotNil(t, manifest, "no UIApplicationSceneManifest: the app does not launch on iOS 27")
	assert.Equal(t, "false", manifest.get("UIApplicationSupportsMultipleScenes").Kind,
		"the runtime has one window, so one scene")

	configs := manifest.get("UISceneConfigurations")
	require.NotNil(t, configs)
	roles := configs.get("UIWindowSceneSessionRoleApplication")
	require.NotNil(t, roles)
	require.Equal(t, "array", roles.Kind)
	require.Len(t, roles.Items, 1)
	assert.Equal(t, iosSceneDelegateClass, roles.Items[0].get("UISceneDelegateClassName").Text)
}

// An app linked against a bellui without the scene delegate gets no manifest:
// one naming a missing class leaves UIKit with no window to show.
func TestInfoPlistNoSceneManifestWithoutDelegate(t *testing.T) {
	d := iosInfoPlistData("com.example.t", "T", "1.0", 1, false, nil)
	d.SceneDelegate = false
	gen := generatedInfoPlist(t, d)
	plutilLint(t, gen)
	info, err := parsePlist(gen)
	require.NoError(t, err)
	assert.Nil(t, info.get("UIApplicationSceneManifest"))
}

// The class the plist names must exist in the iOS runtime, or UIKit has no
// delegate to create the window with and the app shows nothing.
func TestSceneDelegateClassInRuntime(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "internal", "driver", "mobile", "app", "darwin_ios.m"))
	require.NoError(t, err)
	assert.True(t, strings.Contains(string(src),
		"@interface "+iosSceneDelegateClass+" : UIResponder<UIWindowSceneDelegate>"),
		"darwin_ios.m declares no %s window scene delegate", iosSceneDelegateClass)
	assert.True(t, strings.Contains(string(src), "@implementation "+iosSceneDelegateClass))
}
