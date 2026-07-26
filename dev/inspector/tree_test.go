package inspector

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
)

func TestSnapshotRoots_HierarchyAndAbsolutePositions(t *testing.T) {
	test.NewTempApp(t)

	rectA := canvas.NewRectangle(color.Black)
	rectA.Move(geo.NewPos(10, 20))
	rectA.Resize(geo.NewSize(30, 40))

	inner := canvas.NewText("hi", color.Black)
	inner.Move(geo.NewPos(5, 5))
	inner.Resize(geo.NewSize(20, 10))
	sub := container.NewWithoutLayout(inner)
	sub.Move(geo.NewPos(100, 0))
	sub.Resize(geo.NewSize(200, 200))

	root := container.NewWithoutLayout(rectA, sub)
	root.Resize(geo.NewSize(300, 300))

	s := snapshotRoots([]core.CanvasObject{root})

	require.Len(t, s.roots, 1)
	r := s.roots[0]
	assert.Equal(t, kindContainer, r.kind)
	require.Len(t, r.children, 2)

	// child A: absolute position is the parent origin plus its own position.
	assert.Equal(t, geo.NewPos(10, 20), r.children[0].absPos)
	assert.Equal(t, kindPrimitive, r.children[0].kind)

	// nested container accumulates offsets down the tree.
	subNode := r.children[1]
	require.Len(t, subNode.children, 1)
	assert.Equal(t, geo.NewPos(105, 5), subNode.children[0].absPos)

	// byUID and byObj resolve the same node.
	leaf := subNode.children[0]
	assert.Same(t, leaf, s.byUID[leaf.uid])
	assert.Same(t, leaf, s.byObj[inner])
	assert.Equal(t, "0/1/0", leaf.uid)
}

func TestSnapshotRoots_DescendsWidgetRenderer(t *testing.T) {
	test.NewTempApp(t)

	label := widget.NewLabel("hello")
	w := test.NewTempWindow(t, label)

	s := snapshotRoots(w.Canvas().Overlays().List()) // empty
	assert.Empty(t, s.roots)

	s = snapshotRoots([]core.CanvasObject{w.Canvas().Content()})

	var labelNode *node
	for _, n := range s.byObj {
		if _, ok := n.obj.(*widget.Label); ok {
			labelNode = n
		}
	}
	require.NotNil(t, labelNode, "label should appear in the walked tree")
	assert.Equal(t, kindWidget, labelNode.kind)
	assert.NotEmpty(t, labelNode.children, "renderer internals should be descended into")
}

func TestSnapshotChildAndBranchQueries(t *testing.T) {
	test.NewTempApp(t)

	root := container.NewWithoutLayout(canvas.NewRectangle(color.Black))
	root.Resize(geo.NewSize(50, 50))
	s := snapshotRoots([]core.CanvasObject{root})

	// the empty UID is the synthetic tree root over the forest.
	assert.Equal(t, []string{"0"}, s.childUIDs(""))
	assert.True(t, s.isBranch(""))
	assert.True(t, s.isBranch("0"))    // container with a child
	assert.False(t, s.isBranch("0/0")) // the leaf rectangle
	assert.Empty(t, s.childUIDs("nope"))
}

func TestParentUID(t *testing.T) {
	assert.Equal(t, "", parentUID("0"))
	assert.Equal(t, "0", parentUID("0/2"))
	assert.Equal(t, "0/2", parentUID("0/2/1"))
}

func TestNodeLabel(t *testing.T) {
	assert.Equal(t, "?", nodeLabel(nil))
	assert.Equal(t, "*widget.Label", nodeLabel(&node{typeName: "*widget.Label", visible: true}))
	assert.Equal(t, "*widget.Label  (hidden)", nodeLabel(&node{typeName: "*widget.Label"}))
}
