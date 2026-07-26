package widget

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/theme"
)

func TestNewScroll(t *testing.T) {
	rect := canvas.NewRectangle(color.Black)
	rect.SetMinSize(geo.NewSize(10, 10))
	scroll := NewScroll(rect)
	scroll.Resize(geo.NewSize(100, 100))
	barArea := cache.Renderer(scroll).(*scrollContainerRenderer).vertArea
	bar := cache.Renderer(barArea).(*scrollBarAreaRenderer).bar
	assert.Equal(t, float32(0), scroll.Offset.Y)
	// The hit area (and the bar widget inside it) is permanently the full bar
	// width on every device (#449); the idle visual is a thin strip drawn by
	// the bar's renderer, asserted in TestScrollBar_ThinVisualWideHit.
	assert.Equal(t, theme.ScrollBarSize(), barArea.Size().Width)
	assert.Equal(t, theme.ScrollBarSize(), bar.Size().Width)
	assert.Equal(t, float32(0), bar.Position().X)
	assert.Equal(t, geo.NewPos(100-theme.ScrollBarSize(), 0), barArea.Position())
}

func TestScrollBar_ThinVisualWideHit(t *testing.T) {
	// Originally the touch-only #17 behaviour; #449 extends the thin idle
	// visual over a wide invisible hit area to every device.
	require.Less(t, theme.ScrollBarSmallSize(), theme.ScrollBarSize())

	rect := canvas.NewRectangle(color.Black)
	rect.SetMinSize(geo.NewSize(1000, 1000))
	scroll := NewScroll(rect)
	scroll.Resize(geo.NewSize(100, 100))

	cont := cache.Renderer(scroll).(*scrollContainerRenderer)
	for _, area := range []*scrollBarArea{cont.vertArea, cont.horizArea} {
		areaR := cache.Renderer(area).(*scrollBarAreaRenderer)
		barR := cache.Renderer(areaR.bar).(*scrollBarRenderer)

		// The grabbable widget (and its area) stays full width so a finger can
		// hit it anywhere across the touch target.
		switch area.orientation {
		case scrollBarOrientationVertical:
			assert.Equal(t, theme.ScrollBarSize(), area.Size().Width, "vertical hit area width")
			assert.Equal(t, theme.ScrollBarSize(), areaR.bar.Size().Width, "vertical bar widget (hit) width")
			// The drawn bar is a thin strip flush to the trailing (right) edge.
			assert.Equal(t, theme.ScrollBarSmallSize(), barR.background.Size().Width, "vertical drawn bar width")
			assert.Equal(t, theme.ScrollBarSize()-theme.ScrollBarSmallSize(), barR.background.Position().X, "vertical drawn bar X")
		case scrollBarOrientationHorizontal:
			assert.Equal(t, theme.ScrollBarSize(), area.Size().Height, "horizontal hit area height")
			assert.Equal(t, theme.ScrollBarSize(), areaR.bar.Size().Height, "horizontal bar widget (hit) height")
			assert.Equal(t, theme.ScrollBarSmallSize(), barR.background.Size().Height, "horizontal drawn bar height")
			assert.Equal(t, theme.ScrollBarSize()-theme.ScrollBarSmallSize(), barR.background.Position().Y, "horizontal drawn bar Y")
		}

		// The full-width track is hidden so the wide hit area does not obscure
		// the content edge underneath it.
		assert.True(t, areaR.background.Hidden, "track should be hidden while the bar draws thin")
	}
}

func TestScrollContainer_MinSize(t *testing.T) {
	rect := canvas.NewRectangle(color.Black)
	rect.SetMinSize(geo.NewSize(500, 50))
	scroll := NewScroll(rect)
	assert.Equal(t, geo.NewSize(32, 32), scroll.MinSize())

	scrollMin := geo.NewSize(100, 100)
	scroll.SetMinSize(scrollMin)
	cache.Renderer(scroll).Layout(scroll.minSize)

	assert.Equal(t, scrollMin, scroll.MinSize())
	assert.Equal(t, geo.NewSize(500, 100), rect.Size())
	assert.Equal(t, float32(0), scroll.Offset.X)
	assert.Equal(t, float32(0), scroll.Offset.Y)
}

func TestScrollContainer_ScrollToTop(t *testing.T) {
	rect := canvas.NewRectangle(color.Black)
	rect.SetMinSize(geo.NewSize(500, 150))
	scroll := NewScroll(rect)
	scroll.Resize(geo.NewSize(25, 25))

	tmpOffset := geo.NewPos(25, 50)
	scroll.ScrollToOffset(tmpOffset)
	assert.Equal(t, tmpOffset, scroll.Offset)

	scroll.ScrollToTop()
	Y := scroll.Offset.Y
	assert.Equal(t, float32(0), Y)
}

func TestScrollContainer_ScrollToBottom(t *testing.T) {
	rect := canvas.NewRectangle(color.Black)
	rect.SetMinSize(geo.NewSize(500, 50))
	scroll := NewScroll(rect)
	scroll.ScrollToBottom()
	ExpectedY := float32(50)
	Y := scroll.Content.Size().Height - scroll.Size().Height
	assert.Equal(t, ExpectedY, Y)
}

func TestScrollContainer_MinSize_Direction(t *testing.T) {
	t.Run("Both", func(t *testing.T) {
		rect := canvas.NewRectangle(color.Black)
		rect.SetMinSize(geo.NewSize(100, 100))
		scroll := NewScroll(rect)
		size := scroll.MinSize()
		assert.Equal(t, float32(32), size.Height)
		assert.Equal(t, float32(32), size.Width)
	})
	t.Run("HorizontalOnly", func(t *testing.T) {
		rect := canvas.NewRectangle(color.Black)
		rect.SetMinSize(geo.NewSize(100, 100))
		scroll := NewHScroll(rect)
		size := scroll.MinSize()
		assert.Equal(t, float32(100), size.Height)
		assert.Equal(t, float32(32), size.Width)
	})
	t.Run("VerticalOnly", func(t *testing.T) {
		rect := canvas.NewRectangle(color.Black)
		rect.SetMinSize(geo.NewSize(100, 100))
		scroll := NewVScroll(rect)
		size := scroll.MinSize()
		assert.Equal(t, float32(32), size.Height)
		assert.Equal(t, float32(100), size.Width)
	})
}

func TestScrollContainer_OnScrolled(t *testing.T) {
	rect := canvas.NewRectangle(color.Black)
	rect.SetMinSize(geo.NewSize(1000, 1000))
	scroll := NewScroll(rect)
	scroll.Resize(geo.NewSize(100, 100))

	scrolled := false
	scroll.OnScrolled = func(geo.Position) {
		scrolled = true
	}

	scroll.Scrolled(&event.ScrollEvent{Scrolled: geo.NewDelta(-10, -10)})
	assert.True(t, scrolled)
	scrolled = false

	scroll.Scrolled(&event.ScrollEvent{Scrolled: geo.NewDelta(0, 0)})
	assert.False(t, scrolled) // don't repeat for no-change
}

