package common

import (
	"image/color"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
)

// tiltBox is a widget with a transform, holding its objects where they are put.
type tiltBox struct {
	widget.BaseWidget
	m    geo.Matrix
	objs []core.CanvasObject
}

func newTiltBox(m geo.Matrix, objs ...core.CanvasObject) *tiltBox {
	b := &tiltBox{m: m, objs: objs}
	b.ExtendBaseWidget(b)
	return b
}

func (b *tiltBox) Transform() geo.Matrix     { return b.m }
func (b *tiltBox) SetTransform(m geo.Matrix) { b.m = m }

func (b *tiltBox) CreateRenderer() core.WidgetRenderer {
	return widget.NewSimpleRenderer(container.NewWithoutLayout(b.objs...))
}

// TestCanvas_WorldTransformPlacesNodes pins #997 for what the canvas tells the
// painters, the partial-repaint region and the spatial index: a node's paint
// origin, its paint bounds and where the index hits it all follow its world
// transform, translation included. The scenes are the software painter's
// (TestPainter_WorldTransformMovesObjects).
func TestCanvas_WorldTransformPlacesNodes(t *testing.T) {
	test.NewTempApp(t)
	rect := func(x, y, w, h float32) *canvas.Rectangle {
		r := canvas.NewRectangle(color.White)
		r.Move(geo.NewPos(x, y))
		r.Resize(geo.NewSize(w, h))
		return r
	}
	in := func(p geo.Position, pos geo.Position, size geo.Size) bool {
		return p.X >= pos.X && p.Y >= pos.Y && p.X < pos.X+size.Width && p.Y < pos.Y+size.Height
	}

	for name, tc := range map[string]struct {
		scene     func() (root, target core.CanvasObject)
		origin    geo.Position // where the target's (0, 0) is drawn
		drawn, no geo.Position
	}{
		"translation only": {
			scene: func() (core.CanvasObject, core.CanvasObject) {
				r := rect(20, 10, 20, 20)
				r.SetTransform(geo.NewTranslation(100, 0))
				return container.NewWithoutLayout(r), r
			},
			origin: geo.NewPos(120, 10), drawn: geo.NewPos(125, 15), no: geo.NewPos(25, 15),
		},
		"rotation about a pivot": {
			scene: func() (core.CanvasObject, core.CanvasObject) {
				r := rect(50, 50, 40, 20)
				r.SetTransform(geo.NewRotationAround(math.Pi, 20, 10))
				return container.NewWithoutLayout(r), r
			},
			origin: geo.NewPos(90, 70), drawn: geo.NewPos(85, 65), no: geo.NewPos(15, 35),
		},
		"transformed parent, offset child": {
			scene: func() (core.CanvasObject, core.CanvasObject) {
				child := rect(80, 10, 10, 10)
				box := newTiltBox(geo.NewRotationAround(math.Pi/2, 50, 50), child)
				box.Move(geo.NewPos(40, 40))
				box.Resize(geo.NewSize(100, 100))
				return container.NewWithoutLayout(box), child
			},
			origin: geo.NewPos(130, 120), drawn: geo.NewPos(124, 126), no: geo.NewPos(125, 55),
		},
	} {
		t.Run(name, func(t *testing.T) {
			root, target := tc.scene()
			root.Resize(geo.NewSize(200, 200))
			tree := &renderCacheTree{root: &RenderCacheNode{obj: root}}
			c := &Canvas{}
			c.Initialize(nil, func() {})
			c.walkTree(tree, nil, nil, nil, nil)
			node, ok := c.NodeFor(target)
			require.True(t, ok)

			o := node.PaintOrigin()
			assert.InDelta(t, tc.origin.X, o.X, 0.01, "paint origin")
			assert.InDelta(t, tc.origin.Y, o.Y, 0.01, "paint origin")

			pos, size := PaintBounds(node)
			assert.True(t, in(tc.drawn, pos, size), "paint bounds %v %v hold where it is drawn, %v", pos, size, tc.drawn)
			assert.False(t, in(tc.no, pos, size), "and not where it would be without its translation, %v", tc.no)

			idx := &SpatialIndex{}
			idx.reset(200, 200)
			c.walkCacheNodeDFS(tree.root, idx)
			idx.finish()
			isTarget := func(o core.CanvasObject) bool { return o == target }
			found, _, _ := idx.FindObjectAtPositionMatching(tc.drawn, isTarget)
			assert.Equal(t, target, found, "the index hits it where it is drawn")
			found, _, _ = idx.FindObjectAtPositionMatching(tc.no, isTarget)
			assert.Nil(t, found, "and not where it would be without its translation")
		})
	}
}

