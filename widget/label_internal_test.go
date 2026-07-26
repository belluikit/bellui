//go:build !mobile

// Hover is desktop pointer semantics; the mobile driver delivers
// long-press instead, so these tests only apply to desktop builds.

package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/test"
)

// The truncation tooltip is driven by labelHoverOverlay, which is only part
// of the render tree when ShowTooltipWhenTruncated is set. Hovering a
// truncated label must start the tooltip timer; a plain label must not react
// to hover at all (it is deliberately not event.Hoverable so it cannot
// swallow hover from enclosing rows).
func TestLabel_TruncationTooltip_HoverStartsTimer(t *testing.T) {
	test.NewTempApp(t)

	l := NewLabel("some quite long text that will certainly not fit")
	l.Truncation = bellui.TextTruncateEllipsis
	l.ShowTooltipWhenTruncated = true
	w := test.NewTempWindow(t, l)
	w.Resize(geo.NewSize(60, 40))

	test.MoveMouse(w.Canvas(), geo.NewPos(30, 20))

	l.tooltipMu.Lock()
	timerSet := l.tooltipTimer != nil
	l.tooltipMu.Unlock()
	assert.True(t, timerSet, "hovering a truncated label should start the tooltip timer")

	test.MoveMouse(w.Canvas(), geo.NewPos(-10, -10))

	l.tooltipMu.Lock()
	timerSet = l.tooltipTimer != nil
	l.tooltipMu.Unlock()
	assert.False(t, timerSet, "hover out should cancel the tooltip timer")
}

// Labels are recycled in List/Tree/Table rows on scroll. If a row is destroyed
// while the tooltip timer is still pending (no hover-out), Destroy must cancel
// it so the deferred bellui.Do(showTooltip) cannot run against the removed widget.
func TestLabel_TruncationTooltip_DestroyCancelsTimer(t *testing.T) {
	test.NewTempApp(t)

	l := NewLabel("some quite long text that will certainly not fit")
	l.Truncation = bellui.TextTruncateEllipsis
	l.ShowTooltipWhenTruncated = true
	w := test.NewTempWindow(t, l)
	w.Resize(geo.NewSize(60, 40))

	test.MoveMouse(w.Canvas(), geo.NewPos(30, 20))

	l.tooltipMu.Lock()
	timerSet := l.tooltipTimer != nil
	l.tooltipMu.Unlock()
	assert.True(t, timerSet, "hovering a truncated label should start the tooltip timer")

	// Recycle the row out from under the pending timer.
	cache.Renderer(l).Destroy()

	l.tooltipMu.Lock()
	timerSet = l.tooltipTimer != nil
	l.tooltipMu.Unlock()
	assert.False(t, timerSet, "Destroy must cancel the pending tooltip timer")
}