func TestScrollContainer_AddScrollListener(t *testing.T) {
	rect := canvas.NewRectangle(color.Black)
	rect.SetMinSize(geo.NewSize(1000, 1000))
	scroll := NewScroll(rect)
	scroll.Resize(geo.NewSize(100, 100))

	var onScrolled, a, b int
	scroll.OnScrolled = func(geo.Position) { onScrolled++ }
	removeA := scroll.AddScrollListener(func(geo.Position) { a++ })
	removeB := scroll.AddScrollListener(func(geo.Position) { b++ })

	// OnScrolled and every listener fire on a move.
	scroll.Scrolled(&event.ScrollEvent{Scrolled: geo.NewDelta(-10, -10)})
	assert.Equal(t, 1, onScrolled)
	assert.Equal(t, 1, a)
	assert.Equal(t, 1, b)

	// No fire when the offset does not change.
	scroll.Scrolled(&event.ScrollEvent{Scrolled: geo.NewDelta(0, 0)})
	assert.Equal(t, 1, a)
	assert.Equal(t, 1, b)

	// Removal is order-independent: dropping the first-registered listener
	// leaves the others (and OnScrolled) intact.
	removeA()
	scroll.Scrolled(&event.ScrollEvent{Scrolled: geo.NewDelta(-10, -10)})
	assert.Equal(t, 1, a, "removed listener must not fire")
	assert.Equal(t, 2, b)
	assert.Equal(t, 2, onScrolled)

	// Remove is idempotent and the last listener detaches cleanly.
	removeA()
	removeB()
	scroll.Scrolled(&event.ScrollEvent{Scrolled: geo.NewDelta(-10, -10)})
	assert.Equal(t, 2, b)
	assert.Equal(t, 3, onScrolled)

	// A nil listener is a no-op that still returns a usable remove func.
	assert.NotPanics(t, func() { scroll.AddScrollListener(nil)() })
}

func TestScrollContainer_SetMinSize_Direction(t *testing.T) {
	t.Run("Both", func(t *testing.T) {
		rect := canvas.NewRectangle(color.Black)
		rect.SetMinSize(geo.NewSize(100, 100))
		scroll := NewScroll(rect)
		scroll.SetMinSize(geo.NewSize(50, 50))
		size := scroll.MinSize()
		assert.Equal(t, float32(50), size.Height)
		assert.Equal(t, float32(50), size.Width)
	})
	t.Run("HorizontalOnly", func(t *testing.T) {
		rect := canvas.NewRectangle(color.Black)
		rect.SetMinSize(geo.NewSize(100, 100))
		scroll := NewHScroll(rect)
		scroll.SetMinSize(geo.NewSize(50, 50))
		size := scroll.MinSize()
		assert.Equal(t, float32(100), size.Height)
		assert.Equal(t, float32(50), size.Width)
	})
	t.Run("VerticalOnly", func(t *testing.T) {
		rect := canvas.NewRectangle(color.Black)
		rect.SetMinSize(geo.NewSize(100, 100))
		scroll := NewVScroll(rect)
		scroll.SetMinSize(geo.NewSize(50, 50))
		size := scroll.MinSize()
		assert.Equal(t, float32(50), size.Height)
		assert.Equal(t, float32(100), size.Width)
	})
}

func TestScrollContainer_Resize_Direction(t *testing.T) {
	t.Run("Both", func(t *testing.T) {
		rect := canvas.NewRectangle(color.Black)
		rect.SetMinSize(geo.NewSize(100, 100))
		scroll := NewScroll(rect)
		scroll.Resize(scroll.MinSize())
		size := scroll.Size()
		assert.Equal(t, float32(32), size.Height)
		assert.Equal(t, float32(32), size.Width)
	})
	t.Run("HorizontalOnly", func(t *testing.T) {
		rect := canvas.NewRectangle(color.Black)
		rect.SetMinSize(geo.NewSize(100, 100))
		scroll := NewHScroll(rect)
		scroll.Resize(scroll.MinSize())
		size := scroll.Size()
		assert.Equal(t, float32(100), size.Height)
		assert.Equal(t, float32(32), size.Width)
	})
	t.Run("VerticalOnly", func(t *testing.T) {
		rect := canvas.NewRectangle(color.Black)
		rect.SetMinSize(geo.NewSize(100, 100))
		scroll := NewVScroll(rect)
		scroll.Resize(scroll.MinSize())
		size := scroll.Size()
		assert.Equal(t, float32(32), size.Height)
		assert.Equal(t, float32(100), size.Width)
	})
}

func TestScrollContainer_Refresh(t *testing.T) {
	rect := canvas.NewRectangle(color.Black)
	rect.SetMinSize(geo.NewSize(1000, 1000))
	scroll := NewScroll(rect)
	scroll.Resize(geo.NewSize(100, 100))
	assert.Equal(t, geo.NewSize(1000, 1000), rect.Size())
	scroll.Scrolled(&event.ScrollEvent{Scrolled: geo.NewDelta(-1000, -1000)})
	assert.Equal(t, float32(900), scroll.Offset.X)
	assert.Equal(t, float32(900), scroll.Offset.Y)
	assert.Equal(t, geo.NewSize(1000, 1000), rect.Size())
	rect.SetMinSize(geo.NewSize(500, 500))
	scroll.Refresh()
	assert.Equal(t, float32(400), scroll.Offset.X)
	assert.Equal(t, geo.NewSize(500, 500), rect.Size())

	rect2 := canvas.NewRectangle(color.White)
	scroll.Content = rect2
	scroll.Refresh()
	assert.Equal(t, rect2, cache.Renderer(scroll).Objects()[0])
}

func TestScrollContainer_Scrolled(t *testing.T) {
	rect := canvas.NewRectangle(color.Black)
	rect.SetMinSize(geo.NewSize(1000, 1000))
	scroll := NewScroll(rect)
	scroll.Resize(geo.NewSize(100, 100))
	assert.Equal(t, float32(0), scroll.Offset.X)
	assert.Equal(t, float32(0), scroll.Offset.Y)
	scroll.Scrolled(&event.ScrollEvent{Scrolled: geo.NewDelta(-10, -10)})
	assert.Equal(t, float32(10), scroll.Offset.X)
	assert.Equal(t, float32(10), scroll.Offset.Y)
}

