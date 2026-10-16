// Command multiwindowprobe verifies mobile multi-window stack UX (pre-rebrand tracker #645, a pre-rebrand tracker #499
// follow-up): opening a second NewWindow().Show() over a visible one pushes a
// child window that covers the parent and carries a titlebar with a close (X)
// button; closing it pops back to the parent.
//
// iOS/Android have no adb-style touch injection, so on mobile the flow is driven
// on a timer: t+2 open a child, t+5 close it (Close() is exactly what the
// titlebar X button invokes via tryClose). The parent's label is updated on
// close so a screenshot confirms the pop-back actually revealed and repainted
// the parent. The "Open Child" / "Close this child" buttons drive it
// interactively.
package main

import (
	"runtime"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"
)

func openChild(a bellui.App) bellui.Window {
	child := a.NewWindow("Child Window")
	child.SetContent(container.NewVBox(
		widget.NewLabel("CHILD window (over parent)"),
		widget.NewLabel("tap the X in the titlebar to pop back"),
		widget.NewButton("Close this child", func() { child.Close() }),
	))
	child.Resize(geo.NewSize(400, 300))
	child.Show()
	return child
}

func main() {
	a := app.New()
	parent := a.NewWindow("Parent")
	parentLbl := widget.NewLabel("PARENT window (root)")
	parent.SetContent(container.NewVBox(
		parentLbl,
		widget.NewButton("Open Child", func() { openChild(a) }),
	))
	parent.Resize(geo.NewSize(400, 300))

	if runtime.GOOS == "ios" || runtime.GOOS == "android" {
		var child bellui.Window
		time.AfterFunc(2*time.Second, func() {
			bellui.Do(func() { child = openChild(a) })
		})
		time.AfterFunc(5*time.Second, func() {
			bellui.Do(func() {
				if child != nil {
					child.Close() // same path as the titlebar X button
				}
				// Visible proof the pop-back revealed and repainted the parent.
				parentLbl.SetText("PARENT visible again — child was closed")
			})
		})
	}

	bellui.ShowAndRun(parent)
}
