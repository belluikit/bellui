//go:build (ci || android || ios || mobile) && !wasm

// The page Scroll is Draggable only where bellui's scroller_mobile.go is
// built, and these tests are about which Draggable a touch drag reaches.

package tables

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	intdriver "github.com/belluikit/bellui/internal/driver"
	"github.com/belluikit/bellui/internal/driver/common"
	"github.com/belluikit/bellui/test"

	"github.com/belluikit/bellui/x/tables/render"
)

var (
	_ event.DragClaimer = (*DataGrid)(nil)
	_ event.DragClaimer = (*render.HeaderWidget)(nil)
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

// touchSwipe drags contact id from one point to another in ten steps through
// the touch handler the mobile, embedded and desktop-touch drivers share.
func touchSwipe(h *common.TouchHandler, id int, from, to geo.Position) {
	drag := func(d event.Draggable, ev *event.DragEvent) { d.Dragged(ev) }
	h.TapDown(from, id)
	for i := 1; i <= 10; i++ {
		f := float32(i) / 10
		h.TapMove(geo.NewPos(from.X+(to.X-from.X)*f, from.Y+(to.Y-from.Y)*f), id, drag)
	}
	h.TapUp(to, id, func(event.Tappable, *event.PointEvent) {}, func(event.SecondaryTappable, *event.PointEvent) {},
		func(event.LongPressable, *event.PointEvent) {}, func(event.DoubleTappable, *event.PointEvent) {},
		func(d event.Draggable, _ *event.DragEvent) { d.DragEnd() })
}

// A 300x300 grid of 1000 rows at the top of a page 400 high whose content is
// 900 high (#1266): a swipe on the grid scrolls its body while the body can
// move that way, and the page once it cannot, from the body or the header.
func TestDataGrid_SwipeInAPageScrollsTheBodyThenThePage(t *testing.T) {
	g := newTestGrid(t, manyRows(1000), []*ColumnDef{{ColID: "n", Field: "n"}})
	rest := canvas.NewRectangle(color.Transparent)
	rest.SetMinSize(geo.NewSize(300, 600))
	page := container.NewVScroll(container.NewVBox(container.NewGridWrap(geo.NewSize(300, 300), g), rest))
	w := test.NewTempWindow(t, page)
	w.Resize(geo.NewSize(300, 400))
	page.Resize(geo.NewSize(300, 400))
	require.Equal(t, float32(300), g.Size().Height)
	h := &common.TouchHandler{}
	h.InitTouchHandler(&pageFinder{content: page})

	body := geo.NewPos(40, g.HeaderHeight()+150)
	header := geo.NewPos(40, g.HeaderHeight()/2)

	touchSwipe(h, 1, body, body.SubtractXY(0, 80))
	assert.Equal(t, float32(80), g.scrollContent.Offset().Y, "the body took the swipe")
	assert.Zero(t, page.Offset.Y, "the page did not")

	// At the body's end, the next upward swipe goes to the page.
	g.scrollContent.ScrollTo(geo.NewPos(0, 1e9))
	g.scrollContent.Scroll.Refresh()
	bottom := g.scrollContent.Offset().Y
	touchSwipe(h, 2, body, body.SubtractXY(0, 80))
	assert.Equal(t, bottom, g.scrollContent.Offset().Y)
	assert.Equal(t, float32(80), page.Offset.Y, "the page took the swipe the body could not follow")

	// From the header too: it resizes columns only after a mouse press, so on
	// touch it leaves the swipe to the grid, which leaves it to the page.
	page.ScrollToTop()
	touchSwipe(h, 3, header, header.SubtractXY(0, 60))
	assert.Equal(t, float32(60), page.Offset.Y, "a swipe on the header of a grid at its end scrolled the page")

	// Downward from the header, the body can move back, so it does.
	page.ScrollToTop()
	touchSwipe(h, 4, header, header.AddXY(0, 60))
	assert.Equal(t, bottom-60, g.scrollContent.Offset().Y, "a swipe on the header scrolled the body")
	assert.Zero(t, page.Offset.Y)
}

// A disabled grid claims every drag and ignores it: nothing scrolls over it,
// as over a disabled List (#1251).
func TestDataGrid_DisabledClaimsEveryDrag(t *testing.T) {
	g := newTestGrid(t, manyRows(1000), []*ColumnDef{{ColID: "n", Field: "n"}})
	layOut(t, g)
	g.Disable()
	assert.True(t, g.ClaimsDrag(geo.Delta{DY: -20}))
	assert.True(t, g.ClaimsDrag(geo.Delta{DX: -20}))
	g.Enable()
	assert.True(t, g.ClaimsDrag(geo.Delta{DY: -20}), "an enabled grid at its top claims an upward drag")
	assert.False(t, g.ClaimsDrag(geo.Delta{DY: 20}), "and leaves a downward one to the page")
}
