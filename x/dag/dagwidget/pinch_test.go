package dagwidget

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
)

// A pinch zooms by its scale and keeps the graph point under the fingers
// fixed on screen (#379).
func TestDAGCanvas_PinchZoomsAroundFocus(t *testing.T) {
	c := newTestCanvas(t)
	c.Zoom = 1
	c.Offset = geo.NewPos(0, 0)
	focus := geo.NewPos(200, 100)
	before := c.screenToGraphWithZoom(focus, c.Zoom)

	c.Pinched(&event.PinchEvent{PointEvent: event.PointEvent{Position: focus}, Scale: 2})
	assert.InDelta(t, 2.0, c.Zoom, 1e-6)
	after := c.screenToGraphWithZoom(focus, c.Zoom)
	assert.InDelta(t, before.X, after.X, 1e-4, "the point under the fingers stays put")
	assert.InDelta(t, before.Y, after.Y, 1e-4)

	c.Pinched(&event.PinchEvent{PointEvent: event.PointEvent{Position: focus}, Scale: 10})
	assert.InDelta(t, 3.0, c.Zoom, 1e-6, "zoom is clamped like the wheel's")
}

// Fingers that move while pinching carry the graph with them: the graph point
// between them at one step is between them at the next. PinchEnded forgets the
// focus, so the next pinch does not pan from where the last one stopped.
func TestDAGCanvas_PinchPansWithFingers(t *testing.T) {
	c := newTestCanvas(t)
	c.Zoom = 1
	c.Offset = geo.NewPos(0, 0)
	start := geo.NewPos(200, 100)
	anchor := c.screenToGraphWithZoom(start, c.Zoom)

	c.Pinched(&event.PinchEvent{PointEvent: event.PointEvent{Position: start}, Scale: 1})
	moved := geo.NewPos(260, 140)
	c.Pinched(&event.PinchEvent{PointEvent: event.PointEvent{Position: moved}, Scale: 1.5})
	got := c.screenToGraphWithZoom(moved, c.Zoom)
	assert.InDelta(t, anchor.X, got.X, 1e-4, "the graph follows the fingers")
	assert.InDelta(t, anchor.Y, got.Y, 1e-4)

	c.PinchEnded()
	offset := c.Offset
	c.Pinched(&event.PinchEvent{PointEvent: event.PointEvent{Position: start}, Scale: 1})
	assert.Equal(t, offset, c.Offset, "a new pinch starts where its fingers are")
}
