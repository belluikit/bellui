package dialog

import (
	"image/color"
	"testing"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"

	"github.com/stretchr/testify/assert"
)

func TestDialog_ConfirmDoubleCallback(t *testing.T) {
	ch := make(chan int)
	cnf := NewConfirm("Test", "Test", func(_ bool) {
		ch <- 42
	}, test.NewTempWindow(t, nil))
	cnf.SetDismissText("No")
	cnf.SetConfirmText("Yes")
	cnf.SetOnClosed(func() {
		ch <- 43
	})
	cnf.Show()

	assert.False(t, cnf.win.Hidden)
	go cnf.Dismiss()
	assert.EqualValues(t, 42, <-ch)
	assert.EqualValues(t, 43, <-ch)
	assert.True(t, cnf.win.Hidden)
}

func TestDialog_ConfirmCallbackOnlyOnClosed(t *testing.T) {
	ch := make(chan int)
	cnf := NewConfirm("Test", "Test", nil, test.NewTempWindow(t, nil))
	cnf.SetDismissText("No")
	cnf.SetConfirmText("Yes")
	cnf.SetOnClosed(func() {
		ch <- 43
	})
	cnf.Show()

	assert.False(t, cnf.win.Hidden)
	go cnf.Dismiss()
	assert.EqualValues(t, 43, <-ch)
	assert.True(t, cnf.win.Hidden)
}

func TestDialog_ConfirmCallbackOnlyOnConfirm(t *testing.T) {
	ch := make(chan int)
	cnf := NewConfirm("Test", "Test", func(ok bool) {
		if !ok {
			ch <- 0
			return
		}
		ch <- 42
	}, test.NewTempWindow(t, nil))
	cnf.SetDismissText("No")
	cnf.SetConfirmText("Yes")
	cnf.Show()

	assert.False(t, cnf.win.Hidden)
	go cnf.Dismiss()
	assert.EqualValues(t, 0, <-ch)
	assert.True(t, cnf.win.Hidden)

	cnf.Show()
	assert.False(t, cnf.win.Hidden)
	go cnf.Confirm()
	assert.EqualValues(t, 42, <-ch)
	assert.True(t, cnf.win.Hidden)
}

func TestConfirmDialog_Resize(t *testing.T) {
	window := test.NewWindow(nil)
	defer window.Close()
	window.Resize(geo.NewSize(600, 400))
	d := NewConfirm("Test", "Test", nil, window)

	theDialog := d.dialog
	theDialog.SetWindowed(false) // assertions below measure the bare popup geometry
	d.dialog.Show()              // we cannot check window size if not shown

	// Test resize - normal size scenario
	size := geo.NewSize(300, 180) // normal size to fit (600,400)
	theDialog.Resize(size)
	expectedWidth := float32(300)
	assert.Equal(t, expectedWidth, theDialog.win.Content.Size().Width+theme.Padding()*2)
	expectedHeight := float32(180)
	assert.Equal(t, expectedHeight, theDialog.win.Content.Size().Height+theme.Padding()*2)
	// Test resize - normal size scenario again
	size = geo.NewSize(310, 280) // normal size to fit (600,400)
	theDialog.Resize(size)
	expectedWidth = 310
	assert.Equal(t, expectedWidth, theDialog.win.Content.Size().Width+theme.Padding()*2)
	expectedHeight = 280
	assert.Equal(t, expectedHeight, theDialog.win.Content.Size().Height+theme.Padding()*2)

	// Test resize - greater than max size scenario
	size = geo.NewSize(800, 600)
	theDialog.Resize(size)
	expectedWidth = 600                                        // since win width only 600
	assert.Equal(t, expectedWidth, theDialog.win.Size().Width) // max, also work
	assert.Equal(t, expectedWidth, theDialog.win.Content.Size().Width+theme.Padding()*2)
	expectedHeight = 400                                         // since win height only 400
	assert.Equal(t, expectedHeight, theDialog.win.Size().Height) // max, also work
	assert.Equal(t, expectedHeight, theDialog.win.Content.Size().Height+theme.Padding()*2)

	// Test again - extreme small size
	size = geo.NewSize(1, 1)
	theDialog.Resize(size)
	expectedWidth = theDialog.win.Content.MinSize().Width
	assert.Equal(t, expectedWidth, theDialog.win.Content.Size().Width)
	expectedHeight = theDialog.win.Content.MinSize().Height
	assert.Equal(t, expectedHeight, theDialog.win.Content.Size().Height)
}

func TestConfirm_Importance(t *testing.T) {
	test.NewTempApp(t)
	w := test.NewTempWindow(t, canvas.NewRectangle(color.Transparent))
	size := geo.NewSize(200, 300)
	w.Resize(size)

	d := NewConfirm("Delete me?", "This is dangerous!", nil, w)
	d.SetConfirmImportance(widget.DangerImportance)

	test.ApplyTheme(t, test.Theme())
	d.Show()
	test.AssertRendersToImage(t, "dialog-confirm-importance.png", w.Canvas())
}
