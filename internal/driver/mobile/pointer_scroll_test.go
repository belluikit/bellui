//go:build !wasm

package mobile

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	belluicanvas "github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/driver/mobile/event/scroll"
	"github.com/belluikit/bellui/widget"
)

// scrollRecorder is a ScrollEnder that records what it is given.
type scrollRecorder struct {
	widget.BaseWidget
	scrolls []event.ScrollEvent
	ended   int
}

func newScrollRecorder() *scrollRecorder {
	r := &scrollRecorder{}
	r.ExtendBaseWidget(r)
	return r
}

func (r *scrollRecorder) CreateRenderer() core.WidgetRenderer {
	return widget.NewSimpleRenderer(belluicanvas.NewRectangle(color.Gray{Y: 0x80}))
}

func (r *scrollRecorder) MinSize() geo.Size              { return geo.NewSize(100, 100) }
func (r *scrollRecorder) Scrolled(ev *event.ScrollEvent) { r.scrolls = append(r.scrolls, *ev) }
func (r *scrollRecorder) ScrollEnded()                   { r.ended++ }

// recorderCanvas lays out two recorders, one above the other.
func recorderCanvas(t *testing.T) (*canvas, *scrollRecorder, *scrollRecorder) {
	t.Helper()
	top, bottom := newScrollRecorder(), newScrollRecorder()
	c := newCanvas(bellui.CurrentDevice()).(*canvas)
	c.SetContent(container.NewGridWithRows(2, top, bottom))
	c.Resize(geo.NewSize(100, 200))
	return c, top, bottom
}

// A mouse wheel over a Scroll on a touch device scrolls it (#1510). It used to
// arrive as a touch move that went nowhere.
func Test_canvas_PointerWheelScrollsTheScrollUnderIt(t *testing.T) {
	filler := belluicanvas.NewRectangle(color.Transparent)
	filler.SetMinSize(geo.NewSize(100, 1000))
	page := container.NewVScroll(filler)
	c := newCanvas(bellui.CurrentDevice()).(*canvas)
	c.SetContent(page)
	c.Resize(geo.NewSize(100, 200))
	require.Zero(t, page.Offset.Y)

	c.PointerScrolled(geo.NewPos(50, 50), geo.NewDelta(0, -64), false, scroll.PhaseNone)
	assert.Equal(t, float32(64), page.Offset.Y, "a notch toward the user scrolls down")
	c.PointerScrolled(geo.NewPos(50, 50), geo.NewDelta(0, 32), false, scroll.PhaseNone)
	assert.Equal(t, float32(32), page.Offset.Y)
}

// A wheel step is a mouse wheel scroll with no phase, delivered where the
// pointer is, and tells nobody of an end.
func Test_canvas_PointerWheelIsAMouseWheelScroll(t *testing.T) {
	c, top, bottom := recorderCanvas(t)

	c.PointerScrolled(geo.NewPos(50, 150), geo.NewDelta(0, -64), false, scroll.PhaseNone)
	require.Len(t, bottom.scrolls, 1)
	assert.Empty(t, top.scrolls)
	ev := bottom.scrolls[0]
	assert.Equal(t, event.ScrollSourceMouseWheel, ev.Source)
	assert.Equal(t, event.ScrollPhaseUnknown, ev.Phase)
	assert.Equal(t, geo.NewDelta(0, -64), ev.Scrolled)
	assert.Equal(t, geo.NewPos(50, 150), ev.AbsolutePosition)
	assert.Zero(t, bottom.ended)
}

