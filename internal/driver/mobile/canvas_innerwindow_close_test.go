//go:build !windows || !ci

package mobile

import (
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	intdriver "github.com/belluikit/bellui/internal/driver"
	_ "github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"

	"github.com/stretchr/testify/assert"
)

// tapAt drives a full quick tap (down + up) through the shared touch handler,
// the way tapDownCanvas/tapUpCanvas do for real events.
func tapAt(c *canvas, pos geo.Position) {
	c.TapDown(pos, 0)
	c.TapUp(pos, 0, func(wid event.Tappable, ev *event.PointEvent) {
		wid.Tapped(ev)
	}, func(wid event.SecondaryTappable, ev *event.PointEvent) {
		wid.TappedSecondary(ev)
	}, func(wid event.LongPressable, ev *event.PointEvent) {
		wid.LongPressed(ev)
	}, func(wid event.DoubleTappable, ev *event.PointEvent) {
		wid.DoubleTapped(ev)
	}, func(wid event.Draggable, ev *event.DragEvent) {
		wid.Dragged(ev)
	})
}

// showWindowedDialogFrame replicates dialog.wrapModal's windowed structure on
// the mobile canvas: the dialog body framed by an InnerWindow with trailing
// buttons, hosted in an unpadded modal popup.
func showWindowedDialogFrame(c *canvas, onClose func()) *widget.PopUp {
	body := widget.NewLabel("dialog body")
	win := container.NewInnerWindow("", body)
	win.Alignment = widget.ButtonAlignTrailing
	pop := widget.NewModalPopUp(win, c)
	pop.SetPadded(false)
	win.CloseIntercept = onClose
	win.OnMaximized = func() {}
	pop.Resize(geo.NewSize(300, 200))
	pop.Show()
	c.EnsureMinSize()
	return pop
}

// frameCloseButton locates the InnerWindow frame's close button: with trailing
// alignment it is the right-most widget.Button in the title bar. It returns
// the button and its absolute position within the popup tree.
func frameCloseButton(pop *widget.PopUp) (core.CanvasObject, geo.Position) {
	var btn core.CanvasObject
	var btnPos geo.Position
	intdriver.WalkVisibleObjectTree(pop, func(obj core.CanvasObject, pos geo.Position, _ geo.Position, _ geo.Size) bool {
		if _, ok := obj.(*widget.Button); !ok {
			return false
		}
		if btn == nil || pos.X > btnPos.X {
			btn, btnPos = obj, pos
		}
		return false
	}, nil)
	return btn, btnPos
}

// Test_windowedDialogFrameCloseButton_Tap covers #254: the windowed canvas
// dialog hosts its body in a container.InnerWindow (dialog.wrapModal), and
// tapping the frame's close (X) button on a mobile canvas must fire
// CloseIntercept.
func Test_windowedDialogFrameCloseButton_Tap(t *testing.T) {
	c := newCanvas(bellui.CurrentDevice()).(*canvas)
	c.SetContent(widget.NewLabel("content behind"))
	c.Resize(geo.NewSize(600, 600))

	closed := 0
	pop := showWindowedDialogFrame(c, func() { closed++ })

	closeBtn, btnPos := frameCloseButton(pop)
	if closeBtn == nil {
		t.Fatal("no frame buttons found in the InnerWindow title bar")
	}

	btnSize := closeBtn.Size()
	center := btnPos.Add(geo.NewPos(btnSize.Width/2, btnSize.Height/2))
	tapAt(c, center)
	assert.Equal(t, 1, closed, "tapping the frame close button center must fire CloseIntercept")
}
