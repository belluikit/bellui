package widget_test

import (
	"image/color"
	"testing"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"

	"github.com/stretchr/testify/assert"
)

func TestScrollContainer_Theme(t *testing.T) {
	rect := canvas.NewRectangle(color.Transparent)
	rect.SetMinSize(geo.NewSize(250, 250))
	scroll := widget.NewScroll(rect)

	w := test.NewTempWindow(t, scroll)
	w.SetPadded(false)
	w.Resize(geo.NewSize(100, 100))
	test.AssertImageMatches(t, "scroll/theme_initial.png", w.Canvas().Capture())

	test.WithTestTheme(t, func() {
		time.Sleep(100 * time.Millisecond)
		scroll.Refresh()
		test.AssertImageMatches(t, "scroll/theme_changed.png", w.Canvas().Capture())
	})
}

func TestScrollContainer_ThemeOverride(t *testing.T) {
	rect := canvas.NewRectangle(color.Transparent)
	rect.SetMinSize(geo.NewSize(250, 250))
	scroll := widget.NewScroll(rect)
	scroll.Resize(geo.NewSize(100, 100))

	w := test.NewTempWindow(t, scroll)
	w.SetPadded(false)
	w.Resize(geo.NewSize(100, 100))
	test.ApplyTheme(t, test.NewTheme())
	test.AssertImageMatches(t, "scroll/theme_changed.png", w.Canvas().Capture())

	normal := test.Theme()
	bg := canvas.NewRectangle(normal.Color(theme.ColorNameBackground, theme.VariantDark))
	w.SetContent(container.NewStack(bg, container.NewThemeOverride(scroll, normal)))
	w.Resize(geo.NewSize(100, 100))
	// TODO why is this off by a 1bit RGB difference?
	// test.AssertImageMatches(t, "scroll/theme_initial.png", w.Canvas().Capture())
}

func TestScroll_TryScrolled_ContentFits(t *testing.T) {
	// When content fits entirely within the scroll container, TryScrolled should return false.
	rect := canvas.NewRectangle(color.Transparent)
	rect.SetMinSize(geo.NewSize(50, 50))
	scroll := widget.NewScroll(rect)
	scroll.Resize(geo.NewSize(100, 100)) // bigger than content

	ev := &event.ScrollEvent{Scrolled: geo.NewDelta(0, -10)}
	assert.False(t, scroll.TryScrolled(ev), "should not consume scroll when content fits")
}

func TestScroll_TryScrolled_AtBoundary(t *testing.T) {
	rect := canvas.NewRectangle(color.Transparent)
	rect.SetMinSize(geo.NewSize(100, 300))
	scroll := widget.NewScroll(rect)
	scroll.Resize(geo.NewSize(100, 100))

	// At top, scrolling up (positive DY means scroll up) should not be consumed.
	ev := &event.ScrollEvent{Scrolled: geo.NewDelta(0, 10)}
	assert.False(t, scroll.TryScrolled(ev), "should not consume scroll up when at top")

	// Scrolling down (negative DY) should be consumed when not at bottom.
	ev = &event.ScrollEvent{Scrolled: geo.NewDelta(0, -10)}
	assert.True(t, scroll.TryScrolled(ev), "should consume scroll down when not at bottom")

	// Scroll to the very bottom.
	scroll.ScrollToBottom()
	ev = &event.ScrollEvent{Scrolled: geo.NewDelta(0, -10)}
	assert.False(t, scroll.TryScrolled(ev), "should not consume scroll down when at bottom")

	// At bottom, scrolling up should be consumed.
	ev = &event.ScrollEvent{Scrolled: geo.NewDelta(0, 10)}
	assert.True(t, scroll.TryScrolled(ev), "should consume scroll up when at bottom")
}

func TestScroll_TryScrolled_DirectionNone(t *testing.T) {
	rect := canvas.NewRectangle(color.Transparent)
	rect.SetMinSize(geo.NewSize(100, 300))
	scroll := widget.NewScroll(rect)
	scroll.Direction = widget.ScrollNone
	scroll.Resize(geo.NewSize(100, 100))

	ev := &event.ScrollEvent{Scrolled: geo.NewDelta(0, -10)}
	assert.False(t, scroll.TryScrolled(ev), "ScrollNone should never consume scroll events")
}

