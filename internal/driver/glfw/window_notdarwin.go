//go:build !darwin

package glfw

import "time"

const desktopDefaultDoubleTapDelay = 300 * time.Millisecond

func (w *window) doSetFullScreen(full bool) {
	monitor := w.getMonitorForWindow()

	w.doApplyFullScreen(monitor, full)
}

func (w *window) doSetFullScreen2(full bool) {
	monitor := w.getSecondaryMonitor()

	w.doApplyFullScreen(monitor, full)
}

func (w *window) doApplyFullScreen(monitor *monitor, full bool) {
	mode := monitor.GetVideoMode()
	if full {
		if mode != nil {
			w.viewport.SetMonitor(monitor, 0, 0, mode.Width, mode.Height, mode.RefreshRate)
		} else {
			// display-settings query failed (RDP/virtual display); fall back to the
			// current window size rather than dereferencing a nil video mode.
			width, height := w.width, w.height
			if width == 0 || height == 0 {
				width, height = w.screenSize(w.canvas.Size().Max(w.canvas.MinSize()))
			}
			w.viewport.SetMonitor(monitor, 0, 0, width, height, 0)
		}
	} else {
		if w.width == 0 && w.height == 0 { // if we were fullscreen on creation...
			s := w.canvas.Size().Max(w.canvas.MinSize())
			w.width, w.height = w.screenSize(s)
		}
		w.viewport.SetMonitor(nil, w.xpos, w.ypos, w.width, w.height, 0)
	}
}
