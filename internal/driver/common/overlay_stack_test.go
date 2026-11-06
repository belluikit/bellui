package common

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
)

// RemoveOnly keeps each remaining overlay's render cache with it, and forgets
// the removed one's nodes so where it was painted is repainted (#1038, #826).
func TestOverlayStack_RemoveOnlyKeepsRenderCachesInStep(t *testing.T) {
	var forgotten []core.CanvasObject
	s := &overlayStack{forget: func(n *RenderCacheNode) { forgotten = append(forgotten, n.obj) }}
	a, b, c := canvas.NewRectangle(nil), canvas.NewRectangle(nil), canvas.NewRectangle(nil)
	s.Add(a)
	s.Add(b)
	s.Add(c)

	s.RemoveOnly(b)
	assert.Equal(t, []core.CanvasObject{a, c}, s.List())
	if assert.Len(t, s.renderCaches, 2) {
		assert.Same(t, a, s.renderCaches[0].root.obj)
		assert.Same(t, c, s.renderCaches[1].root.obj)
	}
	assert.Equal(t, []core.CanvasObject{b}, forgotten)

	// Remove still drops the overlay and everything above it.
	s.Remove(a)
	assert.Empty(t, s.List())
	assert.Empty(t, s.renderCaches)
}
