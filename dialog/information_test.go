package dialog

import (
	"errors"
	"testing"

	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
	"github.com/stretchr/testify/assert"
)

func TestDialog_MinSize(t *testing.T) {
	window := test.NewWindow(nil)
	defer window.Close()
	d := NewInformation("Looooooooooooooong title", "message...", window)
	information := d.(*dialog)

	dialogContent := information.win.Content.MinSize()
	label := information.body.Objects[4].MinSize()

	assert.Less(t, label.Width, dialogContent.Width)
}

func TestDialog_Resize(t *testing.T) {
	window := test.NewWindow(nil)
	window.Resize(geo.NewSize(600, 400))
	defer window.Close()
	d := NewInformation("Looooooooooooooong title", "message...", window)
	theDialog := d.(*dialog)
	theDialog.SetWindowed(false) // assertions below measure the bare popup geometry
	d.Show()                     // we cannot check window size if not shown

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
	// The content stays inside the canvas's interactive area, which the test
	// canvas reports a few pixels smaller than the canvas (#654).
	_, area := window.Canvas().InteractiveArea()
	assert.Equal(t, area.Width, theDialog.win.Content.Size().Width+theme.Padding()*2)
	expectedHeight = 400                                         // since win height only 400
	assert.Equal(t, expectedHeight, theDialog.win.Size().Height) // max, also work
	assert.Equal(t, area.Height, theDialog.win.Content.Size().Height+theme.Padding()*2)

	// Test again - extreme small size
	size = geo.NewSize(1, 1)
	theDialog.Resize(size)
	expectedWidth = theDialog.win.Content.MinSize().Width
	assert.Equal(t, expectedWidth, theDialog.win.Content.Size().Width)
	expectedHeight = theDialog.win.Content.MinSize().Height
	assert.Equal(t, expectedHeight, theDialog.win.Content.Size().Height)
}

func TestDialog_TextWrapping(t *testing.T) {
	window := test.NewTempWindow(t, nil)
	window.Resize(geo.NewSize(600, 400))

	d := NewInformation("Title", "This is a really really long message that will be used to test the dialog text wrapping capabilities", window)
	theDialog := d.(*dialog)
	d.Show() // we cannot check window size if not shown

	// limits width to 90% of window size
	assert.Equal(t, 600.0*maxTextDialogWinPcntWidth, theDialog.desiredSize.Width)

	theDialog.desiredSize = geo.NewSquareSize(0)
	window.Resize(geo.NewSize(900, 400))
	d.Show()

	// limits width to absolute maximum
	assert.Equal(t, maxTextDialogAbsoluteWidth, theDialog.desiredSize.Width)
}

func TestDialog_InformationCallback(t *testing.T) {
	d := NewInformation("Information", "Hello World", test.NewTempWindow(t, nil))
	tapped := false
	d.SetOnClosed(func() { tapped = true })
	d.Show()

	information := d.(*dialog)
	assert.False(t, information.win.Hidden)
	test.Tap(information.dismiss)
	assert.True(t, tapped)
	assert.True(t, information.win.Hidden)
}

func TestDialog_ErrorCallback(t *testing.T) {
	err := errors.New("Error message")
	d := NewError(err, test.NewTempWindow(t, nil))
	tapped := false
	d.SetOnClosed(func() { tapped = true })
	d.Show()

	information := d.(*dialog)
	assert.False(t, information.win.Hidden)
	test.Tap(information.dismiss)
	assert.True(t, tapped)
	assert.True(t, information.win.Hidden)
}

func TestDialog_ErrorCapitalize(t *testing.T) {
	err := errors.New("here is an error msg")
	d := NewError(err, test.NewTempWindow(t, nil))
	assert.Equal(t, "Here is an error msg", d.(*dialog).content.(*widget.Label).Text)

	err = errors.New("這是一條錯誤訊息")
	d = NewError(err, test.NewTempWindow(t, nil))
	assert.Equal(t, "這是一條錯誤訊息", d.(*dialog).content.(*widget.Label).Text)

	err = errors.New("")
	d = NewError(err, test.NewTempWindow(t, nil))
	assert.Equal(t, "", d.(*dialog).content.(*widget.Label).Text)
}