func TestScrollContainer_Scrolled_Limit(t *testing.T) {
	rect := canvas.NewRectangle(color.Black)
	rect.SetMinSize(geo.NewSize(100, 100))
	scroll := NewScroll(rect)
	scroll.Resize(geo.NewSize(80, 80))
	scroll.Scrolled(&event.ScrollEvent{Scrolled: geo.NewDelta(-25, -25)})
	assert.Equal(t, float32(20), scroll.Offset.X)
}

func TestScrollContainer_Scrolled_Back(t *testing.T) {
	rect := canvas.NewRectangle(color.Black)
	rect.SetMinSize(geo.NewSize(1000, 1000))
	scroll := NewScroll(rect)
	scroll.Resize(geo.NewSize(100, 100))
	scroll.Offset.X = 10
	scroll.Offset.Y = 10
	scroll.Scrolled(&event.ScrollEvent{Scrolled: geo.NewDelta(10, 10)})
	assert.Equal(t, float32(0), scroll.Offset.X)
	assert.Equal(t, float32(0), scroll.Offset.Y)
}

func TestScrollContainer_Scrolled_ScrollNone(t *testing.T) {
	rect := canvas.NewRectangle(color.Black)
	rect.SetMinSize(geo.NewSize(1000, 1000))
	scroll := NewScroll(rect)
	scroll.Direction = ScrollNone
	scroll.Resize(geo.NewSize(100, 100))
	assert.Equal(t, float32(0), scroll.Offset.X)
	assert.Equal(t, float32(0), scroll.Offset.Y)
	scroll.Scrolled(&event.ScrollEvent{Scrolled: geo.NewDelta(-10, -10)})
	assert.Equal(t, float32(0), scroll.Offset.X)
	assert.Equal(t, float32(0), scroll.Offset.Y)
}

func TestScrollContainer_Scrolled_BackLimit(t *testing.T) {
	rect := canvas.NewRectangle(color.Black)
	scroll := NewScroll(rect)
	rect.SetMinSize(geo.NewSize(1000, 1000))
	scroll.Resize(geo.NewSize(100, 100))
	scroll.Offset.X = 10
	scroll.Offset.Y = 10
	scroll.Scrolled(&event.ScrollEvent{Scrolled: geo.NewDelta(20, 20)})
	assert.Equal(t, float32(0), scroll.Offset.X)
	assert.Equal(t, float32(0), scroll.Offset.Y)
}

func TestScrollContainer_Resize(t *testing.T) {
	rect := canvas.NewRectangle(color.Black)
	scroll := NewScroll(rect)
	rect.SetMinSize(geo.NewSize(100, 100))
	scroll.Resize(geo.NewSize(80, 80))
	scroll.Scrolled(&event.ScrollEvent{Scrolled: geo.NewDelta(-20, -20)})
	scroll.Resize(geo.NewSize(100, 100))
	assert.Equal(t, float32(0), scroll.Offset.X)
	assert.Equal(t, float32(0), scroll.Offset.Y)
}

func TestScrollContainer_ResizeOffset(t *testing.T) {
	rect := canvas.NewRectangle(color.Black)
	scroll := NewScroll(rect)
	rect.SetMinSize(geo.NewSize(100, 100))
	scroll.Resize(geo.NewSize(80, 80))
	scroll.Scrolled(&event.ScrollEvent{Scrolled: geo.NewDelta(-20, -20)})
	scroll.Resize(geo.NewSize(90, 90))
	assert.Equal(t, float32(10), scroll.Offset.X)
	assert.Equal(t, float32(10), scroll.Offset.Y)
}

func TestScrollContainer_ResizeExpand(t *testing.T) {
	rect := canvas.NewRectangle(color.Black)
	rect.SetMinSize(geo.NewSize(100, 100))
	scroll := NewScroll(rect)
	scroll.Resize(geo.NewSize(120, 140))
	assert.Equal(t, float32(120), rect.Size().Width)
	assert.Equal(t, float32(140), rect.Size().Height)
}

func TestScrollContainer_ScrollBarForSmallContentIsHidden(t *testing.T) {
	rect := canvas.NewRectangle(color.Black)
	rect.SetMinSize(geo.NewSize(100, 100))
	scroll := NewScroll(rect)
	scroll.Resize(geo.NewSize(100, 200))
	r := cache.Renderer(scroll).(*scrollContainerRenderer)
	assert.False(t, r.vertArea.Visible())
	assert.False(t, r.horizArea.Visible())
}

func TestScrollContainer_ShowHiddenScrollBarIfContentGrows(t *testing.T) {
	rect := canvas.NewRectangle(color.Black)
	scroll := NewScroll(rect)
	r := cache.Renderer(scroll).(*scrollContainerRenderer)
	rect.SetMinSize(geo.NewSize(100, 100))
	scroll.Resize(geo.NewSize(200, 200))
	require.False(t, r.horizArea.Visible())
	require.False(t, r.vertArea.Visible())
	rect.SetMinSize(geo.NewSize(300, 300))
	r.Layout(scroll.Size())
	assert.True(t, r.horizArea.Visible())
	assert.True(t, r.vertArea.Visible())
}

func TestScrollContainer_HideScrollBarIfContentShrinks(t *testing.T) {
	rect := canvas.NewRectangle(color.Black)
	scroll := NewScroll(rect)
	r := cache.Renderer(scroll).(*scrollContainerRenderer)
	rect.SetMinSize(geo.NewSize(300, 300))
	scroll.Resize(geo.NewSize(200, 200))
	require.True(t, r.horizArea.Visible())
	require.True(t, r.vertArea.Visible())
	rect.SetMinSize(geo.NewSize(200, 200))
	r.Layout(scroll.Size())
	assert.False(t, r.horizArea.Visible())
	assert.False(t, r.vertArea.Visible())
}

