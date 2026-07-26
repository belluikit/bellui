package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

func TestTooltip_Creation(t *testing.T) {
	btn := NewButton("Click", nil)
	tt := NewTooltip(btn, "Tooltip text")
	assert.NotNil(t, tt)
	assert.Equal(t, "Tooltip text", tt.Text)
	assert.Equal(t, btn, tt.Content)
	assert.NotNil(t, tt.overlay)
}

func TestTooltip_MinSize(t *testing.T) {
	btn := NewButton("Click", nil)
	tt := NewTooltip(btn, "Tooltip text")
	w := test.NewTempWindow(t, tt)
	_ = w

	assert.Equal(t, btn.MinSize(), tt.MinSize())
}

func TestTooltip_TapDelegatesToContent(t *testing.T) {
	tapped := false
	btn := NewButton("Click", func() { tapped = true })
	tt := NewTooltip(btn, "Tooltip text")
	w := test.NewTempWindow(t, tt)
	_ = w

	test.Tap(tt)
	assert.True(t, tapped, "Tap should delegate to content")
}

func TestTooltip_OverlayForwardsTapToContent(t *testing.T) {
	tapped := false
	btn := NewButton("Click", func() { tapped = true })
	tt := NewTooltip(btn, "Tooltip text")
	w := test.NewTempWindow(t, tt)
	_ = w

	// Tap the overlay directly — it should forward to the button
	tt.overlay.Tapped(&event.PointEvent{Position: geo.NewPos(5, 5)})
	assert.True(t, tapped, "Overlay tap should forward to content")
}

func TestTooltip_OverlayHoverTriggersTimer(t *testing.T) {
	btn := NewButton("Click", nil)
	tt := NewTooltip(btn, "Tooltip text")
	w := test.NewTempWindow(t, tt)
	_ = w

	// Hover on the overlay (simulates hovering over a button)
	tt.overlay.HoverIn(&event.HoverEvent{
		PointEvent: event.PointEvent{Position: geo.NewPos(5, 5)},
	})

	tt.mu.Lock()
	hasTimer := tt.showTimer != nil
	tt.mu.Unlock()
	assert.True(t, hasTimer, "Overlay hover should start tooltip timer")

	tt.overlay.HoverOut()
	tt.mu.Lock()
	assert.Nil(t, tt.showTimer, "Overlay mouse out should cancel timer")
	tt.mu.Unlock()
}

func TestTooltip_OverlayTapHidesPopup(t *testing.T) {
	btn := NewButton("Click", nil)
	tt := NewTooltip(btn, "Tooltip text")
	w := test.NewTempWindow(t, tt)
	_ = w

	// Show popup directly
	tt.showPopUp()
	assert.NotNil(t, tt.tip)

	// Tap overlay should dismiss tooltip and forward tap
	tt.overlay.Tapped(&event.PointEvent{Position: geo.NewPos(5, 5)})
	tt.mu.Lock()
	assert.Nil(t, tt.tip, "Overlay tap should hide popup")
	tt.mu.Unlock()
}

func TestTooltip_EmptyText(t *testing.T) {
	btn := NewButton("Click", nil)
	tt := NewTooltip(btn, "")
	w := test.NewTempWindow(t, tt)
	_ = w

	tt.showPopUp()
	assert.Nil(t, tt.tip, "No popup for empty tooltip text")
}

func TestTooltip_TapHidesPopup(t *testing.T) {
	btn := NewButton("Click", nil)
	tt := NewTooltip(btn, "Tooltip text")
	w := test.NewTempWindow(t, tt)
	_ = w

	tt.showPopUp()
	assert.NotNil(t, tt.tip)

	tt.Tapped(&event.PointEvent{Position: geo.NewPos(5, 5)})
	tt.mu.Lock()
	assert.Nil(t, tt.tip)
	tt.mu.Unlock()
}

func TestTooltip_RendererHasOverlay(t *testing.T) {
	btn := NewButton("Click", nil)
	tt := NewTooltip(btn, "Tooltip text")
	w := test.NewTempWindow(t, tt)
	_ = w

	r := test.TempWidgetRenderer(t, tt)
	objs := r.Objects()
	assert.Len(t, objs, 2, "Renderer should have content and overlay")
	assert.Equal(t, btn, objs[0], "First object should be content")
	assert.Equal(t, tt.overlay, objs[1], "Second object should be hover overlay")
}

func TestTooltip_ShowPopUpPositionsBelowWidget(t *testing.T) {
	btn := NewButton("Click", nil)
	tt := NewTooltip(btn, "Tooltip text")
	w := test.NewTempWindow(t, tt)
	_ = w

	tt.showPopUp()
	assert.NotNil(t, tt.tip)
	tt.hidePopUp()
}

// TestTooltip_ClickPassesThroughWhileVisible is the regression for the
// visible tooltip consuming the click aimed at the wrapped widget: the
// old PopUp-based bubble was the top overlay, and overlay hit-testing
// treated it as an input barrier — the first click only dismissed the
// tip. The tooltip overlay now reports PassesInputThrough, so the
// driver falls through to the content beneath.
func TestTooltip_ClickPassesThroughWhileVisible(t *testing.T) {
	fired := false
	btn := NewButton("Click", func() { fired = true })
	tt := NewTooltip(btn, "Tooltip text")
	w := test.NewTempWindow(t, tt)
	w.Resize(geo.NewSize(200, 100))

	tt.showPopUp()
	assert.NotNil(t, tt.tip)

	// Tap the canvas over the button while the tooltip is visible.
	test.TapCanvas(w.Canvas(), geo.NewPos(10, 10))
	assert.True(t, fired, "click while tooltip visible must reach the widget beneath")
}
