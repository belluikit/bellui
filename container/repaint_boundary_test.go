package container_test

import (
	"testing"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/internal/driver"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
	"github.com/stretchr/testify/assert"
)

func TestRepaintBoundary_MinSize(t *testing.T) {
	label := widget.NewLabel("Hello")
	rb := container.NewRepaintBoundary(label)

	// MinSize should delegate to the content's MinSize.
	expected := label.MinSize()
	assert.Equal(t, expected, rb.MinSize())
}

func TestRepaintBoundary_Layout(t *testing.T) {
	label := widget.NewLabel("Test")
	rb := container.NewRepaintBoundary(label)
	rb.Resize(geo.NewSize(200, 100))

	// Content should be resized to fill the boundary.
	assert.Equal(t, geo.NewSize(200, 100), label.Size())
}

func TestRepaintBoundary_Refresh(t *testing.T) {
	rect := canvas.NewRectangle(nil)
	rb := container.NewRepaintBoundary(rect)
	_ = test.NewTempApp(t)

	w := test.NewWindow(rb)
	defer w.Close()

	// Changing content and refreshing should not panic.
	newRect := canvas.NewRectangle(nil)
	rb.Content = newRect
	rb.Refresh()

	r := cache.Renderer(rb)
	objects := r.Objects()
	assert.Len(t, objects, 1)
	assert.Equal(t, newRect, objects[0])
}

func TestRepaintBoundary_IsRepaintBoundary(t *testing.T) {
	rect := canvas.NewRectangle(nil)
	rb := container.NewRepaintBoundary(rect)

	// Force renderer creation.
	_ = cache.Renderer(rb)

	assert.True(t, driver.IsRepaintBoundary(rb))
}

func TestRepaintBoundary_NonBoundaryWidget(t *testing.T) {
	label := widget.NewLabel("not a boundary")
	_ = cache.Renderer(label)

	assert.False(t, driver.IsRepaintBoundary(label))
}

func TestRepaintBoundary_NonWidget(t *testing.T) {
	rect := canvas.NewRectangle(nil)
	assert.False(t, driver.IsRepaintBoundary(rect))
}

func TestRepaintBoundary_NestedInClip(t *testing.T) {
	label := widget.NewLabel("Nested")
	rb := container.NewRepaintBoundary(label)
	clip := container.NewClip(rb)

	_ = test.NewTempApp(t)
	w := test.NewWindow(clip)
	defer w.Close()

	w.Resize(geo.NewSize(200, 100))

	// Should not panic. Content should be visible.
	assert.True(t, label.Visible())
}