func TestScrollContainer_ScrollBarIsSmall(t *testing.T) {
	rect := canvas.NewRectangle(color.Black)
	scroll := NewScroll(rect)
	rect.SetMinSize(geo.NewSize(500, 500))
	scroll.Resize(geo.NewSize(100, 100))
	areaHoriz := cache.Renderer(scroll).(*scrollContainerRenderer).horizArea
	areaVert := cache.Renderer(scroll).(*scrollContainerRenderer).vertArea
	barHorizR := cache.Renderer(cache.Renderer(areaHoriz).(*scrollBarAreaRenderer).bar).(*scrollBarRenderer)
	barVertR := cache.Renderer(cache.Renderer(areaVert).(*scrollBarAreaRenderer).bar).(*scrollBarRenderer)
	require.True(t, areaHoriz.Visible())
	require.True(t, areaVert.Visible())
	require.Less(t, theme.ScrollBarSmallSize(), theme.ScrollBarSize())

	// The hit area stays full width (#449); the idle *visual* is the small
	// strip, drawn flush to the trailing edge inside the wide bar widget.
	assert.Equal(t, theme.ScrollBarSize(), areaHoriz.Size().Height)
	assert.Equal(t, theme.ScrollBarSize(), areaVert.Size().Width)
	assert.Equal(t, geo.NewPos(0, 100-theme.ScrollBarSize()), areaHoriz.Position())
	assert.Equal(t, geo.NewPos(100-theme.ScrollBarSize(), 0), areaVert.Position())
	assert.Equal(t, theme.ScrollBarSmallSize(), barHorizR.background.Size().Height)
	assert.Equal(t, theme.ScrollBarSmallSize(), barVertR.background.Size().Width)
	assert.Equal(t, theme.ScrollBarSize()-theme.ScrollBarSmallSize(), barHorizR.background.Position().Y)
	assert.Equal(t, theme.ScrollBarSize()-theme.ScrollBarSmallSize(), barVertR.background.Position().X)
}

func TestScrollContainer_ScrollBarGrowsAndShrinksOnMouseInAndMouseOut(t *testing.T) {
	if bellui.CurrentDevice().IsMobile() {
		t.Skip("hover has no meaning on touch devices; the bar visual stays thin permanently")
	}
	rect := canvas.NewRectangle(color.Black)
	scroll := NewScroll(rect)
	rect.SetMinSize(geo.NewSize(500, 500))
	scroll.Resize(geo.NewSize(100, 100))
	areaHoriz := cache.Renderer(scroll).(*scrollContainerRenderer).horizArea
	areaVert := cache.Renderer(scroll).(*scrollContainerRenderer).vertArea
	barHoriz := cache.Renderer(areaHoriz).(*scrollBarAreaRenderer).bar
	barVert := cache.Renderer(areaVert).(*scrollBarAreaRenderer).bar
	barHorizR := cache.Renderer(barHoriz).(*scrollBarRenderer)
	barVertR := cache.Renderer(barVert).(*scrollBarRenderer)
	require.True(t, barHoriz.Visible())
	require.Less(t, theme.ScrollBarSmallSize(), theme.ScrollBarSize())

	// The hit area and bar widget no longer change size on hover (#449) —
	// only the drawn visual toggles between the thin strip and full width,
	// so hovering can never re-wrap content that reserves bar space.
	require.Equal(t, theme.ScrollBarSize(), areaHoriz.Size().Height)
	require.Equal(t, theme.ScrollBarSize(), barHoriz.Size().Height)
	require.Equal(t, theme.ScrollBarSmallSize(), barHorizR.background.Size().Height)
	barHoriz.HoverIn(&event.HoverEvent{})
	assert.Equal(t, theme.ScrollBarSize(), areaHoriz.Size().Height, "hit area unchanged by hover")
	assert.Equal(t, theme.ScrollBarSize(), barHorizR.background.Size().Height, "visual grows to full width")
	assert.Equal(t, float32(0), barHorizR.background.Position().Y)
	barHoriz.HoverOut()
	assert.Equal(t, theme.ScrollBarSmallSize(), barHorizR.background.Size().Height, "visual shrinks back")
	assert.Equal(t, theme.ScrollBarSize()-theme.ScrollBarSmallSize(), barHorizR.background.Position().Y)

	require.True(t, barVert.Visible())
	require.Equal(t, theme.ScrollBarSize(), areaVert.Size().Width)
	require.Equal(t, theme.ScrollBarSize(), barVert.Size().Width)
	require.Equal(t, theme.ScrollBarSmallSize(), barVertR.background.Size().Width)
	barVert.HoverIn(&event.HoverEvent{})
	assert.Equal(t, theme.ScrollBarSize(), areaVert.Size().Width, "hit area unchanged by hover")
	assert.Equal(t, theme.ScrollBarSize(), barVertR.background.Size().Width, "visual grows to full width")
	assert.Equal(t, float32(0), barVertR.background.Position().X)
	barVert.HoverOut()
	assert.Equal(t, theme.ScrollBarSmallSize(), barVertR.background.Size().Width, "visual shrinks back")
	assert.Equal(t, theme.ScrollBarSize()-theme.ScrollBarSmallSize(), barVertR.background.Position().X)
}

func TestScrollContainer_ShowShadowOnLeftIfContentIsScrolled(t *testing.T) {
	rect := canvas.NewRectangle(color.Black)
	rect.SetMinSize(geo.NewSize(500, 100))
	scroll := NewScroll(rect)
	scroll.Resize(geo.NewSize(100, 100))
	r := cache.Renderer(scroll).(*scrollContainerRenderer)
	assert.False(t, r.leftShadow.Visible())
	assert.Equal(t, geo.NewPos(0, 0), r.leftShadow.Position())

	scroll.Scrolled(&event.ScrollEvent{Scrolled: geo.Delta{DX: -1}})
	assert.True(t, r.leftShadow.Visible())

	scroll.Scrolled(&event.ScrollEvent{Scrolled: geo.Delta{DX: 1}})
	assert.False(t, r.leftShadow.Visible())
}

func TestScrollContainer_ShowShadowOnRightIfContentCanScroll(t *testing.T) {
	rect := canvas.NewRectangle(color.Black)
	rect.SetMinSize(geo.NewSize(500, 100))
	scroll := NewScroll(rect)
	scroll.Resize(geo.NewSize(100, 100))
	r := cache.Renderer(scroll).(*scrollContainerRenderer)
	assert.True(t, r.rightShadow.Visible())
	assert.Equal(t, scroll.Size().Width, r.rightShadow.Position().X+r.rightShadow.Size().Width)

	scroll.Scrolled(&event.ScrollEvent{Scrolled: geo.Delta{DX: -400}})
	assert.False(t, r.rightShadow.Visible())

	scroll.Scrolled(&event.ScrollEvent{Scrolled: geo.Delta{DX: 100}})
	assert.True(t, r.rightShadow.Visible())
}

func TestScrollContainer_HideShadowsSuppressesEdgeShadows(t *testing.T) {
	rect := canvas.NewRectangle(color.Black)
	rect.SetMinSize(geo.NewSize(500, 500))
	scroll := NewScroll(rect)
	scroll.HideShadows = true
	scroll.Resize(geo.NewSize(100, 100))
	r := cache.Renderer(scroll).(*scrollContainerRenderer)

	// Content overflows on both axes, but no fade-shadow should ever show.
	assert.False(t, r.rightShadow.Visible())
	assert.False(t, r.bottomShadow.Visible())

	scroll.Scrolled(&event.ScrollEvent{Scrolled: geo.Delta{DX: -100, DY: -100}})
	assert.False(t, r.leftShadow.Visible())
	assert.False(t, r.rightShadow.Visible())
	assert.False(t, r.topShadow.Visible())
	assert.False(t, r.bottomShadow.Visible())
}

