package test_test

import (
	"bytes"
	"image/color"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
)

func TestAssertCanvasTappableAt(t *testing.T) {
	c := test.NewCanvas()
	b := widget.NewButton("foo", nil)
	c.SetContent(b)
	c.Resize(geo.NewSize(300, 300))
	b.Resize(geo.NewSize(100, 100))
	b.Move(geo.NewPos(100, 100))

	tt := &testing.T{}
	assert.True(t, test.AssertCanvasTappableAt(tt, c, geo.NewPos(101, 101)), "tappable found")
	assert.False(t, tt.Failed(), "test did not fail")

	tt = &testing.T{}
	assert.False(t, test.AssertCanvasTappableAt(tt, c, geo.NewPos(99, 99)), "tappable not found")
	assert.True(t, tt.Failed(), "test failed")
}

func TestAssertObjectRendersToImage(t *testing.T) {
	obj := canvas.NewCircle(color.Black)
	obj.Resize(geo.NewSize(20, 20))

	test.AssertObjectRendersToImage(t, "circle.png", obj)
}

func TestRenderObjectToMarkup(t *testing.T) {
	obj := canvas.NewCircle(color.Black)
	obj.Resize(geo.NewSize(20, 20))

	want, err := os.ReadFile("testdata/circle.xml")
	require.NoError(t, err)
	// Fix Windows newlines
	want = bytes.ReplaceAll(want, []byte("\r\n"), []byte("\n"))
	got := strings.ReplaceAll(test.RenderObjectToMarkup(obj), "\r\n", "\n")
	assert.Equal(t, string(want), got, "existing master is equal to rendered markup")
}

func TestAssertObjectRendersToMarkup(t *testing.T) {
	obj := canvas.NewCircle(color.Black)
	obj.Resize(geo.NewSize(20, 20))

	assert.True(t, test.AssertObjectRendersToMarkup(t, "circle.xml", obj), "existing master is equal to rendered markup")
}