// A touchpad gesture's first step that moves is its beginning, the rest are
// changes, all touchpad scrolls, and its ScrollEnders are told once when the
// fingers lift. The scrollables it began over keep it, wherever the pointer
// reports it later.
func Test_canvas_PointerTouchpadGesture(t *testing.T) {
	c, top, bottom := recorderCanvas(t)

	c.PointerScrolled(geo.NewPos(50, 50), geo.Delta{}, true, scroll.PhaseBegan)
	assert.Empty(t, top.scrolls, "a gesture that has not moved scrolls nothing")
	c.PointerScrolled(geo.NewPos(50, 50), geo.NewDelta(0, 4), true, scroll.PhaseChanged)
	c.PointerScrolled(geo.NewPos(50, 150), geo.NewDelta(0, 4), true, scroll.PhaseChanged)
	c.PointerScrolled(geo.NewPos(50, 150), geo.Delta{}, true, scroll.PhaseChanged)
	assert.Zero(t, top.ended, "told only when the fingers lift")
	c.PointerScrolled(geo.NewPos(50, 150), geo.Delta{}, true, scroll.PhaseEnded)

	require.Len(t, top.scrolls, 2)
	assert.Empty(t, bottom.scrolls, "the gesture stays with what it began over")
	assert.Equal(t, event.ScrollPhaseBegan, top.scrolls[0].Phase)
	assert.Equal(t, event.ScrollPhaseChanged, top.scrolls[1].Phase)
	for _, ev := range top.scrolls {
		assert.Equal(t, event.ScrollSourceTouchpad, ev.Source)
		assert.Equal(t, geo.NewDelta(0, 4), ev.Scrolled)
	}
	assert.Equal(t, 1, top.ended)
	assert.Zero(t, bottom.ended)

	c.PointerScrolled(geo.NewPos(50, 50), geo.Delta{}, true, scroll.PhaseEnded)
	assert.Equal(t, 1, top.ended, "an end with no gesture under way tells nobody")
}

// A gesture that begins while another is under way ends the first: its
// ScrollEnders are told, and the new one reports its own beginning.
func Test_canvas_PointerTouchpadBeganEndsTheLastGesture(t *testing.T) {
	c, top, bottom := recorderCanvas(t)

	c.PointerScrolled(geo.NewPos(50, 50), geo.Delta{}, true, scroll.PhaseBegan)
	c.PointerScrolled(geo.NewPos(50, 50), geo.NewDelta(3, 0), true, scroll.PhaseChanged)
	c.PointerScrolled(geo.NewPos(50, 150), geo.Delta{}, true, scroll.PhaseBegan)
	assert.Equal(t, 1, top.ended)
	c.PointerScrolled(geo.NewPos(50, 150), geo.NewDelta(3, 0), true, scroll.PhaseChanged)
	require.Len(t, bottom.scrolls, 1)
	assert.Equal(t, event.ScrollPhaseBegan, bottom.scrolls[0].Phase)

	// A step with no beginning starts a gesture of its own.
	c, top, _ = recorderCanvas(t)
	c.PointerScrolled(geo.NewPos(50, 50), geo.NewDelta(0, 2), true, scroll.PhaseChanged)
	require.Len(t, top.scrolls, 1)
	assert.Equal(t, event.ScrollPhaseBegan, top.scrolls[0].Phase)
	c.PointerScrolled(geo.NewPos(50, 50), geo.Delta{}, true, scroll.PhaseEnded)
	assert.Equal(t, 1, top.ended)
}

// The driver converts a pointer scroll's pixels to canvas units with the
// canvas's scale, for both the position and the distance.
func Test_driver_PointerScrollConvertsPixels(t *testing.T) {
	d := &driver{}
	w := d.CreateWindow("wheel").(*window)
	t.Cleanup(w.Close)
	top, bottom := newScrollRecorder(), newScrollRecorder()
	w.SetContent(container.NewGridWithRows(2, top, bottom))
	w.Show()
	w.canvas.Resize(geo.NewSize(100, 200))
	s := w.canvas.Scale()

	d.handlePointerScroll(w, scroll.Event{X: 50 * s, Y: 150 * s, DY: -64 * s})
	require.Len(t, bottom.scrolls, 1, "the pixel position maps to the lower half")
	assert.Empty(t, top.scrolls)
	assert.InDelta(t, -64, bottom.scrolls[0].Scrolled.DY, 0.01)
	assert.InDelta(t, 150, bottom.scrolls[0].AbsolutePosition.Y, 1)
}