func TestScrollContainer_ShowShadowOnTopIfContentIsScrolled(t *testing.T) {
	rect := canvas.NewRectangle(color.Black)
	rect.SetMinSize(geo.NewSize(100, 500))
	scroll := NewScroll(rect)
	scroll.Resize(geo.NewSize(100, 100))
	r := cache.Renderer(scroll).(*scrollContainerRenderer)
	assert.False(t, r.topShadow.Visible())
	assert.Equal(t, geo.NewPos(0, 0), r.topShadow.Position())

	scroll.Scrolled(&event.ScrollEvent{Scrolled: geo.Delta{DY: -1}})
	assert.True(t, r.topShadow.Visible())

	scroll.Scrolled(&event.ScrollEvent{Scrolled: geo.Delta{DY: 1}})
	assert.False(t, r.topShadow.Visible())
}

func TestScrollContainer_ShowShadowOnBottomIfContentCanScroll(t *testing.T) {
	rect := canvas.NewRectangle(color.Black)
	rect.SetMinSize(geo.NewSize(100, 500))
	scroll := NewScroll(rect)
	scroll.Resize(geo.NewSize(100, 100))
	r := cache.Renderer(scroll).(*scrollContainerRenderer)
	assert.True(t, r.bottomShadow.Visible())
	assert.Equal(t, scroll.Size().Height, r.bottomShadow.Position().Y+r.bottomShadow.Size().Height)

	scroll.Scrolled(&event.ScrollEvent{Scrolled: geo.Delta{DY: -400}})
	assert.False(t, r.bottomShadow.Visible())

	scroll.Scrolled(&event.ScrollEvent{Scrolled: geo.Delta{DY: 100}})
	assert.True(t, r.bottomShadow.Visible())
}

func TestScrollContainer_ScrollHorizontallyWithVerticalMouseScroll(t *testing.T) {
	rect := canvas.NewRectangle(color.Black)
	rect.SetMinSize(geo.NewSize(1000, 50))
	scroll := NewScroll(rect)
	scroll.Resize(geo.NewSize(100, 100))
	assert.Equal(t, float32(0), scroll.Offset.X)
	assert.Equal(t, float32(0), scroll.Offset.Y)
	scroll.Scrolled(&event.ScrollEvent{Scrolled: geo.NewDelta(0, -10)})
	assert.Equal(t, float32(10), scroll.Offset.X)
	assert.Equal(t, float32(0), scroll.Offset.Y)

	t.Run("not if scroll event includes horizontal offset", func(t *testing.T) {
		rect := canvas.NewRectangle(color.Black)
		rect.SetMinSize(geo.NewSize(1000, 50))
		scroll := NewScroll(rect)
		scroll.Resize(geo.NewSize(100, 100))
		assert.Equal(t, float32(0), scroll.Offset.X)
		assert.Equal(t, float32(0), scroll.Offset.Y)
		scroll.Scrolled(&event.ScrollEvent{Scrolled: geo.NewDelta(-20, -40)})
		assert.Equal(t, float32(20), scroll.Offset.X)
		assert.Equal(t, float32(0), scroll.Offset.Y)
	})

	t.Run("not if content is vertically scrollable", func(t *testing.T) {
		rect := canvas.NewRectangle(color.Black)
		rect.SetMinSize(geo.NewSize(1000, 1000))
		scroll := NewScroll(rect)
		scroll.Resize(geo.NewSize(100, 100))
		assert.Equal(t, float32(0), scroll.Offset.X)
		assert.Equal(t, float32(0), scroll.Offset.Y)
		scroll.Scrolled(&event.ScrollEvent{Scrolled: geo.NewDelta(0, -10)})
		assert.Equal(t, float32(0), scroll.Offset.X)
		assert.Equal(t, float32(10), scroll.Offset.Y)
	})
}

func TestScrollBarRenderer_BarSize(t *testing.T) {
	rect := canvas.NewRectangle(color.Black)
	rect.SetMinSize(geo.NewSize(100, 100))
	scroll := NewScroll(rect)
	scroll.Resize(geo.NewSize(100, 100))
	areaHoriz := cache.Renderer(cache.Renderer(scroll).(*scrollContainerRenderer).horizArea).(*scrollBarAreaRenderer)
	areaVert := cache.Renderer(cache.Renderer(scroll).(*scrollContainerRenderer).vertArea).(*scrollBarAreaRenderer)

	assert.Equal(t, float32(100), areaHoriz.bar.Size().Width)
	assert.Equal(t, float32(100), areaVert.bar.Size().Height)

	// resize so the content overflows: the thumb is the viewport's
	// proportional share of it (80 * 80/100).
	scroll.Resize(geo.NewSize(80, 80))
	assert.Equal(t, float32(64), areaHoriz.bar.Size().Width)
	assert.Equal(t, float32(64), areaVert.bar.Size().Height)
}

func TestScrollBarRenderer_MinimumThumbLength(t *testing.T) {
	// On very long content the proportional thumb would be sub-pixel; the
	// theme floor keeps it grabbable (#449).
	rect := canvas.NewRectangle(color.Black)
	rect.SetMinSize(geo.NewSize(100, 100000))
	scroll := NewScroll(rect)
	scroll.Resize(geo.NewSize(100, 100))
	areaVert := cache.Renderer(cache.Renderer(scroll).(*scrollContainerRenderer).vertArea).(*scrollBarAreaRenderer)

	minThumb := theme.Current().Size(theme.SizeNameScrollBarMinThumb)
	require.Greater(t, minThumb, float32(0))
	assert.Equal(t, minThumb, areaVert.bar.Size().Height)

	// The clamped thumb still maps the full scroll range: dragging to the end
	// of the track reaches the end of the content.
	bar := areaVert.bar
	bar.Dragged(&event.DragEvent{Dragged: geo.Delta{DY: 100}})
	assert.Equal(t, float32(100000-100), scroll.Offset.Y)

	// A short scroll area caps the floor at half the track, so there is
	// always room left to drag the thumb.
	scroll.Resize(geo.NewSize(100, minThumb))
	assert.Equal(t, minThumb/2, areaVert.bar.Size().Height)
}

