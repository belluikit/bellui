package common

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	intdriver "github.com/belluikit/bellui/internal/driver"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
)

// inertHolder is a disableable widget that lays its child out in its own
// bounds, standing for any disabled container.
type inertHolder struct {
	widget.DisableableWidget
	child core.CanvasObject
}

func (h *inertHolder) CreateRenderer() core.WidgetRenderer {
	return widget.NewSimpleRenderer(h.child)
}

// tapTarget is a tappable that cannot be disabled, like a Hyperlink.
type tapTarget struct{ *canvas.Rectangle }

func (*tapTarget) Tapped(*event.PointEvent) {}

// selectableHolder is a disabled widget whose text stays selectable, like a
// disabled Entry: its child, the selection layer, still takes input.
type selectableHolder struct{ inertHolder }

func (*selectableHolder) SelectableWhenDisabled() bool { return true }

// TestSpatialIndexReportsInert pins #1378 on the hit test the drivers use
// while the canvas is clean: a hit inside a disabled widget is reported
// inert, read from the widget's state at the hit, not when the index was
// built, as the tree-walk hit test reports it. A hit under an object that
// keeps its text selectable while disabled is not.
func TestSpatialIndexReportsInert(t *testing.T) {
	test.NewTempApp(t)

	link := &tapTarget{canvas.NewRectangle(color.White)}
	selChild := &tapTarget{canvas.NewRectangle(color.White)}
	sel := &selectableHolder{inertHolder{child: selChild}}
	sel.ExtendBaseWidget(sel)
	sel.Disable()
	holder := &inertHolder{child: container.NewGridWithColumns(2, link, sel)}
	holder.ExtendBaseWidget(holder)
	holder.Resize(geo.NewSize(200, 100))
	content := container.NewWithoutLayout(holder)
	content.Resize(geo.NewSize(400, 400))

	c := &Canvas{}
	c.Initialize(&windowSizedCanvas{size: geo.NewSize(400, 400)}, func() {})
	c.SetContentTreeAndFocusMgr(content)
	c.WalkTrees(noWalk, noWalk)
	c.RebuildSpatialIndex()
	idx := c.SpatialIndex()
	require.NotNil(t, idx)

	tappable := func(o core.CanvasObject) bool { _, ok := o.(event.Tappable); return ok }
	onLink := geo.NewPos(20, 20)
	onSelChild := geo.NewPos(120, 20)

	hit := func(at geo.Position) (core.CanvasObject, bool, bool) {
		obj, _, ok, inert := idx.FindEnabledObjectAtPositionMatching(at, tappable)
		require.True(t, ok)
		wobj, _, _, winert := intdriver.FindEnabledObjectAtPositionMatching(at, tappable, nil, content)
		assert.Equal(t, obj, wobj, "the index and the tree walk hit different objects")
		return obj, inert, winert
	}

	obj, inert, winert := hit(onLink)
	assert.Same(t, link, obj)
	assert.False(t, inert)
	assert.False(t, winert)

	holder.Disable() // no rebuild: the index reads the state at the hit
	obj, inert, winert = hit(onLink)
	assert.Same(t, link, obj)
	assert.True(t, inert, "the index missed the disabled holder")
	assert.True(t, winert, "the tree walk missed the disabled holder")

	obj, inert, winert = hit(onSelChild)
	assert.Same(t, selChild, obj)
	assert.False(t, inert, "a hit under a selectable-when-disabled object is not inert (index)")
	assert.False(t, winert, "a hit under a selectable-when-disabled object is not inert (walk)")
}
