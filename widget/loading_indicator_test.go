package widget_test

import (
	"testing"

	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"

	"github.com/stretchr/testify/assert"
)

func TestLoadingIndicator_New(t *testing.T) {
	test.NewTempApp(t)
	l := widget.NewLoadingIndicator(widget.LoadingShapeRect)
	assert.NotNil(t, l)
	assert.Equal(t, widget.LoadingShapeRect, l.Shape)
}

func TestLoadingIndicator_Shapes(t *testing.T) {
	test.NewTempApp(t)
	shapes := []widget.LoadingShape{
		widget.LoadingShapeRect,
		widget.LoadingShapeCircle,
		widget.LoadingShapeText,
	}
	for _, shape := range shapes {
		l := widget.NewLoadingIndicator(shape)
		min := l.MinSize()
		assert.Greater(t, min.Width, float32(0), "shape %d width", shape)
		assert.Greater(t, min.Height, float32(0), "shape %d height", shape)
	}
}

func TestLoadingIndicator_CircleMinSize(t *testing.T) {
	test.NewTempApp(t)
	l := widget.NewLoadingIndicator(widget.LoadingShapeCircle)
	min := l.MinSize()
	assert.GreaterOrEqual(t, min.Width, float32(40))
	assert.GreaterOrEqual(t, min.Height, float32(40))
}

func TestLoadingIndicator_Renderer(t *testing.T) {
	test.NewTempApp(t)
	l := widget.NewLoadingIndicator(widget.LoadingShapeText)
	assert.NotPanics(t, func() { test.TempWidgetRenderer(t, l) })
}