func TestScrollContainerRenderer_LimitBarSize(t *testing.T) {
	rect := canvas.NewRectangle(color.Black)
	rect.SetMinSize(geo.NewSize(100, 100))
	scroll := NewScroll(rect)
	scroll.Resize(geo.NewSize(120, 120))
	areaHoriz := cache.Renderer(cache.Renderer(scroll).(*scrollContainerRenderer).horizArea).(*scrollBarAreaRenderer)
	areaVert := cache.Renderer(cache.Renderer(scroll).(*scrollContainerRenderer).vertArea).(*scrollBarAreaRenderer)

	assert.Equal(t, float32(120), areaHoriz.bar.Size().Width)
	assert.Equal(t, float32(120), areaVert.bar.Size().Height)
}

func TestScrollContainerRenderer_Direction(t *testing.T) {
	t.Run("Both", func(t *testing.T) {
		rect := canvas.NewRectangle(color.Black)
		rect.SetMinSize(geo.NewSize(100, 100))
		scroll := NewScroll(rect)
		r := cache.Renderer(scroll).(*scrollContainerRenderer)
		assert.NotNil(t, r.vertArea)
		assert.NotNil(t, r.topShadow)
		assert.NotNil(t, r.bottomShadow)
		assert.NotNil(t, r.horizArea)
		assert.NotNil(t, r.leftShadow)
		assert.NotNil(t, r.rightShadow)
	})
	t.Run("HorizontalOnly", func(t *testing.T) {
		rect := canvas.NewRectangle(color.Black)
		rect.SetMinSize(geo.NewSize(100, 100))
		scroll := NewHScroll(rect)
		r := cache.Renderer(scroll).(*scrollContainerRenderer)
		assert.NotNil(t, r.vertArea)
		assert.False(t, r.vertArea.Visible())
		assert.NotNil(t, r.topShadow)
		assert.False(t, r.topShadow.Visible())
		assert.NotNil(t, r.bottomShadow)
		assert.False(t, r.bottomShadow.Visible())
		assert.NotNil(t, r.horizArea)
		assert.NotNil(t, r.leftShadow)
		assert.NotNil(t, r.rightShadow)
	})
	t.Run("VerticalOnly", func(t *testing.T) {
		rect := canvas.NewRectangle(color.Black)
		rect.SetMinSize(geo.NewSize(100, 100))
		scroll := NewVScroll(rect)
		r := cache.Renderer(scroll).(*scrollContainerRenderer)
		assert.NotNil(t, r.vertArea)
		assert.NotNil(t, r.topShadow)
		assert.NotNil(t, r.bottomShadow)
		assert.NotNil(t, r.horizArea)
		assert.False(t, r.horizArea.Visible())
		assert.NotNil(t, r.leftShadow)
		assert.False(t, r.leftShadow.Visible())
		assert.NotNil(t, r.rightShadow)
		assert.False(t, r.rightShadow.Visible())
	})
}

func TestScrollContainerRenderer_MinSize_Direction(t *testing.T) {
	t.Run("Both", func(t *testing.T) {
		rect := canvas.NewRectangle(color.Black)
		rect.SetMinSize(geo.NewSize(100, 100))
		scroll := NewScroll(rect)
		size := cache.Renderer(scroll).MinSize()
		assert.Equal(t, float32(32), size.Height)
		assert.Equal(t, float32(32), size.Width)
	})
	t.Run("HorizontalOnly", func(t *testing.T) {
		rect := canvas.NewRectangle(color.Black)
		rect.SetMinSize(geo.NewSize(100, 100))
		scroll := NewHScroll(rect)
		size := cache.Renderer(scroll).MinSize()
		assert.Equal(t, float32(100), size.Height)
		assert.Equal(t, float32(32), size.Width)
	})
	t.Run("VerticalOnly", func(t *testing.T) {
		rect := canvas.NewRectangle(color.Black)
		rect.SetMinSize(geo.NewSize(100, 100))
		scroll := NewVScroll(rect)
		size := cache.Renderer(scroll).MinSize()
		assert.Equal(t, float32(32), size.Height)
		assert.Equal(t, float32(100), size.Width)
	})
}

func TestScrollContainerRenderer_SetMinSize_Direction(t *testing.T) {
	t.Run("Both", func(t *testing.T) {
		rect := canvas.NewRectangle(color.Black)
		rect.SetMinSize(geo.NewSize(100, 100))
		scroll := NewScroll(rect)
		scroll.SetMinSize(geo.NewSize(50, 50))
		size := cache.Renderer(scroll).MinSize()
		assert.Equal(t, float32(50), size.Height)
		assert.Equal(t, float32(50), size.Width)
	})
	t.Run("HorizontalOnly", func(t *testing.T) {
		rect := canvas.NewRectangle(color.Black)
		rect.SetMinSize(geo.NewSize(100, 100))
		scroll := NewHScroll(rect)
		scroll.SetMinSize(geo.NewSize(50, 50))
		size := cache.Renderer(scroll).MinSize()
		assert.Equal(t, float32(100), size.Height)
		assert.Equal(t, float32(50), size.Width)
	})
	t.Run("VerticalOnly", func(t *testing.T) {
		rect := canvas.NewRectangle(color.Black)
		rect.SetMinSize(geo.NewSize(100, 100))
		scroll := NewVScroll(rect)
		scroll.SetMinSize(geo.NewSize(50, 50))
		size := cache.Renderer(scroll).MinSize()
		assert.Equal(t, float32(50), size.Height)
		assert.Equal(t, float32(100), size.Width)
	})
}

// Drag tests use a 200 viewport over 500 content: the proportional thumb is
// 200*(200/500) = 80, above the minimum-thumb floor (#449), so the mapping is
// track = 200-80 = 120 over a 300 scroll range — a factor of exactly 2.5.

func TestScrollBar_Dragged_ClickedInside(t *testing.T) {
	rect := canvas.NewRectangle(color.Black)
	rect.SetMinSize(geo.NewSize(500, 500))
	scroll := NewScroll(rect)
	scroll.Resize(geo.NewSize(200, 200))
	scrollBarHoriz := cache.Renderer(cache.Renderer(scroll).(*scrollContainerRenderer).horizArea).(*scrollBarAreaRenderer).bar
	scrollBarVert := cache.Renderer(cache.Renderer(scroll).(*scrollContainerRenderer).vertArea).(*scrollBarAreaRenderer).bar

	// Create drag event with starting position inside scroll rectangle area
	dragEvent := event.DragEvent{Dragged: geo.Delta{DX: 20}}
	assert.Equal(t, float32(0), scroll.Offset.X)
	scrollBarHoriz.Dragged(&dragEvent)
	assert.Equal(t, float32(50), scroll.Offset.X)

	dragEvent = event.DragEvent{Dragged: geo.Delta{DY: 20}}
	assert.Equal(t, float32(0), scroll.Offset.Y)
	scrollBarVert.Dragged(&dragEvent)
	assert.Equal(t, float32(50), scroll.Offset.Y)
}

