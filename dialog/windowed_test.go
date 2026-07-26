package dialog

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

func TestDialog_WindowedDragAndResize(t *testing.T) {
	test.NewTempApp(t)
	w := test.NewTempWindow(t, nil)
	w.Resize(geo.NewSize(600, 400))

	d := NewInformation("Title", "message", w)
	d.Show()
	defer d.Hide()

	dlg := d.(*dialog)
	win, ok := dlg.win.Content.(*container.InnerWindow)
	assert.True(t, ok, "windowed dialogs host their body in an InnerWindow")

	// Dragging the title bar moves the dialog.
	before := win.Position()
	win.OnDragged(&event.DragEvent{Dragged: geo.Delta{DX: 30, DY: 20}})
	after := win.Position()
	assert.Equal(t, before.Add(geo.NewPos(30, 20)), after)

	// Dragging the corner grows the dialog, clamped to its minimum size.
	sizeBefore := win.Size()
	win.OnResized(&event.DragEvent{Dragged: geo.Delta{DX: 40, DY: 30}})
	sizeAfter := win.Size()
	assert.Equal(t, sizeBefore.Add(geo.NewSize(40, 30)), sizeAfter)

	// A huge shrink clamps to the window minimum (the exact value depends
	// on label wrapping, so just verify it shrank and stayed usable).
	win.OnResized(&event.DragEvent{Dragged: geo.Delta{DX: -1000, DY: -1000}})
	assert.Less(t, win.Size().Width, sizeAfter.Width)
	assert.Less(t, win.Size().Height, sizeAfter.Height)
	assert.Greater(t, win.Size().Width, float32(50))
	assert.Greater(t, win.Size().Height, float32(50))
}

func TestDialog_SetWindowedFalse(t *testing.T) {
	test.NewTempApp(t)
	w := test.NewTempWindow(t, nil)
	w.Resize(geo.NewSize(600, 400))

	d := NewInformation("Title", "message", w)
	d.SetWindowed(false)
	d.Show()
	defer d.Hide()

	dlg := d.(*dialog)
	_, windowed := dlg.win.Content.(*container.InnerWindow)
	assert.False(t, windowed, "SetWindowed(false) hosts the bare dialog body")
	assert.Equal(t, dlg.body, dlg.win.Content)
}

func TestFolderOpen_Windowed(t *testing.T) {
	test.NewTempApp(t)
	w := test.NewTempWindow(t, nil)

	d := NewFolderOpen(func(bellui.ListableURI, error) {}, w)
	assert.True(t, d.windowed, "folder dialogs default to the windowed frame")
}
