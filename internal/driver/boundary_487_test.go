package driver_test

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/internal/driver"
	"github.com/belluikit/bellui/widget"
)

type boundaryWidget struct{ widget.BaseWidget }

func (w *boundaryWidget) CreateRenderer() core.WidgetRenderer {
	return &boundaryRenderer{widget.NewSimpleRenderer(canvas.NewRectangle(color.Black))}
}

type boundaryRenderer struct{ core.WidgetRenderer }

func (*boundaryRenderer) IsRepaintBoundary() {}

// TestRepaintBoundaryStatus pins what a walk may keep (#487): the answer for a
// widget is settled only once its renderer exists, since until then it may
// still turn out to be a boundary.
func TestRepaintBoundaryStatus(t *testing.T) {
	boundary, settled := driver.RepaintBoundaryStatus(canvas.NewRectangle(color.Black))
	assert.False(t, boundary)
	assert.True(t, settled, "a primitive's answer is not settled")

	w := &boundaryWidget{}
	w.ExtendBaseWidget(w)
	boundary, settled = driver.RepaintBoundaryStatus(w)
	assert.False(t, boundary)
	assert.False(t, settled, "settled before the widget has a renderer")

	cache.Renderer(w)
	boundary, settled = driver.RepaintBoundaryStatus(w)
	assert.True(t, boundary)
	assert.True(t, settled)
	assert.True(t, driver.IsRepaintBoundary(w))
}