func TestScrollBar_DraggedBack_ClickedInside(t *testing.T) {
	rect := canvas.NewRectangle(color.Black)
	rect.SetMinSize(geo.NewSize(500, 500))
	scroll := NewScroll(rect)
	scroll.Resize(geo.NewSize(200, 200))
	scrollBarHoriz := cache.Renderer(cache.Renderer(scroll).(*scrollContainerRenderer).horizArea).(*scrollBarAreaRenderer).bar
	scrollBarVert := cache.Renderer(cache.Renderer(scroll).(*scrollContainerRenderer).vertArea).(*scrollBarAreaRenderer).bar

	// Drag forward
	dragEvent := event.DragEvent{Dragged: geo.Delta{DX: 20}}
	scrollBarHoriz.Dragged(&dragEvent)
	dragEvent = event.DragEvent{Dragged: geo.Delta{DY: 20}}
	scrollBarVert.Dragged(&dragEvent)

	// Drag back
	dragEvent = event.DragEvent{Dragged: geo.Delta{DX: -10}}
	assert.Equal(t, float32(50), scroll.Offset.X)
	scrollBarHoriz.Dragged(&dragEvent)
	assert.Equal(t, float32(25), scroll.Offset.X)

	dragEvent = event.DragEvent{Dragged: geo.Delta{DY: -10}}
	assert.Equal(t, float32(50), scroll.Offset.Y)
	scrollBarVert.Dragged(&dragEvent)
	assert.Equal(t, float32(25), scroll.Offset.Y)
}

func TestScrollBar_Dragged_Limit(t *testing.T) {
	rect := canvas.NewRectangle(color.Black)
	rect.SetMinSize(geo.NewSize(500, 500))
	scroll := NewScroll(rect)
	scroll.Resize(geo.NewSize(200, 200))
	scrollBarHoriz := cache.Renderer(cache.Renderer(scroll).(*scrollContainerRenderer).horizArea).(*scrollBarAreaRenderer).bar
	scrollBarVert := cache.Renderer(cache.Renderer(scroll).(*scrollContainerRenderer).vertArea).(*scrollBarAreaRenderer).bar

	// Drag over limit
	dragEvent := event.DragEvent{Dragged: geo.Delta{DX: 2000}}
	assert.Equal(t, float32(0), scroll.Offset.X)
	scrollBarHoriz.Dragged(&dragEvent)
	assert.Equal(t, float32(300), scroll.Offset.X)

	dragEvent = event.DragEvent{Dragged: geo.Delta{DY: 2000}}
	assert.Equal(t, float32(0), scroll.Offset.Y)
	scrollBarVert.Dragged(&dragEvent)
	assert.Equal(t, float32(300), scroll.Offset.Y)

	// Drag again
	dragEvent = event.DragEvent{Dragged: geo.Delta{DX: 100}}
	// Offset doesn't go over limit
	assert.Equal(t, float32(300), scroll.Offset.X)
	scrollBarHoriz.Dragged(&dragEvent)
	assert.Equal(t, float32(300), scroll.Offset.X)

	dragEvent = event.DragEvent{Dragged: geo.Delta{DY: 100}}
	// Offset doesn't go over limit
	assert.Equal(t, float32(300), scroll.Offset.Y)
	scrollBarVert.Dragged(&dragEvent)
	assert.Equal(t, float32(300), scroll.Offset.Y)

	// Drag back (still outside limit: accumulated 2000+100-1000 > track)
	dragEvent = event.DragEvent{Dragged: geo.Delta{DX: -1000}}
	scrollBarHoriz.Dragged(&dragEvent)
	assert.Equal(t, float32(300), scroll.Offset.X)

	dragEvent = event.DragEvent{Dragged: geo.Delta{DY: -1000}}
	scrollBarVert.Dragged(&dragEvent)
	assert.Equal(t, float32(300), scroll.Offset.Y)

	// Drag back (inside limit: accumulated 60 on the 120 track → offset 150)
	dragEvent = event.DragEvent{Dragged: geo.Delta{DX: -1040}}
	scrollBarHoriz.Dragged(&dragEvent)
	assert.Equal(t, float32(150), scroll.Offset.X)

	dragEvent = event.DragEvent{Dragged: geo.Delta{DY: -1040}}
	scrollBarVert.Dragged(&dragEvent)
	assert.Equal(t, float32(150), scroll.Offset.Y)
}

func TestScrollBar_Dragged_BackLimit(t *testing.T) {
	rect := canvas.NewRectangle(color.Black)
	rect.SetMinSize(geo.NewSize(500, 500))
	scroll := NewScroll(rect)
	scroll.Resize(geo.NewSize(200, 200))
	scrollBarHoriz := cache.Renderer(cache.Renderer(scroll).(*scrollContainerRenderer).horizArea).(*scrollBarAreaRenderer).bar
	scrollBarVert := cache.Renderer(cache.Renderer(scroll).(*scrollContainerRenderer).vertArea).(*scrollBarAreaRenderer).bar

	// Drag over back limit
	dragEvent := event.DragEvent{Dragged: geo.Delta{DX: -1000}}
	// Offset doesn't go over limit
	assert.Equal(t, float32(0), scroll.Offset.X)
	scrollBarHoriz.Dragged(&dragEvent)
	assert.Equal(t, float32(0), scroll.Offset.X)

	dragEvent = event.DragEvent{Dragged: geo.Delta{DY: -1000}}
	// Offset doesn't go over limit
	assert.Equal(t, float32(0), scroll.Offset.Y)
	scrollBarVert.Dragged(&dragEvent)
	assert.Equal(t, float32(0), scroll.Offset.Y)

	// Drag (still outside limit)
	dragEvent = event.DragEvent{Dragged: geo.Delta{DX: 500}}
	scrollBarHoriz.Dragged(&dragEvent)
	assert.Equal(t, float32(0), scroll.Offset.X)

	dragEvent = event.DragEvent{Dragged: geo.Delta{DY: 500}}
	scrollBarVert.Dragged(&dragEvent)
	assert.Equal(t, float32(0), scroll.Offset.Y)

	// Drag (inside limit: accumulated 20 on the 120 track → offset 50)
	dragEvent = event.DragEvent{Dragged: geo.Delta{DX: 520}}
	scrollBarHoriz.Dragged(&dragEvent)
	assert.Equal(t, float32(50), scroll.Offset.X)

	dragEvent = event.DragEvent{Dragged: geo.Delta{DY: 520}}
	scrollBarVert.Dragged(&dragEvent)
	assert.Equal(t, float32(50), scroll.Offset.Y)
}

