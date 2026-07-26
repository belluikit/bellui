//go:build !windows || !ci

package mobile

import (
	"image/color"
	"io"
	"log"
	"os"
	"testing"
	"time"

	"github.com/belluikit/bellui"
	belluicanvas "github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/mobile"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/driver/common"
	_ "github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"

	"github.com/stretchr/testify/assert"
)

func Test_canvas_ChildMinSizeChangeAffectsAncestorsUpToRoot(t *testing.T) {
	c := newCanvas(bellui.CurrentDevice()).(*canvas)
	leftObj1 := belluicanvas.NewRectangle(color.Black)
	leftObj1.SetMinSize(geo.NewSize(100, 50))
	leftObj2 := belluicanvas.NewRectangle(color.Black)
	leftObj2.SetMinSize(geo.NewSize(100, 50))
	leftCol := container.NewVBox(leftObj1, leftObj2)
	rightObj1 := belluicanvas.NewRectangle(color.Black)
	rightObj1.SetMinSize(geo.NewSize(100, 50))
	rightObj2 := belluicanvas.NewRectangle(color.Black)
	rightObj2.SetMinSize(geo.NewSize(100, 50))
	rightCol := container.NewVBox(rightObj1, rightObj2)
	content := container.NewHBox(leftCol, rightCol)
	c.SetContent(container.NewCenter(content))
	c.Resize(geo.NewSize(300, 300))

	oldContentSize := geo.NewSize(200+theme.Padding(), 100+theme.Padding())
	assert.Equal(t, oldContentSize, content.Size())

	leftObj1.SetMinSize(geo.NewSize(110, 60))
	c.EnsureMinSize()

	expectedContentSize := oldContentSize.Add(geo.NewSize(10, 10))
	assert.Equal(t, expectedContentSize, content.Size())
}

func Test_canvas_Dragged(t *testing.T) {
	dragged := false
	var draggedObj event.Draggable
	scroll := container.NewScroll(widget.NewLabel("Hi\nHi\nHi"))
	c := newCanvas(bellui.CurrentDevice()).(*canvas)
	c.SetContent(scroll)
	c.Resize(geo.NewSize(40, 24))
	assert.Equal(t, float32(0), scroll.Offset.Y)

	// Drag over the content column, not the right edge: on touch the scrollbar
	// renders permanently at full width (~16px) and occupies the right of this
	// 40px-wide canvas, so a drag there would grab the bar instead of the
	// content we want to fling-scroll.
	c.TapDown(geo.NewPos(10, 3), 0)
	c.TapMove(geo.NewPos(10, 10), 0, func(wid event.Draggable, ev *event.DragEvent) {
		wid.Dragged(ev)
		dragged = true
		draggedObj = wid
	})

	assert.True(t, dragged)
	assert.Equal(t, scroll, draggedObj)
	dragged = false
	c.TapMove(geo.NewPos(10, 5), 0, func(wid event.Draggable, ev *event.DragEvent) {
		wid.Dragged(ev)
		dragged = true
	})
	assert.True(t, dragged)
	assert.Equal(t, geo.NewPos(0, 5), scroll.Offset)
}

func Test_canvas_DraggingOutOfWidget(t *testing.T) {
	c := newCanvas(bellui.CurrentDevice()).(*canvas)
	slider := widget.NewSlider(0.0, 100.0)
	c.SetContent(container.NewGridWithRows(2, slider, widget.NewLabel("Outside")))
	c.Resize(geo.NewSize(100, 200))

	assert.Zero(t, slider.Value)
	lastValue := slider.Value

	dragged := false
	c.TapDown(geo.NewPos(23, 13), 0)
	c.TapMove(geo.NewPos(30, 13), 0, func(wid event.Draggable, ev *event.DragEvent) {
		assert.Equal(t, slider, wid)
		wid.Dragged(ev)
		dragged = true
	})
	assert.True(t, dragged)
	assert.Greater(t, slider.Value, lastValue)
	lastValue = slider.Value

	dragged = false
	c.TapMove(geo.NewPos(40, 120), 0, func(wid event.Draggable, ev *event.DragEvent) {
		assert.Equal(t, slider, wid)
		wid.Dragged(ev)
		dragged = true
	})
	assert.True(t, dragged)
	assert.Greater(t, slider.Value, lastValue)
}

