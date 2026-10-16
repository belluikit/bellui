//go:build linux && !x11 && !wasm

package glfw

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/driver/desktop"

	"github.com/belluikit/glfw"
)

// startDragWayland begins a native Wayland drag-out via the belluikit/glfw fork's
// StartWaylandDrag (v1.7.0+, pre-rebrand tracker #838 / glfw#10): GLFW creates the wl_data_source,
// offers each flavor's MIME type and calls wl_data_device.start_drag against the
// source surface using the private pointer button serial. It must run on the main
// goroutine during the button gesture — bellui dispatches Dragged there, and the
// serial is only valid while the button is held.
//
// Limitations vs the other backends (documented, not bugs):
//   - No completion signal is exposed (wl_data_source.dnd_finished is internal to
//     GLFW), so the drag is fire-and-forget: DragData.OnCompleted is NOT called on
//     Wayland.
//   - No drag icon/cursor is attached (a preview surface is a follow-up, like the
//     X11 cursor gap).
//
// Returns whether the drag started (false when not Wayland, no focused source
// window, no input serial, or no payload).
func startDragWayland(data desktop.DragData) bool {
	flavors := buildDragFlavors(data)
	if len(flavors) == 0 {
		return false
	}

	win := focusedGLWindow()
	if win == nil || win.viewport == nil {
		return false // no source surface / serial to seed start_drag
	}

	cf := make([]glfw.ClipboardFlavor, len(flavors))
	for i, f := range flavors {
		cf[i] = glfw.ClipboardFlavor{MIMEType: f.mime, Data: f.data}
	}

	return win.viewport.StartWaylandDrag(cf, glfw.WaylandDragCopy|glfw.WaylandDragMove)
}

// focusedGLWindow returns the glfw-backed window that currently holds input focus
// — the source of the drag. StartWaylandDrag seeds start_drag from the source
// surface and its latest pointer serial, so the drag must originate from the
// focused window (the one whose Dragged handler is running).
func focusedGLWindow() *window {
	drv, ok := bellui.CurrentApp().Driver().(*gLDriver)
	if !ok {
		return nil
	}
	for _, w := range drv.windowList() {
		gw, ok := w.(*window)
		if !ok || gw.viewport == nil {
			continue
		}
		if gw.viewport.GetAttrib(glfw.Focused) == glfw.True {
			return gw
		}
	}
	return nil
}
