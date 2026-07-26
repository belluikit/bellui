package dialog

// Window-hosted mode for FileDialog (#597): SetWindowHosted parity with the
// shared *dialog host (windowhost.go). FileDialog has its own show/hide
// plumbing (the fileDialog.win popup, direct win.Show()/Hide() calls and
// internal popups anchored to win.Canvas), so the host-aware seams below give
// it the same "open the body in its own native window, block the parent with a
// modal scrim" behavior. On single-window platforms the mode is ignored and
// the overlay popup presentation is used (see windowHostSupported).

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
)

// hosted reports whether this file dialog should open in its own window.
func (f *fileDialog) hosted() bool {
	return f.file.windowHosted && windowHostSupported
}

// showHostWindow opens the dialog UI (f.body) in its own native window, sized
// to size and centered over the parent, blocking the parent with a scrim.
func (f *fileDialog) showHostWindow(size geo.Size) {
	if f.hostWin != nil {
		f.hostWin.RequestFocus()
		return
	}

	w := bellui.CurrentApp().NewWindow(f.title.Text)
	f.hostWin = w
	w.SetContent(f.body)
	w.Resize(size)
	w.SetCloseIntercept(func() { f.dismiss.OnTapped() })
	w.SetOnClosed(f.hostWindowClosed)
	w.Canvas().SetOnTypedKey(func(ev *event.KeyEvent) {
		if ev.Name == event.KeyEscape {
			f.dismiss.OnTapped()
		}
	})

	f.scrim = addScrim(f.file.parent, func() {
		if f.hostWin != nil {
			f.hostWin.RequestFocus()
		}
	})
	adoptParentWindow(w, f.file.parent)
	centerWindowOver(w, f.file.parent, size)
	w.Show()
}

// hostWindowClosed runs when the host window closes for any reason. When the
// window was closed from outside the dialog API (window manager, application
// code calling Close on it) the dialog cleans up and reports a cancellation.
func (f *fileDialog) hostWindowClosed() {
	if f.hostWin == nil { // closed through the dialog API, already handled
		return
	}
	f.hostWin = nil
	removeScrim(f.file.parent, f.scrim)
	f.scrim = nil
	f.reportCancel()
}

// hide tears down whichever surface hosts the dialog — the host window and its
// scrim when window-hosted, the overlay popup otherwise — without running any
// response callback (the caller reports its own response).
func (f *fileDialog) hide() {
	if f.hostWin != nil {
		w := f.hostWin
		f.hostWin = nil
		removeScrim(f.file.parent, f.scrim)
		f.scrim = nil
		w.Close()
		return
	}
	f.win.Hide()
}

// show re-shows an already-built dialog. For host mode Hide closes (destroys)
// the window, so a fresh one is opened; the overlay popup is simply re-shown.
func (f *fileDialog) show() {
	if f.hosted() {
		f.showHostWindow(f.startSize)
		return
	}
	f.win.Show()
}

// refresh refreshes whichever surface currently hosts the dialog body. The
// overlay popup must not be refreshed while the body lives in a host window:
// its renderer would lay the shared body out to the popup's size.
func (f *fileDialog) refresh() {
	if f.hostWin != nil {
		f.body.Refresh()
		return
	}
	f.win.Refresh()
}

// resize routes a resize request to the active surface.
func (f *fileDialog) resize(size geo.Size) {
	if f.hostWin != nil {
		f.hostWin.Resize(size)
		return
	}
	f.win.Resize(size)
}

// minSize reports the minimum size of the active surface.
func (f *fileDialog) minSize() geo.Size {
	if f.hostWin != nil {
		return f.body.MinSize()
	}
	return f.win.MinSize()
}

// canvas is the canvas that currently hosts the dialog body, used to anchor
// internal popups (options menu) and focus the save filename entry.
func (f *fileDialog) canvas() core.Canvas {
	if f.hostWin != nil {
		return f.hostWin.Canvas()
	}
	return f.win.Canvas
}

// dialogParent is the window that nested dialogs (New Folder form, Overwrite
// confirm, errors) should open over. When window-hosted that is the dialog's
// own window, so they appear above it rather than behind the scrim on the
// blocked parent.
func (f *fileDialog) dialogParent() bellui.Window {
	if f.hostWin != nil {
		return f.hostWin
	}
	return f.file.parent
}