func Test_canvas_Focusable(t *testing.T) {
	// Discarding log output for tests
	// The following method logs an error:
	// wid.Tapped(ev) on lines 148 and 159
	// c.Focus(content) on line 168
	log.SetOutput(io.Discard)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	c := newCanvas(bellui.CurrentDevice()).(*canvas)
	content := newFocusableEntry(c)
	c.SetContent(content)
	content.Resize(geo.NewSize(25, 25))

	pos := geo.NewPos(10, 10)
	d.DoFromGoroutine(func() {
		c.TapDown(pos, 0)
		c.TapUp(pos, 0, func(wid event.Tappable, ev *event.PointEvent) {
			wid.Tapped(ev)
		}, nil, nil, nil, nil)
	}, true)
	waitAndCheck(common.TapDoubleDelay/time.Millisecond+150, func() {
		assert.Equal(t, 1, content.focusedTimes)
		assert.Equal(t, 0, content.unfocusedTimes)
	})

	d.DoFromGoroutine(func() {
		c.TapDown(pos, 1)
		c.TapUp(pos, 1, func(wid event.Tappable, ev *event.PointEvent) {
			wid.Tapped(ev)
		}, nil, nil, nil, nil)
	}, true)
	waitAndCheck(common.TapDoubleDelay/time.Millisecond+150, func() {
		assert.Equal(t, 1, content.focusedTimes)
		assert.Equal(t, 0, content.unfocusedTimes)
	})

	d.DoFromGoroutine(func() {
		c.Focus(content)
		assert.Equal(t, 1, content.focusedTimes)
		assert.Equal(t, 0, content.unfocusedTimes)

		c.Unfocus()
		assert.Equal(t, 1, content.focusedTimes)
		assert.Equal(t, 1, content.unfocusedTimes)

		content.Disable()
		c.Focus(content)
		assert.Equal(t, 1, content.focusedTimes)
		assert.Equal(t, 1, content.unfocusedTimes)

		c.TapDown(geo.NewPos(10, 10), 2)
		assert.Equal(t, 1, content.focusedTimes)
		assert.Equal(t, 1, content.unfocusedTimes)
	}, true)
}

func Test_canvas_InteractiveArea(t *testing.T) {
	dev := &device{
		safeTop:    17,
		safeLeft:   42,
		safeBottom: 71,
		safeRight:  24,
	}
	scale := dev.SystemScaleForWindow(nil)

	c := newCanvas(dev)
	c.SetContent(belluicanvas.NewRectangle(color.Black))
	canvasSize := geo.NewSize(600, 800)
	c.(*canvas).Resize(canvasSize)

	t.Run("for canvas with size", func(t *testing.T) {
		pos, size := c.InteractiveArea()
		assert.Equal(t, geo.NewPos(float32(dev.safeLeft)/scale, float32(dev.safeTop)/scale), pos)
		assert.Equal(t, canvasSize.SubtractWidthHeight(float32(dev.safeLeft+dev.safeRight)/scale, float32(dev.safeTop+dev.safeBottom)/scale), size)
	})

	t.Run("when canvas size has changed", func(t *testing.T) {
		changedCanvasSize := geo.NewSize(800, 600)
		c.(*canvas).Resize(changedCanvasSize)
		pos, size := c.InteractiveArea()
		assert.Equal(t, geo.NewPos(float32(dev.safeLeft)/scale, float32(dev.safeTop)/scale), pos)
		assert.Equal(t, changedCanvasSize.SubtractWidthHeight(float32(dev.safeLeft+dev.safeRight)/scale, float32(dev.safeTop+dev.safeBottom)/scale), size)
	})

	t.Run("when canvas got hovering window head", func(t *testing.T) {
		c.(*canvas).Resize(canvasSize)
		hoveringMenu := belluicanvas.NewRectangle(color.Black)
		hoveringMenu.SetMinSize(geo.NewSize(17, 17))
		windowHead := container.NewHBox(hoveringMenu) // a single object in window head is considered to be the hovering menu
		c.(*canvas).setWindowHead(windowHead)
		pos, size := c.InteractiveArea()
		assert.Equal(t, geo.NewPos(float32(dev.safeLeft)/scale, float32(dev.safeTop)/scale), pos)
		assert.Equal(t, canvasSize.SubtractWidthHeight(float32(dev.safeLeft+dev.safeRight)/scale, float32(dev.safeTop+dev.safeBottom)/scale), size)
	})

	t.Run("when canvas got displacing window head", func(t *testing.T) {
		c.(*canvas).Resize(canvasSize)
		menu := belluicanvas.NewRectangle(color.Black)
		menu.SetMinSize(geo.NewSize(17, 17))
		title := belluicanvas.NewRectangle(color.Black)
		title.SetMinSize(geo.NewSize(42, 20))
		expectedOffset := 20 + 2*theme.Padding()
		windowHead := container.NewHBox(menu, title) // two or more objects in window head are considered to be the window title bar
		c.(*canvas).setWindowHead(windowHead)
		pos, size := c.InteractiveArea()
		assert.Equal(t, geo.NewPos(float32(dev.safeLeft)/scale, float32(dev.safeTop)/scale+expectedOffset), pos)
		assert.Equal(t, canvasSize.SubtractWidthHeight(float32(dev.safeLeft+dev.safeRight)/scale, float32(dev.safeTop+dev.safeBottom)/scale+expectedOffset), size)
	})
}

