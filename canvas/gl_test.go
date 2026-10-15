package canvas_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
)

// These tests cover the context-free CanvasObject surface of GLView (creation,
// move/resize/hide/refresh). The GL-thread Render callback and actual drawing
// require a live GL context and are exercised by the -tags gl painter tests,
// not here.

func TestNewGLView(t *testing.T) {
	called := false
	render := func(ctx canvas.GLContext, w, h int, scale float32) { called = true }

	v := canvas.NewGLView(render)
	assert.NotNil(t, v)
	assert.NotNil(t, v.Render, "render callback should be stored")
	assert.Implements(t, (*core.CanvasObject)(nil), v)
	assert.False(t, called, "render callback must not be invoked at construction")
}

func TestGLView_MoveResize(t *testing.T) {
	v := canvas.NewGLView(nil)

	v.Resize(geo.NewSize(80, 40))
	assert.Equal(t, geo.NewSize(80, 40), v.Size())
	v.Resize(geo.NewSize(80, 40)) // no-op guard path
	assert.Equal(t, geo.NewSize(80, 40), v.Size())

	v.Move(geo.NewPos(10, 20))
	assert.Equal(t, geo.NewPos(10, 20), v.Position())
	v.Move(geo.NewPos(10, 20)) // no-op guard path
	assert.Equal(t, geo.NewPos(10, 20), v.Position())
}

func TestGLView_HideRefresh(t *testing.T) {
	v := canvas.NewGLView(nil)
	assert.True(t, v.Visible())

	v.Hide()
	assert.False(t, v.Visible())

	v.Show()
	assert.True(t, v.Visible())

	assert.NotPanics(t, func() { v.Refresh() })
}
