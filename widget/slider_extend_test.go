package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
)

type extendedSlider struct {
	Slider
}

func newExtendedSlider() *extendedSlider {
	slider := &extendedSlider{}
	slider.ExtendBaseWidget(slider)
	slider.Min = 0
	slider.Max = 10
	return slider
}

func TestSlider_Extended_Value(t *testing.T) {
	slider := newExtendedSlider()
	slider.Resize(slider.MinSize().Add(geo.NewSize(20, 0)))
	objs := cache.Renderer(slider).Objects()
	assert.Len(t, objs, 4)
	thumb := objs[2]
	thumbPos := thumb.Position()

	slider.Value = 2
	slider.Refresh()
	assert.Greater(t, thumb.Position().X, thumbPos.X)
	assert.Equal(t, thumbPos.Y, thumb.Position().Y)
}

func TestSlider_Extended_Drag(t *testing.T) {
	slider := newExtendedSlider()
	objs := cache.Renderer(slider).Objects()
	assert.Len(t, objs, 4)
	thumb := objs[2]
	thumbPos := thumb.Position()

	drag := &event.DragEvent{Dragged: geo.NewDelta(10, 2)}
	slider.Dragged(drag)
	assert.Greater(t, thumbPos.X, thumb.Position().X)
	assert.Equal(t, thumbPos.Y, thumb.Position().Y)
}
