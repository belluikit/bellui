package layout_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/layout"
)

// The aspect stack puts every object in the largest box of its ratio that
// fits, centred; its MinSize is the largest child's grown to the ratio (#993).
func TestAspectStackLayout(t *testing.T) {
	a, b := canvas.NewRectangle(nil), canvas.NewRectangle(nil)
	objs := []core.CanvasObject{a, b}
	square := layout.NewAspectStackLayout(1)

	square.Layout(objs, geo.NewSize(300, 200)) // wide: the height decides
	for _, o := range objs {
		assert.Equal(t, geo.NewSize(200, 200), o.Size())
		assert.Equal(t, geo.NewPos(50, 0), o.Position())
	}
	square.Layout(objs, geo.NewSize(120, 400)) // tall: the width decides
	assert.Equal(t, geo.NewSize(120, 120), a.Size())
	assert.Equal(t, geo.NewPos(0, 140), a.Position())

	wide := layout.NewAspectStackLayout(2)
	wide.Layout(objs, geo.NewSize(300, 300))
	assert.Equal(t, geo.NewSize(300, 150), a.Size())
	assert.Equal(t, geo.NewPos(0, 75), a.Position())

	a.SetMinSize(geo.NewSize(40, 10))
	b.SetMinSize(geo.NewSize(10, 30))
	assert.Equal(t, geo.NewSize(40, 40), square.MinSize(objs), "the larger side wins")
	assert.Equal(t, geo.NewSize(60, 30), wide.MinSize(objs), "grown to 2:1")
	b.Hide()
	assert.Equal(t, geo.NewSize(40, 40), square.MinSize(objs), "hidden children don't count")
	assert.Equal(t, geo.NewSize(40, 20), wide.MinSize(objs))

	layout.NewAspectStackLayout(0).Layout(objs, geo.NewSize(300, 200))
	assert.Equal(t, geo.NewSize(200, 200), a.Size(), "a ratio of 0 is a square")
}