func TestAssertRendersToImage(t *testing.T) {
	c := test.NewCanvas() // no painter, would be cycle - images will be blank
	c.SetContent(canvas.NewCircle(color.Black))
	c.Resize(geo.NewSize(10, 10))

	t.Run("non-existing master", func(t *testing.T) {
		tt := &testing.T{}
		assert.False(t, test.AssertRendersToImage(tt, "non_existing_master.png", c), "non existing master is not equal to rendered image")
		assert.True(t, tt.Failed(), "test failed")
		_, err := os.Stat("testdata/failed/non_existing_master.png")
		assert.NoError(t, err)
	})

	t.Run("matching master", func(t *testing.T) {
		tt := &testing.T{}
		assert.True(t, test.AssertRendersToImage(tt, "image_master.png", c), "existing master is equal to rendered image")
		assert.False(t, tt.Failed(), "test should not fail")
		_, err := os.Stat("testdata/failed/image_master.png")
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("diffing master", func(t *testing.T) {
		c.Resize(geo.NewSize(15, 15))

		tt := &testing.T{}
		assert.False(t, test.AssertRendersToImage(tt, "image_diffing_master.png", c), "existing master is not equal to rendered image")
		assert.True(t, tt.Failed(), "test should fail")
		_, err := os.Stat("testdata/failed/image_diffing_master.png")
		assert.NoError(t, err)
	})

	if !t.Failed() {
		_ = os.RemoveAll("testdata/failed")
	}
}

func TestRenderToMarkup(t *testing.T) {
	c := test.NewCanvas()
	c.SetContent(canvas.NewCircle(color.Black))

	want, err := os.ReadFile("testdata/markup_master.xml")
	require.NoError(t, err)
	// Fix Windows newlines
	want = bytes.ReplaceAll(want, []byte("\r\n"), []byte("\n"))
	got := strings.ReplaceAll(test.RenderToMarkup(c), "\r\n", "\n")
	assert.Equal(t, string(want), got, "existing master is equal to rendered markup")
}

func TestAssertRendersToMarkup(t *testing.T) {
	c := test.NewCanvas()
	c.SetContent(canvas.NewCircle(color.Black))

	markup := "<canvas padded size=\"9x9\">\n" +
		"\t<content>\n" +
		"\t\t<circle fillColor=\"rgba(0,0,0,255)\" pos=\"4,4\" size=\"1x1\"/>\n" +
		"\t</content>\n" +
		"</canvas>\n"

	t.Run("non-existing master", func(t *testing.T) {
		tt := &testing.T{}
		assert.False(t, test.AssertRendersToMarkup(tt, "non_existing_master.xml", c), "non existing master is not equal to rendered markup")
		assert.True(t, tt.Failed(), "test failed")
		assert.Equal(t, markup, readMarkup(t, "testdata/failed/non_existing_master.xml"), "markup was written to disk")
	})

	t.Run("matching master", func(t *testing.T) {
		tt := &testing.T{}
		assert.True(t, test.AssertRendersToMarkup(tt, "markup_master.xml", c), "existing master is equal to rendered markup")
		assert.False(t, tt.Failed(), "test should not fail")
	})

	t.Run("diffing master", func(t *testing.T) {
		tt := &testing.T{}
		assert.False(t, test.AssertRendersToMarkup(tt, "markup_diffing_master.xml", c), "existing master is not equal to rendered markup")
		assert.True(t, tt.Failed(), "test should fail")
		assert.Equal(t, markup, readMarkup(t, "testdata/failed/markup_diffing_master.xml"), "markup was written to disk")
	})

	if !t.Failed() {
		os.RemoveAll("testdata/failed")
	}
}

func TestDrag(t *testing.T) {
	c := test.NewCanvas()
	c.SetPadded(false)
	d := &draggable{}
	c.SetContent(container.NewWithoutLayout(d))
	c.Resize(geo.NewSize(30, 30))
	d.Resize(geo.NewSize(20, 20))
	d.Move(geo.NewPos(10, 10))

	test.Drag(c, geo.NewPos(5, 5), 10, 10)
	assert.Nil(t, d.event, "nothing happens if no draggable was found at position")
	assert.False(t, d.wasDragged)

	test.Drag(c, geo.NewPos(15, 15), 17, 42)
	assert.Equal(t, &event.DragEvent{
		// AbsolutePosition is populated from pos (#1018) so widgets that
		// resolve the cursor via AbsolutePosition work headlessly.
		PointEvent: event.PointEvent{Position: geo.Position{X: 5, Y: 5}, AbsolutePosition: geo.Position{X: 15, Y: 15}},
		Dragged:    geo.NewDelta(17, 42),
	}, d.event)
	assert.True(t, d.wasDragged)
}

func TestDragSteps(t *testing.T) {
	c := test.NewCanvas()
	c.SetPadded(false)
	d := &recordingDraggable{}
	c.SetContent(container.NewWithoutLayout(d))
	c.Resize(geo.NewSize(120, 120))
	d.Resize(geo.NewSize(100, 100))
	d.Move(geo.NewPos(10, 10))

	// No draggable at the position → nothing dispatched.
	test.DragSteps(c, geo.NewPos(5, 5), 40, 40, 4, nil)
	assert.Empty(t, d.events)
	assert.False(t, d.ended)

	var midDrag []geo.Position
	test.DragSteps(c, geo.NewPos(20, 20), 40, 40, 4, func(step int) {
		// Observed while the drag is still in progress, before DragEnd.
		assert.False(t, d.ended, "DragEnd must not fire until every step is done")
		midDrag = append(midDrag, d.events[len(d.events)-1].AbsolutePosition)
	})

	// Four Dragged events, each advancing AbsolutePosition by 10 toward the target.
	assert.Len(t, d.events, 4)
	assert.True(t, d.ended)
	for i, ev := range d.events {
		wantAbs := geo.NewPos(20+float32(i+1)*10, 20+float32(i+1)*10)
		assert.Equal(t, wantAbs, ev.AbsolutePosition, "step %d absolute", i)
		// Object is at (10,10) so local = absolute − (10,10).
		assert.Equal(t, wantAbs.SubtractXY(10, 10), ev.Position, "step %d local", i)
		assert.Equal(t, geo.NewDelta(10, 10), ev.Dragged, "step %d delta", i)
	}
	// The observer saw each step's live AbsolutePosition.
	assert.Equal(t, []geo.Position{
		geo.NewPos(30, 30), geo.NewPos(40, 40), geo.NewPos(50, 50), geo.NewPos(60, 60),
	}, midDrag)

	// steps < 1 clamps to a single Dragged carrying the whole delta.
	d2 := &recordingDraggable{}
	c.SetContent(container.NewWithoutLayout(d2))
	d2.Resize(geo.NewSize(100, 100))
	d2.Move(geo.NewPos(10, 10))
	test.DragSteps(c, geo.NewPos(20, 20), 30, 15, 0, nil)
	assert.Len(t, d2.events, 1)
	assert.Equal(t, geo.NewDelta(30, 15), d2.events[0].Dragged)
	assert.Equal(t, geo.NewPos(50, 35), d2.events[0].AbsolutePosition)
	assert.True(t, d2.ended)
}

func TestFocusNext(t *testing.T) {
	c := test.NewCanvas()
	f1 := &focusable{}
	f2 := &focusable{}
	f3 := &focusable{}
	c.SetContent(container.NewWithoutLayout(f1, f2, f3))

	assert.Nil(t, c.Focused())
	assert.False(t, f1.focused)
	assert.False(t, f2.focused)
	assert.False(t, f3.focused)

	test.FocusNext(c)
	assert.Equal(t, f1, c.Focused())
	assert.True(t, f1.focused)
	assert.False(t, f2.focused)
	assert.False(t, f3.focused)

	test.FocusNext(c)
	assert.Equal(t, f2, c.Focused())
	assert.False(t, f1.focused)
	assert.True(t, f2.focused)
	assert.False(t, f3.focused)

	test.FocusNext(c)
	assert.Equal(t, f3, c.Focused())
	assert.False(t, f1.focused)
	assert.False(t, f2.focused)
	assert.True(t, f3.focused)

	test.FocusNext(c)
	assert.Equal(t, f1, c.Focused())
	assert.True(t, f1.focused)
	assert.False(t, f2.focused)
	assert.False(t, f3.focused)
}

func TestFocusPrevious(t *testing.T) {
	c := test.NewCanvas()
	f1 := &focusable{}
	f2 := &focusable{}
	f3 := &focusable{}
	c.SetContent(container.NewWithoutLayout(f1, f2, f3))

	assert.Nil(t, c.Focused())
	assert.False(t, f1.focused)
	assert.False(t, f2.focused)
	assert.False(t, f3.focused)

	test.FocusPrevious(c)
	assert.Equal(t, f3, c.Focused())
	assert.False(t, f1.focused)
	assert.False(t, f2.focused)
	assert.True(t, f3.focused)

	test.FocusPrevious(c)
	assert.Equal(t, f2, c.Focused())
	assert.False(t, f1.focused)
	assert.True(t, f2.focused)
	assert.False(t, f3.focused)

	test.FocusPrevious(c)
	assert.Equal(t, f1, c.Focused())
	assert.True(t, f1.focused)
	assert.False(t, f2.focused)
	assert.False(t, f3.focused)

	test.FocusPrevious(c)
	assert.Equal(t, f3, c.Focused())
	assert.False(t, f1.focused)
	assert.False(t, f2.focused)
	assert.True(t, f3.focused)
}

func TestScroll(t *testing.T) {
	c := test.NewCanvas()
	c.SetPadded(false)
	s := &scrollable{}
	c.SetContent(container.NewWithoutLayout(s))
	c.Resize(geo.NewSize(30, 30))
	s.Resize(geo.NewSize(20, 20))
	s.Move(geo.NewPos(10, 10))

	test.Scroll(c, geo.NewPos(5, 5), 10, 10)
	assert.Nil(t, s.event, "nothing happens if no scrollable was found at position")

	test.Scroll(c, geo.NewPos(15, 15), 17, 42)
	assert.Equal(t, &event.ScrollEvent{Scrolled: geo.NewDelta(17, 42)}, s.event)
}

func readMarkup(t *testing.T, path string) string {
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(raw)
}

var _ event.Draggable = (*draggable)(nil)

type draggable struct {
	widget.BaseWidget
	event      *event.DragEvent
	wasDragged bool
}

func (d *draggable) DragEnd() {
	d.wasDragged = true
}

func (d *draggable) Dragged(ev *event.DragEvent) {
	d.event = ev
}

var _ event.Draggable = (*recordingDraggable)(nil)

// recordingDraggable keeps every DragEvent it receives so a multi-step drag can
// be asserted event-by-event.
type recordingDraggable struct {
	widget.BaseWidget
	events []*event.DragEvent
	ended  bool
}

func (d *recordingDraggable) DragEnd() { d.ended = true }

func (d *recordingDraggable) Dragged(ev *event.DragEvent) {
	// Copy: DragSteps reuses one struct per call site only via fresh allocation,
	// but guard against any aliasing so recorded events stay independent.
	cp := *ev
	d.events = append(d.events, &cp)
}

var _ event.Focusable = (*focusable)(nil)

type focusable struct {
	widget.BaseWidget
	focused bool
}

func (f *focusable) FocusGained() {
	f.focused = true
}

func (f *focusable) FocusLost() {
	f.focused = false
}

func (f *focusable) TypedKey(ev *event.KeyEvent) {
}

func (f *focusable) TypedRune(r rune) {
}

var _ event.Scrollable = (*scrollable)(nil)

type scrollable struct {
	widget.BaseWidget
	event *event.ScrollEvent
}

func (s *scrollable) Scrolled(ev *event.ScrollEvent) {
	s.event = ev
}