func Test_canvas_PixelCoordinateAtPosition(t *testing.T) {
	c := newCanvas(bellui.CurrentDevice()).(*canvas)

	pos := geo.NewPos(4, 4)
	c.scale = 2.5
	x, y := c.PixelCoordinateForPosition(pos)
	assert.Equal(t, 10, x)
	assert.Equal(t, 10, y)
}

func Test_canvas_ResizeWithModalPopUpOverlay(t *testing.T) {
	c := newCanvas(bellui.CurrentDevice()).(*canvas)

	c.SetContent(widget.NewLabel("Content"))

	popup := widget.NewModalPopUp(widget.NewLabel("PopUp"), c)
	popupBgSize := geo.NewSize(200, 200)
	popup.Show()
	popup.Resize(popupBgSize)

	canvasSize := geo.NewSize(600, 700)
	c.Resize(canvasSize)

	// get popup content padding dynamically
	popupContentPadding := popup.MinSize().Subtract(popup.Content.MinSize())

	assert.Equal(t, popupBgSize.Subtract(popupContentPadding), popup.Content.Size())
	assert.Equal(t, canvasSize, popup.Size())
}

func Test_canvas_Tappable(t *testing.T) {
	content := &touchableLabel{Label: widget.NewLabel("Hi\nHi\nHi")}
	content.ExtendBaseWidget(content)
	c := newCanvas(bellui.CurrentDevice()).(*canvas)
	c.SetContent(content)
	c.Resize(geo.NewSize(36, 24))
	content.Resize(geo.NewSize(24, 24))

	c.TapDown(geo.NewPos(15, 15), 0)
	assert.True(t, content.down)

	c.TapUp(geo.NewPos(15, 15), 0, func(wid event.Tappable, ev *event.PointEvent) {
	}, func(wid event.SecondaryTappable, ev *event.PointEvent) {
	}, func(wid event.LongPressable, ev *event.PointEvent) {
	}, func(wid event.DoubleTappable, ev *event.PointEvent) {
	}, func(wid event.Draggable, ev *event.DragEvent) {
	})
	assert.True(t, content.up)

	c.TapDown(geo.NewPos(15, 15), 0)
	c.TapMove(geo.NewPos(35, 15), 0, func(wid event.Draggable, ev *event.DragEvent) {
		wid.Dragged(ev)
	})
	assert.True(t, content.cancel)
}

func Test_canvas_Tapped(t *testing.T) {
	tapped := false
	altTapped := false
	buttonTap := false
	var pointEvent *event.PointEvent
	var tappedObj event.Tappable
	button := widget.NewButton("Test", func() {
		buttonTap = true
	})
	c := newCanvas(bellui.CurrentDevice()).(*canvas)
	c.SetContent(button)
	c.Resize(geo.NewSize(36, 24))
	button.Move(geo.NewPos(3, 3))

	tapPos := geo.NewPos(6, 6)
	c.TapDown(tapPos, 0)
	c.TapUp(tapPos, 0, func(wid event.Tappable, ev *event.PointEvent) {
		tapped = true
		tappedObj = wid
		pointEvent = ev
		wid.Tapped(ev)
	}, func(wid event.SecondaryTappable, ev *event.PointEvent) {
		altTapped = true
		wid.TappedSecondary(ev)
	}, func(wid event.LongPressable, ev *event.PointEvent) {
		wid.LongPressed(ev)
	}, func(wid event.DoubleTappable, ev *event.PointEvent) {
		wid.DoubleTapped(ev)
	}, func(wid event.Draggable, ev *event.DragEvent) {
	})

	assert.True(t, tapped, "tap primary")
	assert.False(t, altTapped, "don't tap secondary")
	assert.True(t, buttonTap, "button should be tapped")
	assert.Equal(t, button, tappedObj)
	if assert.NotNil(t, pointEvent) {
		assert.Equal(t, geo.NewPos(6, 6), pointEvent.AbsolutePosition)
		assert.Equal(t, geo.NewPos(3, 3), pointEvent.Position)
	}
}

