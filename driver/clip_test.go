package driver_test

import (
	"testing"

	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/driver"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"

	"github.com/stretchr/testify/assert"
)

func TestClipBoundsForObject_InsideScroll(t *testing.T) {
	test.NewTempApp(t)

	inner := widget.NewLabel("clipped content that is wider than the viewport")
	scroll := container.NewScroll(inner)

	w := test.NewWindow(scroll)
	defer w.Close()
	w.Resize(geo.NewSize(120, 80))

	c := w.Canvas()
	bounds, ok := driver.ClipBoundsForObject(c, inner)
	if !ok {
		t.Fatal("a widget inside a scroll should report a clipping ancestor")
	}
	// The clip is the scroll's bounds, which fit within the window.
	assert.LessOrEqual(t, bounds.Size.Width, float32(120))
	assert.LessOrEqual(t, bounds.Size.Height, float32(80))
	assert.Greater(t, bounds.Size.Width, float32(0))
	assert.Greater(t, bounds.Size.Height, float32(0))
}

func TestClipBoundsForObject_NoClip(t *testing.T) {
	test.NewTempApp(t)

	content := widget.NewLabel("unclipped")
	w := test.NewWindow(content)
	defer w.Close()
	w.Resize(geo.NewSize(200, 100))

	if _, ok := driver.ClipBoundsForObject(w.Canvas(), content); ok {
		t.Error("a widget drawn against the canvas should report no clipping ancestor")
	}
}

func TestClipBoundsForObject_NilSafe(t *testing.T) {
	if _, ok := driver.ClipBoundsForObject(nil, nil); ok {
		t.Error("nil canvas/object should report no clip")
	}
}
