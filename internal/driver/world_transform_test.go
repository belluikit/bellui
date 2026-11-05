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

// TestFindObjectAtPosition_WorldTransform pins #997 for the hit-test: an object
// is hit where its world transform draws it, translation included, and not
// where it would be without that translation. The scenes are the software
// painter's (TestPainter_WorldTransformMovesObjects).
func TestFindObjectAtPosition_WorldTransform(t *testing.T) {
	rect := func(x, y, w, h float32) *canvas.Rectangle {
		r := canvas.NewRectangle(color.Black)
		r.Move(geo.NewPos(x, y))
		r.Resize(geo.NewSize(w, h))
		return r
	}
	for name, tc := range map[string]struct {
		scene     func() (root, target core.CanvasObject)
		drawn, no geo.Position
		local     geo.Position // drawn, in the target's coordinates
	}{
		"translation only": {
			scene: func() (core.CanvasObject, core.CanvasObject) {
				r := rect(20, 10, 20, 20)
				r.SetTransform(geo.NewTranslation(100, 0))
				return container.NewWithoutLayout(r), r
			},
			drawn: geo.NewPos(125, 15), no: geo.NewPos(25, 15), local: geo.NewPos(5, 5),
		},
		"rotation about a pivot": {
			scene: func() (core.CanvasObject, core.CanvasObject) {
				r := rect(50, 50, 40, 20)
				r.SetTransform(geo.NewRotationAround(math.Pi, 20, 10))
				return container.NewWithoutLayout(r), r
			},
			drawn: geo.NewPos(85, 65), no: geo.NewPos(15, 35), local: geo.NewPos(5, 5),
		},
		"transformed parent, offset child": {
			scene: func() (core.CanvasObject, core.CanvasObject) {
				child := rect(80, 10, 10, 10)
				box := newTiltBox(geo.NewRotationAround(math.Pi/2, 50, 50), child)
				box.Move(geo.NewPos(40, 40))
				box.Resize(geo.NewSize(100, 100))
				return container.NewWithoutLayout(box), child
			},
			drawn: geo.NewPos(124, 126), no: geo.NewPos(125, 55), local: geo.NewPos(6, 6),
		},
	} {
		t.Run(name, func(t *testing.T) {
			root, target := tc.scene()
			root.Resize(geo.NewSize(200, 200))
			isTarget := func(o core.CanvasObject) bool { return o == target }

			found, local, _ := driver.FindObjectAtPositionMatching(tc.drawn, isTarget, nil, root)
			assert.Equal(t, target, found, "hit where it is drawn, %v", tc.drawn)
			assert.InDelta(t, tc.local.X, local.X, 0.01)
			assert.InDelta(t, tc.local.Y, local.Y, 0.01)
			found, _, _ = driver.FindObjectAtPositionMatching(tc.no, isTarget, nil, root)
			assert.Nil(t, found, "not hit where it would be without its translation, %v", tc.no)

			all := driver.FindAllObjectsAtPositionMatching(tc.drawn, isTarget, nil, root)
			if assert.Len(t, all, 1) {
				assert.InDelta(t, tc.local.X, all[0].Position.X, 0.01)
			}
			assert.Empty(t, driver.FindAllObjectsAtPositionMatching(tc.no, isTarget, nil, root))
		})
	}
}

// TestWalkVisibleObjectTreeCulled_NotByUntransformedPlace pins #997 for the
// culled walk: a transformed object is drawn where its transform puts it, so
// the walk does not ask cull about it from where it would be without one.
func TestWalkVisibleObjectTreeCulled_NotByUntransformedPlace(t *testing.T) {
	inner := canvas.NewRectangle(color.Black)
	inner.Resize(geo.NewSize(10, 10))
	// Off to the left without its transform, translated back in by it.
	box := newTiltBox(geo.NewTranslation(100, 0), inner)
	box.Move(geo.NewPos(-100, 0))
	box.Resize(geo.NewSize(50, 50))

	var asked []core.CanvasObject
	cull := func(obj core.CanvasObject, _, _ geo.Position, _ geo.Size) bool {
		asked = append(asked, obj)
		return true // everything asked is off the clip
	}
	reached := false
	driver.WalkVisibleObjectTreeCulled(box, cull, func(o core.CanvasObject, _, _ geo.Position, _ geo.Size) bool {
		reached = reached || o == inner
		return false
	}, nil)

	assert.Empty(t, asked, "a transformed object, and what is below it, is not culled from where it would be without its transform")
	assert.True(t, reached, "its subtree is walked")
}

