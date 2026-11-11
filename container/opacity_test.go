package container

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"

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

// countingResource counts how often its content is read.
type countingResource struct {
	bellui.Resource
	reads int
}

func (r *countingResource) Content() []byte {
	r.reads++
	return r.Resource.Content()
}

// TestOpacity_SetOpacityReadsNoImage pins #1138: changing the opacity redraws
// the content without refreshing it, so an image under it reads nothing. An
// animated opacity did, on every frame.
func TestOpacity_SetOpacityReadsNoImage(t *testing.T) {
	test.NewTempApp(t)
	res := &countingResource{Resource: bellui.NewStaticResource("dot.svg", []byte(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><circle cx="5" cy="5" r="4" fill="red"/></svg>`,
	))}
	img := canvas.NewImageFromResource(res)
	o := NewOpacity(img, 1)
	w := test.NewTempWindow(t, o)
	w.Resize(geo.NewSize(80, 80))
	reads := res.reads
	require.NotZero(t, reads, "the image was read to draw it")

	for i := range 20 {
		o.SetOpacity(float32(i) / 20)
	}
	assert.Equal(t, reads, res.reads, "no reads while the opacity changes")
	assert.Equal(t, float32(0.95), o.GroupOpacity())

	o.Refresh()
	assert.Greater(t, res.reads, reads, "Refresh still refreshes the content")
}
