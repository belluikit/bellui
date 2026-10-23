package widget_test

import (
	"image/color"
	"testing"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/widget"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every programmatic scroll reports through OnScrolled and the scroll
// listeners, as user scrolls do; #339 found only ScrollToBottom did.
func TestScroll_ProgrammaticScrollsNotify(t *testing.T) {
	rect := canvas.NewRectangle(color.Transparent)
	rect.SetMinSize(geo.NewSize(500, 500))
	scroll := widget.NewScroll(rect)
	scroll.Resize(geo.NewSize(100, 100))

	var got []geo.Position
	scroll.OnScrolled = func(p geo.Position) { got = append(got, p) }
	var listened []geo.Position
	scroll.AddScrollListener(func(p geo.Position) { listened = append(listened, p) })

	scroll.ScrollToOffset(geo.NewPos(0, 50))
	scroll.ScrollToBottom()
	scroll.ScrollToTop()

	want := []geo.Position{{X: 0, Y: 50}, {X: 0, Y: 400}, {X: 0, Y: 0}}
	assert.Equal(t, want, got, "OnScrolled")
	assert.Equal(t, want, listened, "AddScrollListener")

	// No movement, no report.
	scroll.ScrollToTop()
	scroll.ScrollToOffset(geo.Position{})
	require.Len(t, got, 3, "a scroll that does not move must not report")

	// An offset past the end is clamped once and reported once, at the clamped value.
	scroll.ScrollToOffset(geo.NewPos(0, 9999))
	assert.Equal(t, []geo.Position{{X: 0, Y: 400}}, got[3:])
}

// ScrollToBottom on content that is wide but not tall must not move the
// horizontal offset; it used to run the wheel path, whose axis swap turned it
// into a horizontal scroll.
func TestScroll_ScrollToBottomKeepsHorizontalOffset(t *testing.T) {
	rect := canvas.NewRectangle(color.Transparent)
	rect.SetMinSize(geo.NewSize(500, 50))
	scroll := widget.NewScroll(rect)
	scroll.Resize(geo.NewSize(100, 100))

	scroll.ScrollToOffset(geo.NewPos(200, 0))
	scroll.ScrollToBottom()
	assert.Equal(t, geo.NewPos(200, 0), scroll.Offset)
}