// TestCanvas_ClipUnderTransform pins #997 for the clips the canvas records: a
// clipping container moved by a transform clips its children where it is
// drawn, in a walk from the root and in one from a node below the transform,
// so the spatial index hits a child there and not past the clip.
func TestCanvas_ClipUnderTransform(t *testing.T) {
	test.NewTempApp(t)
	inner := canvas.NewRectangle(color.White)
	inner.Resize(geo.NewSize(80, 80)) // larger than the clip
	clip := &bellui.Container{ClipToBounds: true, Objects: []core.CanvasObject{inner}}
	clip.Move(geo.NewPos(10, 10))
	clip.Resize(geo.NewSize(40, 40))
	box := newTiltBox(geo.NewTranslation(100, 0), clip)
	box.Resize(geo.NewSize(60, 60))
	root := container.NewWithoutLayout(box)
	root.Resize(geo.NewSize(200, 200))
	tree := &renderCacheTree{root: &RenderCacheNode{obj: root}}
	c := &Canvas{}
	c.Initialize(nil, func() {})
	c.walkTree(tree, nil, nil, nil, nil)
	clipNode, ok := c.NodeFor(clip)
	require.True(t, ok)

	for name, walk := range map[string]func(){
		"from the root":      func() {},
		"from below the box": func() { c.walkTreeFrom(clipNode, nil, nil, nil, nil) },
	} {
		t.Run(name, func(t *testing.T) {
			walk()
			node, ok := c.NodeFor(inner)
			require.True(t, ok)
			assert.Equal(t, geo.NewPos(110, 10), node.clipPos)
			assert.Equal(t, geo.NewSize(40, 40), node.clipSize)

			idx := &SpatialIndex{}
			idx.reset(200, 200)
			c.walkCacheNodeDFS(tree.root, idx)
			idx.finish()
			isInner := func(o core.CanvasObject) bool { return o == inner }
			found, _, _ := idx.FindObjectAtPositionMatching(geo.NewPos(120, 20), isInner)
			assert.Equal(t, core.CanvasObject(inner), found, "hit inside the clip where it is drawn")
			found, _, _ = idx.FindObjectAtPositionMatching(geo.NewPos(160, 20), isInner)
			assert.Nil(t, found, "not past the clip's edge")
		})
	}
}

// TestCanvas_ClipUnderRotation pins #1003 for the spatial index: a clip
// turned an eighth clips its children to the turned square, as the painters
// draw it, not to the box around it. The scene is the real-loop test's
// (TestWorldTransformDraws, "clip under a rotation"): a diamond about
// (90, 90), whose box is (47.6, 47.6)-(132.4, 132.4).
func TestCanvas_ClipUnderRotation(t *testing.T) {
	test.NewTempApp(t)
	inner := canvas.NewRectangle(color.White)
	inner.Move(geo.NewPos(-50, -50))
	inner.Resize(geo.NewSize(160, 160)) // past the clip on every side
	clip := &bellui.Container{ClipToBounds: true, Objects: []core.CanvasObject{inner}}
	clip.Move(geo.NewPos(20, 20))
	clip.Resize(geo.NewSize(60, 60))
	box := newTiltBox(geo.NewRotationAround(math.Pi/4, 50, 50), clip)
	box.Move(geo.NewPos(40, 40))
	box.Resize(geo.NewSize(100, 100))
	root := container.NewWithoutLayout(box)
	root.Resize(geo.NewSize(200, 200))
	tree := &renderCacheTree{root: &RenderCacheNode{obj: root}}
	c := &Canvas{}
	c.Initialize(nil, func() {})
	c.walkTree(tree, nil, nil, nil, nil)

	idx := &SpatialIndex{}
	idx.reset(200, 200)
	c.walkCacheNodeDFS(tree.root, idx)
	idx.finish()
	isInner := func(o core.CanvasObject) bool { return o == inner }
	found, _, _ := idx.FindObjectAtPositionMatching(geo.NewPos(90, 90), isInner)
	assert.Equal(t, core.CanvasObject(inner), found, "hit inside the turned clip")
	found, _, _ = idx.FindObjectAtPositionMatching(geo.NewPos(60, 60), isInner)
	assert.Nil(t, found, "not in the corner of the box around it, where it is not drawn")
	assert.Empty(t, idx.FindAllObjectsAtPositionMatching(geo.NewPos(60, 60), isInner), "nor among all the objects there")
	found, _, _ = idx.FindObjectAtPositionMatching(geo.NewPos(60, 60), func(o core.CanvasObject) bool { return o == box })
	assert.Equal(t, core.CanvasObject(box), found, "what holds the clip is still hit there")
}

// A turned clip inside another is tested against both: (60, 60) lies inside
// the inner clip, which is larger, but outside the outer one (#1003).
func TestCanvas_NestedClipsUnderRotation(t *testing.T) {
	test.NewTempApp(t)
	leaf := canvas.NewRectangle(color.White)
	leaf.Resize(geo.NewSize(160, 160))
	innerClip := &bellui.Container{ClipToBounds: true, Objects: []core.CanvasObject{leaf}}
	innerClip.Move(geo.NewPos(-50, -50))
	innerClip.Resize(geo.NewSize(160, 160))
	clip := &bellui.Container{ClipToBounds: true, Objects: []core.CanvasObject{innerClip}}
	clip.Move(geo.NewPos(20, 20))
	clip.Resize(geo.NewSize(60, 60))
	box := newTiltBox(geo.NewRotationAround(math.Pi/4, 50, 50), clip)
	box.Move(geo.NewPos(40, 40))
	box.Resize(geo.NewSize(100, 100))
	root := container.NewWithoutLayout(box)
	root.Resize(geo.NewSize(200, 200))
	tree := &renderCacheTree{root: &RenderCacheNode{obj: root}}
	c := &Canvas{}
	c.Initialize(nil, func() {})
	c.walkTree(tree, nil, nil, nil, nil)

	idx := &SpatialIndex{}
	idx.reset(200, 200)
	c.walkCacheNodeDFS(tree.root, idx)
	idx.finish()
	assert.Len(t, idx.cuts, 2, "both clips are turned")
	isLeaf := func(o core.CanvasObject) bool { return o == leaf }
	found, _, _ := idx.FindObjectAtPositionMatching(geo.NewPos(90, 90), isLeaf)
	assert.Equal(t, core.CanvasObject(leaf), found, "hit inside both clips")
	found, _, _ = idx.FindObjectAtPositionMatching(geo.NewPos(60, 60), isLeaf)
	assert.Nil(t, found, "not inside the inner clip only")
}