func Test_canvas_LongPressToolTip(t *testing.T) {
	// Capture the driver-level tooltip seam so the assertions don't wait on the
	// real controller's 500ms show delay (#827).
	origShow, origHide := core.ShowToolTip, core.HideToolTip
	t.Cleanup(func() { core.ShowToolTip, core.HideToolTip = origShow, origHide })
	var shown core.CanvasObject
	hidden := 0
	core.ShowToolTip = func(obj core.CanvasObject, _ core.Canvas) { shown = obj }
	core.HideToolTip = func() { hidden++ }

	button := widget.NewButton("Tip", func() {})
	button.SetToolTipText("hello")
	c := newCanvas(bellui.CurrentDevice()).(*canvas)
	c.SetContent(button)
	c.Resize(geo.NewSize(80, 40))
	button.Move(geo.NewPos(3, 3))

	pos := geo.NewPos(10, 10)
	noop := func(event.Tappable, *event.PointEvent) {}

	// A press over a tooltipped widget arms its tooltip; the controller's delay
	// is the press-and-hold threshold.
	c.TapDown(pos, 0)
	assert.Equal(t, button, shown, "TapDown over a tooltipped widget arms the tooltip")

	// Releasing dismisses it (so a quick tap never actually reveals a bubble).
	c.TapUp(pos, 0, noop, nil, nil, nil, nil)
	assert.GreaterOrEqual(t, hidden, 1, "TapUp dismisses the tooltip")

	// A press where nothing carries a tooltip text clears any stale tip.
	shown, hidden = nil, 0
	c.SetContent(widget.NewLabel("no tip"))
	c.Resize(geo.NewSize(80, 40))
	c.TapDown(pos, 0)
	assert.Nil(t, shown, "a press with no tooltip under it shows nothing")
	assert.GreaterOrEqual(t, hidden, 1)

	// A drag past the tap slop cancels a pending tooltip (it is a scroll, not a hold).
	shown, hidden = nil, 0
	c.SetContent(button)
	c.Resize(geo.NewSize(80, 40))
	button.Move(geo.NewPos(3, 3))
	c.TapDown(pos, 0)
	assert.Equal(t, button, shown)
	hidden = 0
	c.TapMove(geo.NewPos(45, 10), 0, func(event.Draggable, *event.DragEvent) {})
	assert.GreaterOrEqual(t, hidden, 1, "a drag cancels the tooltip")
}

func Test_canvas_TappedAndDoubleTapped(t *testing.T) {
	tapped := 0
	but := newDoubleTappableButton()
	but.OnTapped = func() {
		tapped = 1
	}
	but.onDoubleTap = func() {
		tapped = 2
	}

	c := newCanvas(bellui.CurrentDevice()).(*canvas)
	c.SetContent(container.NewStack(but))
	c.Resize(geo.NewSize(36, 24))

	simulateTap(c)
	waitAndCheck(700, func() {
		assert.Equal(t, 1, tapped)
	})

	simulateTap(c)
	simulateTap(c)
	waitAndCheck(700, func() {
		assert.Equal(t, 2, tapped)
	})
}

func Test_canvas_TappedMulti(t *testing.T) {
	buttonTap := false
	button := widget.NewButton("Test", func() {
		buttonTap = true
	})
	c := newCanvas(bellui.CurrentDevice()).(*canvas)
	c.SetContent(button)
	c.Resize(geo.NewSize(36, 24))
	button.Move(geo.NewPos(3, 3))

	tapPos := geo.NewPos(6, 6)
	c.TapDown(tapPos, 0)
	c.TapUp(tapPos, 1, func(wid event.Tappable, ev *event.PointEvent) { // different tapID
		wid.Tapped(ev)
	}, func(wid event.SecondaryTappable, ev *event.PointEvent) {
	}, func(wid event.LongPressable, ev *event.PointEvent) {
	}, func(wid event.DoubleTappable, ev *event.PointEvent) {
		wid.DoubleTapped(ev)
	}, func(wid event.Draggable, ev *event.DragEvent) {
	})

	assert.False(t, buttonTap, "button should not be tapped")
}

