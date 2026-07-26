//go:build !wasm

package glfw

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/lang"
)

func addMissingQuitForMainMenu(menus *bellui.MainMenu, w *window) *bellui.MainMenu {
	localQuit := lang.L("Quit")
	var lastItem *bellui.MenuItem
	if len(menus.Items[0].Items) > 0 {
		lastItem = menus.Items[0].Items[len(menus.Items[0].Items)-1]
		if lastItem.Label == localQuit {
			lastItem.IsQuit = true
		}
	}
	if lastItem == nil || !lastItem.IsQuit { // make sure the first menu always has a quit option
		quitItem := bellui.NewMenuItem(localQuit, nil)
		quitItem.IsQuit = true
		menus.Items[0].Items = append(menus.Items[0].Items, bellui.NewMenuItemSeparator(), quitItem)
	}
	for _, item := range menus.Items[0].Items {
		if item.IsQuit && item.Action == nil {
			item.Action = func() {
				for _, win := range w.driver.AllWindows() {
					if glWin, ok := win.(*window); ok {
						glWin.closed(glWin.view())
					} else {
						win.Close() // for test windows
					}
				}
			}
		}
	}
	return menus
}
