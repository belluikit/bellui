//go:build wasm

package glfw

import "github.com/belluikit/bellui"

func addMissingQuitForMainMenu(menus *bellui.MainMenu, w *window) *bellui.MainMenu {
	// no-op for a web browser
	return menus
}