func Test_canvas_TappedSecondary(t *testing.T) {
	var pointEvent *event.PointEvent
	var altTappedObj event.SecondaryTappable
	obj := &tappableLabel{}
	obj.ExtendBaseWidget(obj)
	c := newCanvas(bellui.CurrentDevice()).(*canvas)
	c.SetContent(obj)
	c.Resize(geo.NewSize(36, 24))
	obj.Move(geo.NewPos(3, 3))

	tapPos := geo.NewPos(6, 6)
	c.TapDown(tapPos, 0)
	time.Sleep(310 * time.Millisecond)
	c.TapUp(tapPos, 0, func(wid event.Tappable, ev *event.PointEvent) {
		obj.tap = true
		wid.Tapped(ev)
	}, func(wid event.SecondaryTappable, ev *event.PointEvent) {
		obj.altTap = true
		altTappedObj = wid
		pointEvent = ev
		wid.TappedSecondary(ev)
	}, func(wid event.LongPressable, ev *event.PointEvent) {
		wid.LongPressed(ev)
	}, func(wid event.DoubleTappable, ev *event.PointEvent) {
		wid.DoubleTapped(ev)
	}, func(wid event.Draggable, ev *event.DragEvent) {
	})

	assert.False(t, obj.tap, "don't tap primary")
	assert.True(t, obj.altTap, "tap secondary")
	assert.Equal(t, obj, altTappedObj)
	if assert.NotNil(t, pointEvent) {
		assert.Equal(t, geo.NewPos(6, 6), pointEvent.AbsolutePosition)
		assert.Equal(t, geo.NewPos(3, 3), pointEvent.Position)
	}
}

type touchableLabel struct {
	*widget.Label
	down, up, cancel bool
}

func (t *touchableLabel) TouchDown(event *mobile.TouchEvent) {
	t.down = true
}

func (t *touchableLabel) TouchUp(event *mobile.TouchEvent) {
	t.up = true
}

func (t *touchableLabel) TouchCancel(event *mobile.TouchEvent) {
	t.cancel = true
}

type tappableLabel struct {
	widget.Label
	tap, altTap bool
}

func (t *tappableLabel) Tapped(_ *event.PointEvent) {
	t.tap = true
}

func (t *tappableLabel) TappedSecondary(_ *event.PointEvent) {
	t.altTap = true
}

type focusableEntry struct {
	widget.Entry
	c core.Canvas

	focusedTimes   int
	unfocusedTimes int
}

func newFocusableEntry(c core.Canvas) *focusableEntry {
	entry := &focusableEntry{c: c}
	entry.ExtendBaseWidget(entry)
	return entry
}

func (f *focusableEntry) FocusGained() {
	f.focusedTimes++
	f.Entry.FocusGained()
}

func (f *focusableEntry) FocusLost() {
	f.unfocusedTimes++
	f.Entry.FocusLost()
}

func (f *focusableEntry) Tapped(ev *event.PointEvent) {
	f.c.Focus(f)
}

type doubleTappableButton struct {
	widget.Button

	onDoubleTap func()
}

func (t *doubleTappableButton) DoubleTapped(_ *event.PointEvent) {
	t.onDoubleTap()
}

func newDoubleTappableButton() *doubleTappableButton {
	but := &doubleTappableButton{}
	but.ExtendBaseWidget(but)

	return but
}

func simulateTap(c *canvas) {
	c.TapDown(geo.NewPos(15, 15), 0)
	time.Sleep(50 * time.Millisecond)
	c.TapUp(geo.NewPos(15, 15), 0, func(wid event.Tappable, ev *event.PointEvent) {
		wid.Tapped(ev)
	}, func(wid event.SecondaryTappable, ev *event.PointEvent) {
	}, func(wid event.LongPressable, ev *event.PointEvent) {
	}, func(wid event.DoubleTappable, ev *event.PointEvent) {
		wid.DoubleTapped(ev)
	}, func(wid event.Draggable, ev *event.DragEvent) {
	})
}

func waitAndCheck(msWait time.Duration, fn func()) {
	waitForCheck := common.DonePool.Get()

	go func() {
		defer common.DonePool.Put(waitForCheck)

		time.Sleep(msWait * time.Millisecond)
		d.DoFromGoroutine(func() {
			fn()

			waitForCheck <- struct{}{}
		}, true)
	}()
	<-waitForCheck
}
