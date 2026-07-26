package bellui

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
)

// newContainerWithoutLayout and newContainerWithLayout replicate the removed
// public bellui.NewContainer* constructors for these package-internal tests (the
// public API is now container.New / container.NewWithoutLayout, which this
// package cannot import without a cycle).
func newContainerWithoutLayout(objects ...core.CanvasObject) *Container {
	ret := &Container{Objects: objects}
	ret.size = ret.MinSize()
	return ret
}

func newContainerWithLayout(layout core.Layout, objects ...core.CanvasObject) *Container {
	ret := &Container{Objects: objects, Layout: layout}
	ret.size = layout.MinSize(objects)
	ret.layout()
	return ret
}

func TestContainer_Add(t *testing.T) {
	box := new(dummyObject)
	container := newContainerWithLayout(new(customLayout))
	assert.Empty(t, container.Objects)
	assert.Equal(t, float32(10), container.MinSize().Width)
	assert.Equal(t, float32(0), container.MinSize().Height)

	container.Add(box)
	assert.Len(t, container.Objects, 1)
	assert.Equal(t, float32(10), container.MinSize().Width)
	assert.Equal(t, float32(10), container.MinSize().Height)

	oldLength := len(container.Objects)
	container.Add(nil)
	assert.Len(t, container.Objects, oldLength)

	box2 := new(dummyObject)
	container.Add(box2)
	assert.Len(t, container.Objects, 2)
	assert.Equal(t, float32(10), container.MinSize().Width)
	assert.Equal(t, float32(20), container.MinSize().Height)
	assert.Equal(t, float32(0), box2.Position().X)
	assert.Equal(t, float32(10), box2.Position().Y)
}

func TestContainer_CustomLayout(t *testing.T) {
	box := new(dummyObject)
	layout := new(customLayout)
	container := newContainerWithLayout(layout, box)

	size := layout.MinSize(container.Objects)
	assert.Equal(t, size, container.MinSize())
	assert.Equal(t, size, container.Size())
	assert.Equal(t, size, box.Size())
}

func TestContainer_Hide(t *testing.T) {
	box := new(dummyObject)
	container := newContainerWithoutLayout(box)

	assert.True(t, container.Visible())
	assert.True(t, box.Visible())
	container.Hide()
	assert.False(t, container.Visible())
	assert.True(t, box.Visible())
}

func TestContainer_MinSize(t *testing.T) {
	box := new(dummyObject)
	minSize := box.MinSize()

	container := newContainerWithoutLayout(box)
	assert.Equal(t, minSize, container.MinSize())
}

func TestContainer_Move(t *testing.T) {
	box := new(dummyObject)
	container := newContainerWithoutLayout(box)

	size := geo.NewSize(100, 100)
	pos := geo.NewPos(0, 0)
	container.Resize(size)
	assert.Equal(t, pos, box.Position())

	pos = geo.NewPos(10, 10)
	container.Move(pos)
	assert.Equal(t, pos, container.Position())
	assert.Equal(t, geo.NewPos(0, 0), box.Position())

	box.Move(pos)
	assert.Equal(t, pos, box.Position())
}

func TestContainer_NilLayout(t *testing.T) {
	box := new(dummyObject)
	boxSize := box.size
	container := newContainerWithoutLayout(box)

	size := geo.NewSize(100, 100)
	container.Resize(size)
	assert.Equal(t, size, container.Size())
	assert.Equal(t, boxSize, box.Size())
}

func TestContainer_Remove(t *testing.T) {
	box1 := new(dummyObject)
	box2 := new(dummyObject)
	container := newContainerWithLayout(new(customLayout), box1, box2)
	assert.Len(t, container.Objects, 2)
	assert.Equal(t, float32(10), container.MinSize().Width)
	assert.Equal(t, float32(20), container.MinSize().Height)

	container.Remove(box1)
	assert.Len(t, container.Objects, 1)
	assert.Equal(t, float32(10), container.MinSize().Width)
	assert.Equal(t, float32(10), container.MinSize().Height)
	assert.Equal(t, float32(0), box2.Position().X)
	assert.Equal(t, float32(0), box2.Position().Y)
}

func TestContainer_RemoveAll(t *testing.T) {
	box1 := new(dummyObject)
	box2 := new(dummyObject)
	container := newContainerWithLayout(new(customLayout), box1, box2)
	assert.Len(t, container.Objects, 2)

	container.RemoveAll()
	assert.Empty(t, container.Objects)
}

func TestContainer_Show(t *testing.T) {
	box := new(dummyObject)
	container := newContainerWithoutLayout(box)

	container.Hide()
	assert.True(t, box.Visible())
	assert.False(t, container.Visible())

	container.Show()
	assert.True(t, box.Visible())
	assert.True(t, container.Visible())
}

type customLayout struct{}

func (c *customLayout) Layout(objs []core.CanvasObject, size geo.Size) {
	y := float32(0)
	for _, child := range objs {
		child.Resize(size)
		child.Move(geo.NewPos(0, y))
		y += 10
	}
}

func (c *customLayout) MinSize(objs []core.CanvasObject) geo.Size {
	return geo.NewSize(10, float32(10*len(objs)))
}

type dummyObject struct {
	size   geo.Size
	pos    geo.Position
	hidden bool
}

func (d *dummyObject) Hide() {
	d.hidden = true
}

func (d *dummyObject) MinSize() geo.Size {
	return geo.NewSize(5, 5)
}

func (d *dummyObject) Move(pos geo.Position) {
	d.pos = pos
}

func (d *dummyObject) Position() geo.Position {
	return d.pos
}

func (d *dummyObject) Refresh() {
}

func (d *dummyObject) Resize(size geo.Size) {
	d.size = size
}

func (d *dummyObject) Show() {
	d.hidden = false
}

func (d *dummyObject) Size() geo.Size {
	return d.size
}

func (d *dummyObject) Visible() bool {
	return !d.hidden
}
