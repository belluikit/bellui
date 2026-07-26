//go:build !darwin || no_native_menus

package glfw

import "github.com/belluikit/bellui"

func setupNativeMenu(_ *window, _ *bellui.MainMenu) {
	// no-op
}

// ensureRegularActivationPolicy is a no-op away from the macOS native-menu build:
// on other platforms it is meaningless, and under -tags no_native_menus GLFW keeps
// its own menu bar and still performs the activation-policy promotion itself.
func ensureRegularActivationPolicy() {}

// disableGLFWMenubar is a no-op on platforms other than macOS-with-native-menus.
// Only the macOS native-menu build tells GLFW to skip building its own menu bar
// (see menu_darwin.go / #831).
func disableGLFWMenubar() {}

// ensureBelluiMainMenu is a no-op here; only the macOS native-menu build constructs
// bellui's own main menu bar (see menu_darwin.go / #831).
func ensureBelluiMainMenu() {}
