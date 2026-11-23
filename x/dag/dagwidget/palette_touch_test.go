//go:build (ci || android || ios || mobile) && !wasm

// The palette's Scroll is Draggable only where bellui's scroller_mobile.go is
// built, and these tests are about which Draggable a touch drag reaches.

package dagwidget

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	intdriver "github.com/belluikit/bellui/internal/driver"
	"github.com/belluikit/bellui/internal/driver/common"
	"github.com/belluikit/bellui/test"

	"github.com/belluikit/bellui/x/dag/dag"
)

// pageFinder finds objects in content the way the drivers' finders do.
type pageFinder struct{ content core.CanvasObject }

func (f *pageFinder) FindObjectAtPositionMatching(pos geo.Position,
	matches func(core.CanvasObject) bool,
) (core.CanvasObject, geo.Position, int) {
	return intdriver.FindObjectAtPositionMatching(pos, matches, nil, f.content)
}

// FindEnabledObjectAtPositionMatching is the ObjectFinder method toolkits
// with #1378 call; nothing here is disabled, so no hit is inert. It calls the
// older finder, so the test builds against the pinned toolkit too.
func (f *pageFinder) FindEnabledObjectAtPositionMatching(pos geo.Position,
	matches func(core.CanvasObject) bool,
) (core.CanvasObject, geo.Position, int, bool) {
	o, p, l := f.FindObjectAtPositionMatching(pos, matches)
	return o, p, l, false
}

func (f *pageFinder) CanvasForTouch() core.Canvas { return nil }

func touchDrag(d event.Draggable, ev *event.DragEvent) { d.Dragged(ev) }

func touchUp(h *common.TouchHandler, id int, at geo.Position) {
	h.TapUp(at, id, func(event.Tappable, *event.PointEvent) {}, func(event.SecondaryTappable, *event.PointEvent) {},
		func(event.LongPressable, *event.PointEvent) {}, func(event.DoubleTappable, *event.PointEvent) {},
		func(d event.Draggable, _ *event.DragEvent) { d.DragEnd() })
}

// paletteBeside lays out a palette of twenty entries, 300 high so it scrolls,
// to the left of an empty canvas, and returns a touch handler over both whose
// long-press fires after the usual hold.
func paletteBeside(t *testing.T) (*DAGCanvas, *container.Scroll, *common.TouchHandler, chan struct{}) {
	t.Helper()
	test.NewTempApp(t)
	reg := dag.NewRegistry()
	for i := range 20 {
		reg.MustRegister(&dag.Component{
			ID: "step" + strconv.Itoa(i), Name: "Step " + strconv.Itoa(i), Category: "Steps",
			Outputs: []dag.PortDef{{Name: "out", Direction: dag.PortOutput}},
		})
	}
	c := NewDAGCanvas(dag.NewGraph("test", "Test"))
	c.Capabilities = AllCapabilities()
	p := NewPalette(reg, c)
	page := container.NewBorder(nil, nil, p, nil, c)
	w := test.NewTempWindow(t, page)
	w.SetPadded(false)
	w.Resize(geo.NewSize(800, 300))

	scroll, ok := test.TempWidgetRenderer(t, p).Objects()[0].(*container.Scroll)
	require.True(t, ok, "the palette's renderer holds its scroll")
	require.Greater(t, p.content.MinSize().Height, p.Size().Height+100, "the palette scrolls")

	h := &common.TouchHandler{}
	h.InitTouchHandler(&pageFinder{content: page})
	held := make(chan struct{}, 1)
	h.LongPressCallback = func(wid event.LongPressable, ev *event.PointEvent) {
		wid.LongPressed(ev)
		held <- struct{}{}
	}
	return c, scroll, h, held
}

// A swipe that starts on a palette entry scrolls the palette and adds no node
// (bellui#1296). Before, the entry took every touch drag, so a finger on an
// entry could not scroll the palette.
func TestPalette_SwipeOnEntryScrollsThePalette(t *testing.T) {
	c, scroll, h, _ := paletteBeside(t)
	on := geo.NewPos(100, 150)

	h.TapDown(on, 1)
	for i := 1; i <= 6; i++ {
		h.TapMove(on.SubtractXY(0, float32(i)*10), 1, touchDrag)
	}
	touchUp(h, 1, on.SubtractXY(0, 60))
	assert.Equal(t, float32(60), scroll.Offset.Y, "the palette scrolled under a swipe that started on an entry")
	assert.Empty(t, c.Graph.Nodes, "the swipe added no node")
}

// A long-press then a drag on an entry drags it out: released over the
// canvas, it adds the node there, and the palette does not scroll.
func TestPalette_LongPressThenDragDragsTheEntryOut(t *testing.T) {
	c, scroll, h, held := paletteBeside(t)
	on := geo.NewPos(100, 150)

	h.TapDown(on, 1)
	select {
	case <-held:
	case <-time.After(2 * time.Second):
		t.Fatal("no long-press fired on the entry: it is not LongPressable")
	}
	to := geo.NewPos(500, 150)
	for i := 1; i <= 10; i++ {
		f := float32(i) / 10
		h.TapMove(geo.NewPos(on.X+(to.X-on.X)*f, on.Y+(to.Y-on.Y)*f), 1, touchDrag)
	}
	touchUp(h, 1, to)
	assert.Zero(t, scroll.Offset.Y, "the drag after the long-press did not scroll the palette")
	assert.Len(t, c.Graph.Nodes, 1, "the entry dropped on the canvas added its node")
}
