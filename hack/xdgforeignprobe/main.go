// Command xdgforeignprobe verifies the xdg-foreign portal-dialog parenting on
// Wayland (fyne#626 + belluikit/glfw#7). It prints the window's xdg-foreign
// export handle (the fork's (*glfw.Window).ExportWaylandHandle, surfaced on
// driver.WaylandWindowContext.ExportedHandle) and opens a native file dialog, so
// a dbus-monitor of org.freedesktop.portal.FileChooser.OpenFile shows its
// parent_window == "wayland:<handle>".
//
//	BELLUI_PLATFORM=wayland WGPU_BACKEND=vulkan go run ./hack/xdgforeignprobe
package main

import (
	"fmt"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/dialog"
	"github.com/belluikit/bellui/driver"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"
)

func main() {
	dialog.UseNativeFileDialogs(true)

	a := app.NewWithID("io.bellui.xdgforeignprobe")
	w := a.NewWindow("xdg-foreign probe")
	w.SetContent(widget.NewLabel("xdg-foreign probe"))
	w.Resize(geo.NewSize(400, 200))

	go func() {
		time.Sleep(2 * time.Second)
		bellui.Do(func() {
			if nw, ok := w.(driver.NativeWindow); ok {
				nw.RunNative(func(ctx any) {
					if wc, ok := ctx.(driver.WaylandWindowContext); ok {
						fmt.Printf("EXPORTED_HANDLE=%q\n", wc.ExportedHandle)
					} else {
						fmt.Printf("NOT_WAYLAND_CONTEXT: %T\n", ctx)
					}
				})
			}
			// Open a native file dialog → portal OpenFile carries parent_window.
			dialog.ShowFileOpen(func(bellui.URIReadCloser, error) {}, w)
		})
		time.Sleep(6 * time.Second)
		bellui.Do(func() { a.Quit() })
	}()

	bellui.ShowAndRun(w)
}
