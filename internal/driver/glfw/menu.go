package glfw

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
)

func buildMenuOverlay(menus *bellui.MainMenu, w *window) core.CanvasObject {
	if len(menus.Items) == 0 {
		bellui.LogError("Main menu must have at least one child menu", nil)
		return nil
	}

	menus = addMissingQuitForMainMenu(menus, w)
	return NewMenuBar(menus, w.canvas)
}