func TestScrollBar_DraggedWithNonZeroStartPosition(t *testing.T) {
	rect := canvas.NewRectangle(color.Black)
	rect.SetMinSize(geo.NewSize(500, 500))
	scroll := NewScroll(rect)
	scroll.Resize(geo.NewSize(200, 200))
	scrollBarHoriz := cache.Renderer(cache.Renderer(scroll).(*scrollContainerRenderer).horizArea).(*scrollBarAreaRenderer).bar
	scrollBarVert := cache.Renderer(cache.Renderer(scroll).(*scrollContainerRenderer).vertArea).(*scrollBarAreaRenderer).bar

	dragEvent := event.DragEvent{Dragged: geo.Delta{DX: 50}}
	assert.Equal(t, float32(0), scroll.Offset.X)
	scrollBarHoriz.Dragged(&dragEvent)
	assert.Equal(t, float32(125), scroll.Offset.X)

	dragEvent = event.DragEvent{Dragged: geo.Delta{DY: 50}}
	assert.Equal(t, float32(0), scroll.Offset.Y)
	scrollBarVert.Dragged(&dragEvent)
	assert.Equal(t, float32(125), scroll.Offset.Y)

	// Drag again (after releasing mouse button)
	dragEvent = event.DragEvent{Dragged: geo.Delta{DX: 20}}
	scrollBarHoriz.Dragged(&dragEvent)
	assert.Equal(t, float32(175), scroll.Offset.X)

	dragEvent = event.DragEvent{Dragged: geo.Delta{DY: 20}}
	scrollBarVert.Dragged(&dragEvent)
	assert.Equal(t, float32(175), scroll.Offset.Y)
}

func TestScrollBar_LargeHandleWhileInDrag(t *testing.T) {
	if bellui.CurrentDevice().IsMobile() {
		t.Skip("touch scrollbars render at full width permanently; idle/hover geometry is desktop-only")
	}
	rect := canvas.NewRectangle(color.Black)
	rect.SetMinSize(geo.NewSize(1000, 1000))
	scroll := NewScroll(rect)
	scroll.Resize(geo.NewSize(200, 200))
	scrollBarHoriz := cache.Renderer(cache.Renderer(scroll).(*scrollContainerRenderer).horizArea).(*scrollBarAreaRenderer).bar

	// Make sure that hovering makes the bar large.
	mouseEvent := &event.HoverEvent{}
	assert.False(t, scrollBarHoriz.area.expanded())
	scrollBarHoriz.HoverIn(mouseEvent)
	assert.True(t, scrollBarHoriz.area.expanded())
	scrollBarHoriz.HoverOut()
	assert.False(t, scrollBarHoriz.area.expanded())

	// Make sure that the bar stays large when dragging, even if the mouse leaves the bar.
	dragEvent := &event.DragEvent{Dragged: geo.Delta{DX: 10}}
	scrollBarHoriz.Dragged(dragEvent)
	assert.True(t, scrollBarHoriz.area.expanded())
	scrollBarHoriz.HoverOut()
	assert.True(t, scrollBarHoriz.area.expanded())
	scrollBarHoriz.DragEnd()
	scrollBarHoriz.HoverOut()
	assert.False(t, scrollBarHoriz.area.expanded())
}

func TestScrollContainer_TapToScroll(t *testing.T) {
	rect := canvas.NewRectangle(color.Transparent)
	rect.SetMinSize(geo.NewSize(250, 250))
	s := NewScroll(rect)
	s.Resize(geo.NewSize(100, 100))
	r := s.CreateRenderer().(*scrollContainerRenderer)

	// Testing the vertical scroll bar...
	// tapping on the scroll bar itself does nothing
	r.vertArea.HoverIn(nil)
	r.vertArea.Tapped(&event.PointEvent{
		Position: geo.NewPos(2, 2),
	})
	assert.Equal(t, geo.NewPos(0, 0), s.Offset)

	// tapping below the bar jumps so the bar centers on the tap point:
	// bar length = 100 * (100/250) = 40, tap at 50 → bar offset 30 of a
	// 60 track → content offset 0.5 * (250-100) = 75
	r.vertArea.Tapped(&event.PointEvent{
		Position: geo.NewPos(2, 50),
	})
	assert.Equal(t, float32(75), s.Offset.Y)
	oldY := s.Offset.Y

	// tapping above the bar scrolls up
	r.Refresh() // updates bar location
	r.vertArea.Tapped(&event.PointEvent{
		Position: geo.NewPos(2, 2),
	})
	assert.Less(t, s.Offset.Y, oldY)

	// Testing the horizontal scroll bar...
	s.Offset = geo.NewPos(0, 0)
	s.Refresh()
	r.horizArea.HoverIn(nil)

	// tapping on the scroll bar itself does nothing
	r.horizArea.Tapped(&event.PointEvent{
		Position: geo.NewPos(2, 2),
	})
	assert.Equal(t, geo.NewPos(0, 0), s.Offset)

	// tapping right of bar scrolls right
	r.horizArea.Tapped(&event.PointEvent{
		Position: geo.NewPos(50, 2),
	})
	assert.Greater(t, s.Offset.X, float32(0))
	oldX := s.Offset.X

	// tapping left of the bar scrolls left
	r.Refresh() // updates bar location
	r.horizArea.Tapped(&event.PointEvent{
		Position: geo.NewPos(2, 2),
	})
	assert.Less(t, s.Offset.X, oldX)
}

func TestScrollContainer_DragTrackToScroll(t *testing.T) {
	rect := canvas.NewRectangle(color.Transparent)
	rect.SetMinSize(geo.NewSize(250, 250))
	s := NewScroll(rect)
	s.Resize(geo.NewSize(100, 100))
	r := s.CreateRenderer().(*scrollContainerRenderer)

	// a drag starting on the vertical track centers the bar at the pointer
	// (same geometry as the tap test: pointer at 50 → content offset 75)
	r.vertArea.Dragged(&event.DragEvent{
		PointEvent: event.PointEvent{Position: geo.NewPos(2, 50)},
	})
	assert.Equal(t, float32(75), s.Offset.Y)
	assert.True(t, r.vertArea.isDragging)

	// further drag events keep the bar tracking the pointer, clamped at the end
	r.vertArea.Dragged(&event.DragEvent{
		PointEvent: event.PointEvent{Position: geo.NewPos(2, 95)},
	})
	assert.Equal(t, float32(150), s.Offset.Y)

	r.vertArea.DragEnd()
	assert.False(t, r.vertArea.isDragging)

	// same for the horizontal track
	r.horizArea.Dragged(&event.DragEvent{
		PointEvent: event.PointEvent{Position: geo.NewPos(50, 2)},
	})
	assert.Equal(t, float32(75), s.Offset.X)
	r.horizArea.DragEnd()
}
