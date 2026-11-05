package driver_test

import (
	"image/color"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/driver"
	"github.com/belluikit/bellui/widget"
)

// Without a transform, mapping a point is AbsolutePositionForObject plus the
// point, for every object of a tree that nests containers and a widget's
// renderer (#1003).
func TestPositionMapping_MatchesLayoutWithoutTransforms(t *testing.T) {
	leaf := canvas.NewRectangle(color.Black)
	leaf.Move(geo.NewPos(3, 4))
	leaf.Resize(geo.NewSize(10, 10))
	label := widget.NewLabel("text")
	label.Move(geo.NewPos(7, 9))
	label.Resize(geo.NewSize(60, 30))
	inner := &bellui.Container{Objects: []core.CanvasObject{leaf, label}}
	inner.Move(geo.NewPos(11, 13))
	inner.Resize(geo.NewSize(100, 100))
	root := &bellui.Container{Objects: []core.CanvasObject{inner}}
	root.Move(geo.NewPos(5, 6))
	root.Resize(geo.NewSize(200, 200))
	trees := []core.CanvasObject{root}

	p := geo.NewPos(2, 3)
	n := 0
	driver.WalkVisibleObjectTree(root, func(o core.CanvasObject, _, _ geo.Position, _ geo.Size) bool {
		n++
		want := driver.AbsolutePositionForObject(o, trees).Add(p)
		assert.Equal(t, want, driver.AbsolutePositionForPoint(o, p, trees), "%T", o)
		back, ok := driver.PositionForAbsolutePoint(o, want, trees)
		assert.True(t, ok)
		assert.Equal(t, p, back, "%T", o)
		return false
	}, nil)
	assert.Greater(t, n, 4, "the walk reached the label's renderer objects")
}

// Under a transform, the mapping agrees with the hit-test: the hit point maps
// to the local point it reports, and back (#1003). The scenes are
// TestFindObjectAtPosition_WorldTransform's.
func TestPositionMapping_FollowsWorldTransform(t *testing.T) {
	rect := func(x, y, w, h float32) *canvas.Rectangle {
		r := canvas.NewRectangle(color.Black)
		r.Move(geo.NewPos(x, y))
		r.Resize(geo.NewSize(w, h))
		return r
	}
	for name, tc := range map[string]struct {
		scene        func() (root, target core.CanvasObject)
		drawn, local geo.Position
	}{
		"translation only": {
			scene: func() (core.CanvasObject, core.CanvasObject) {
				r := rect(20, 10, 20, 20)
				r.SetTransform(geo.NewTranslation(100, 0))
				return container.NewWithoutLayout(r), r
			},
			drawn: geo.NewPos(125, 15), local: geo.NewPos(5, 5),
		},
		"rotation about a pivot": {
			scene: func() (core.CanvasObject, core.CanvasObject) {
				r := rect(50, 50, 40, 20)
				r.SetTransform(geo.NewRotationAround(math.Pi, 20, 10))
				return container.NewWithoutLayout(r), r
			},
			drawn: geo.NewPos(85, 65), local: geo.NewPos(5, 5),
		},
		"transformed parent, offset child": {
			scene: func() (core.CanvasObject, core.CanvasObject) {
				child := rect(80, 10, 10, 10)
				box := newTiltBox(geo.NewRotationAround(math.Pi/2, 50, 50), child)
				box.Move(geo.NewPos(40, 40))
				box.Resize(geo.NewSize(100, 100))
				return container.NewWithoutLayout(box), child
			},
			drawn: geo.NewPos(124, 126), local: geo.NewPos(6, 6),
		},
	} {
		t.Run(name, func(t *testing.T) {
			root, target := tc.scene()
			root.Resize(geo.NewSize(200, 200))
			trees := []core.CanvasObject{root}

			_, hit, _ := driver.FindObjectAtPositionMatching(tc.drawn, func(o core.CanvasObject) bool { return o == target }, nil, root)
			assert.InDelta(t, tc.local.X, hit.X, 0.01, "the hit-test agrees with the scene")

			abs := driver.AbsolutePositionForPoint(target, tc.local, trees)
			assert.InDelta(t, tc.drawn.X, abs.X, 0.01)
			assert.InDelta(t, tc.drawn.Y, abs.Y, 0.01)
			back, ok := driver.PositionForAbsolutePoint(target, tc.drawn, trees)
			assert.True(t, ok)
			assert.InDelta(t, tc.local.X, back.X, 0.01)
			assert.InDelta(t, tc.local.Y, back.Y, 0.01)
		})
	}
}

// AbsolutePositionForObject is the position from layout and ignores
// transforms: the owner's decision on #1003, so apps calling the v1 method do
// not silently get different results. This fails the day it starts following
// the world transform.
func TestAbsolutePositionForObject_IgnoresTransforms(t *testing.T) {
	r := canvas.NewRectangle(color.Black)
	r.Move(geo.NewPos(20, 10))
	r.Resize(geo.NewSize(20, 20))
	r.SetTransform(geo.NewTranslation(100, 0))
	child := canvas.NewRectangle(color.Black)
	child.Move(geo.NewPos(80, 10))
	child.Resize(geo.NewSize(10, 10))
	box := newTiltBox(geo.NewRotationAround(math.Pi/2, 50, 50), child)
	box.Move(geo.NewPos(40, 40))
	box.Resize(geo.NewSize(100, 100))
	root := container.NewWithoutLayout(r, box)
	root.Resize(geo.NewSize(200, 200))
	trees := []core.CanvasObject{root}

	assert.Equal(t, geo.NewPos(20, 10), driver.AbsolutePositionForObject(r, trees))
	assert.Equal(t, geo.NewPos(120, 50), driver.AbsolutePositionForObject(child, trees))
	assert.Equal(t, geo.NewPos(120, 10), driver.AbsolutePositionForPoint(r, geo.Position{}, trees), "where it is drawn differs")
}

// An object in none of the trees maps as AbsolutePositionForObject's (0,0)
// would, and the inverse says so.
func TestPositionMapping_NotInTrees(t *testing.T) {
	stray := canvas.NewRectangle(color.Black)
	root := container.NewWithoutLayout(canvas.NewRectangle(color.Black))
	trees := []core.CanvasObject{root}
	assert.Equal(t, geo.NewPos(4, 5), driver.AbsolutePositionForPoint(stray, geo.NewPos(4, 5), trees))
	back, ok := driver.PositionForAbsolutePoint(stray, geo.NewPos(4, 5), trees)
	assert.False(t, ok)
	assert.Equal(t, geo.NewPos(4, 5), back)
}