func TestScroll_TryScrolled_NilContent(t *testing.T) {
	// Create a scroll with content, then remove it (as Entry does when scroll is disabled).
	rect := canvas.NewRectangle(color.Transparent)
	rect.SetMinSize(geo.NewSize(100, 300))
	scroll := widget.NewScroll(rect)
	scroll.Resize(geo.NewSize(100, 100))
	scroll.Content = nil

	ev := &event.ScrollEvent{Scrolled: geo.NewDelta(0, -10)}
	assert.False(t, scroll.TryScrolled(ev), "nil content should not consume scroll events")
}

func TestScroll_TryScrolled_HorizontalOnly(t *testing.T) {
	rect := canvas.NewRectangle(color.Transparent)
	rect.SetMinSize(geo.NewSize(300, 50))
	scroll := widget.NewHScroll(rect)
	scroll.Resize(geo.NewSize(100, 100))

	// Horizontal scroll should be consumed.
	ev := &event.ScrollEvent{Scrolled: geo.NewDelta(-10, 0)}
	assert.True(t, scroll.TryScrolled(ev), "should consume horizontal scroll")

	// Vertical-only input on horizontal-only scroller: content is wider than
	// container and fits vertically, so the axis swap redirects vertical to horizontal.
	ev = &event.ScrollEvent{Scrolled: geo.NewDelta(0, -10)}
	assert.True(t, scroll.TryScrolled(ev), "vertical input should be swapped to horizontal for wide content")
}

func TestScroll_TryScrolled_VerticalOnly(t *testing.T) {
	rect := canvas.NewRectangle(color.Transparent)
	rect.SetMinSize(geo.NewSize(50, 300))
	scroll := widget.NewVScroll(rect)
	scroll.Resize(geo.NewSize(100, 100))

	// Vertical scroll should be consumed.
	ev := &event.ScrollEvent{Scrolled: geo.NewDelta(0, -10)}
	assert.True(t, scroll.TryScrolled(ev), "should consume vertical scroll")

	// Horizontal input on vertical-only scroller: content fits horizontally,
	// no axis swap applies, and horizontal scrolling has no effect.
	ev = &event.ScrollEvent{Scrolled: geo.NewDelta(-10, 0)}
	assert.False(t, scroll.TryScrolled(ev), "horizontal input should not be consumed by vertical-only scroller")
}

func TestScroll_TryScrolled_ReserveBarSpaceFalse(t *testing.T) {
	rect := canvas.NewRectangle(color.Transparent)
	rect.SetMinSize(geo.NewSize(100, 300))
	scroll := widget.NewScroll(rect)
	scroll.ReserveBarSpace = false
	scroll.Resize(geo.NewSize(100, 100))

	// Should still scroll normally with ReserveBarSpace disabled.
	ev := &event.ScrollEvent{Scrolled: geo.NewDelta(0, -10)}
	assert.True(t, scroll.TryScrolled(ev), "should consume scroll even with ReserveBarSpace=false")

	scroll.ScrollToBottom()
	ev = &event.ScrollEvent{Scrolled: geo.NewDelta(0, -10)}
	assert.False(t, scroll.TryScrolled(ev), "should propagate at bottom boundary")
}

func TestScroll_ReserveBarSpace_Layout(t *testing.T) {
	if bellui.CurrentDevice().IsMobile() {
		t.Skip("on touch the scrollbar overlays content instead of reserving layout space")
	}
	rect := canvas.NewRectangle(color.Transparent)
	rect.SetMinSize(geo.NewSize(50, 300))
	scroll := widget.NewScroll(rect)
	scroll.Resize(geo.NewSize(100, 100))

	// With ReserveBarSpace=true (default), content should be narrower than scroll container.
	contentWidth := scroll.Content.Size().Width
	assert.Less(t, contentWidth, float32(100), "content should be narrower than container with ReserveBarSpace")

	// With ReserveBarSpace=false, content gets full width.
	scroll2 := widget.NewScroll(rect)
	scroll2.ReserveBarSpace = false
	scroll2.Resize(geo.NewSize(100, 100))
	contentWidth2 := scroll2.Content.Size().Width
	assert.Equal(t, float32(100), contentWidth2, "content should get full width with ReserveBarSpace=false")
}
