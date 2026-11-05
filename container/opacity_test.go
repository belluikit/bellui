package container

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

func TestOpacity(t *testing.T) {
	test.NewTempApp(t)
	r := canvas.NewRectangle(color.White)
	r.SetMinSize(geo.NewSize(30, 20))
	o := NewOpacity(r, 0.4)

	assert.Equal(t, float32(0.4), o.GroupOpacity())
	assert.Equal(t, geo.NewSize(30, 20), o.MinSize(), "the content's MinSize")
	o.Resize(geo.NewSize(50, 40))
	assert.Equal(t, geo.NewSize(50, 40), r.Size(), "the content fills it")

	o.SetOpacity(3)
	assert.Equal(t, float32(1), o.GroupOpacity(), "clamped to 1")
	o.SetOpacity(-1)
	assert.Equal(t, float32(0), o.GroupOpacity(), "clamped to 0")

	r2 := canvas.NewRectangle(color.Black)
	o.Content = r2
	o.Refresh()
	assert.Equal(t, geo.NewSize(50, 40), r2.Size(), "a replaced content is laid out")
}