// TestFindObjectAtPosition_ClipUnderTransform pins #997 for clips: a clipping
// container moved by a transform clips its children where it is drawn, so a
// child is hit there, and the clip is reported there.
func TestFindObjectAtPosition_ClipUnderTransform(t *testing.T) {
	inner := canvas.NewRectangle(color.Black)
	inner.Resize(geo.NewSize(80, 80)) // larger than the clip
	clip := &bellui.Container{ClipToBounds: true, Objects: []core.CanvasObject{inner}}
	clip.Move(geo.NewPos(10, 10))
	clip.Resize(geo.NewSize(40, 40))
	box := newTiltBox(geo.NewTranslation(100, 0), clip)
	box.Resize(geo.NewSize(60, 60))
	root := &bellui.Container{Objects: []core.CanvasObject{box}}
	root.Resize(geo.NewSize(200, 200))
	isInner := func(o core.CanvasObject) bool { return o == inner }

	found, local, _ := driver.FindObjectAtPositionMatching(geo.NewPos(120, 20), isInner, nil, root)
	assert.Equal(t, core.CanvasObject(inner), found, "hit inside the clip where it is drawn")
	assert.Equal(t, geo.NewPos(10, 10), local)
	found, _, _ = driver.FindObjectAtPositionMatching(geo.NewPos(20, 20), isInner, nil, root)
	assert.Nil(t, found, "not where the clip would be without the transform")
	found, _, _ = driver.FindObjectAtPositionMatching(geo.NewPos(160, 20), isInner, nil, root)
	assert.Nil(t, found, "nor past the clip's edge, where the child still reaches")

	bounds, ok := driver.ClipBoundsForObject(inner, []core.CanvasObject{root})
	assert.True(t, ok)
	assert.Equal(t, geo.Rectangle{Pos: geo.NewPos(110, 10), Size: geo.NewSize(40, 40)}, bounds)
}

// TestFindObjectAtPosition_ClipUnderRotation pins #1003 for the hit-test: a
// clip turned an eighth clips its children to the turned square, as the
// painters draw it, not to the box around it. The scene is the real-loop
// test's (TestWorldTransformDraws, "clip under a rotation"): a diamond about
// (90, 90), whose box is (47.6, 47.6)-(132.4, 132.4).
func TestFindObjectAtPosition_ClipUnderRotation(t *testing.T) {
	inner := canvas.NewRectangle(color.Black)
	inner.Move(geo.NewPos(-50, -50))
	inner.Resize(geo.NewSize(160, 160)) // past the clip on every side
	clip := &bellui.Container{ClipToBounds: true, Objects: []core.CanvasObject{inner}}
	clip.Move(geo.NewPos(20, 20))
	clip.Resize(geo.NewSize(60, 60))
	box := newTiltBox(geo.NewRotationAround(math.Pi/4, 50, 50), clip)
	box.Move(geo.NewPos(40, 40))
	box.Resize(geo.NewSize(100, 100))
	root := &bellui.Container{Objects: []core.CanvasObject{box}}
	root.Resize(geo.NewSize(200, 200))
	isInner := func(o core.CanvasObject) bool { return o == inner }

	found, _, _ := driver.FindObjectAtPositionMatching(geo.NewPos(90, 90), isInner, nil, root)
	assert.Equal(t, core.CanvasObject(inner), found, "hit inside the turned clip")
	found, _, _ = driver.FindObjectAtPositionMatching(geo.NewPos(60, 60), isInner, nil, root)
	assert.Nil(t, found, "not in the corner of the box around it, where it is not drawn")
	all := driver.FindAllObjectsAtPositionMatching(geo.NewPos(60, 60), isInner, nil, root)
	assert.Empty(t, all, "nor among all the objects there")
	found, _, _ = driver.FindObjectAtPositionMatching(geo.NewPos(60, 60), func(o core.CanvasObject) bool { return o == box }, nil, root)
	assert.Equal(t, core.CanvasObject(box), found, "what holds the clip is still hit there")
}
