package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
)

// roundedWidget's renderer clips with rounded corners of radius radius.
type roundedWidget struct {
	widget.BaseWidget
	radius float32
}

func (w *roundedWidget) CreateRenderer() core.WidgetRenderer {
	return &roundedRenderer{w: w, rect: canvas.NewRectangle(nil)}
}

type roundedRenderer struct {
	w    *roundedWidget
	rect *canvas.Rectangle
}

func (r *roundedRenderer) RoundedClipCornerRadius() float32 { return r.w.radius }
func (r *roundedRenderer) Destroy()                         {}
func (r *roundedRenderer) Layout(geo.Size)                  {}
func (r *roundedRenderer) MinSize() geo.Size                { return geo.NewSquareSize(10) }
func (r *roundedRenderer) Objects() []core.CanvasObject     { return []core.CanvasObject{r.rect} }
func (r *roundedRenderer) Refresh()                         {}

// offClip is a position far outside the 100×100 clip used below.
var offClip = geo.NewPos(0, 1000)

// cullAsWalk asks the canvas the way a walk does, with the node it visits.
func cullAsWalk(c *Canvas, n *RenderCacheNode) bool {
	c.visiting = n
	defer func() { c.visiting = nil }()
	return c.CullOffClip(n.obj, offClip, geo.Position{}, geo.NewSquareSize(100))
}

// TestCullOffClip_NodeCacheFollowsRefresh pins #599's cache: a node keeps its
// object's clip answer until the object is refreshed. A renderer that stops
// clipping without a refresh is still culled as a clip; after the refresh it
// is not.
func TestCullOffClip_NodeCacheFollowsRefresh(t *testing.T) {
	test.NewTempApp(t)
	w := &roundedWidget{radius: 8}
	w.ExtendBaseWidget(w)
	w.Resize(geo.NewSquareSize(10))
	c := newCullCanvas(w)
	n, ok := c.NodeFor(w)
	require.True(t, ok)

	assert.True(t, cullAsWalk(c, n), "a rounded clip-owner off the clip is culled")
	w.radius = 0
	assert.True(t, cullAsWalk(c, n), "without a refresh, the remembered answer stands")
	c.MarkMinSizeDirty(w) // what a refresh does
	assert.False(t, cullAsWalk(c, n), "after the refresh it no longer clips, so it is not culled")
	assert.Equal(t, CullOffClip(w, offClip, geo.Position{}, geo.NewSquareSize(100)), cullAsWalk(c, n),
		"the remembered answer matches asking the renderer")
}

// TestCullOffClip_NodeCacheFollowsTheme: a theme change measures again,
// without a refresh, as the theme decides what widgets clip (a Card's corner
// radius, for one).
func TestCullOffClip_NodeCacheFollowsTheme(t *testing.T) {
	app := test.NewTempApp(t)
	w := &roundedWidget{radius: 8}
	w.ExtendBaseWidget(w)
	w.Resize(geo.NewSquareSize(10))
	c := newCullCanvas(w)
	n, ok := c.NodeFor(w)
	require.True(t, ok)

	require.True(t, cullAsWalk(c, n))
	w.radius = 0 // no refresh
	app.Settings().SetTheme(theme.DefaultTheme())
	assert.False(t, cullAsWalk(c, n), "a new theme generation measures again")
}

func newCullCanvas(obj core.CanvasObject) *Canvas {
	content := container.NewWithoutLayout(obj)
	content.Resize(geo.NewSquareSize(100))
	c := &Canvas{}
	c.Initialize(&cullSizeable{size: geo.NewSquareSize(100)}, func() {})
	c.SetContentTreeAndFocusMgr(content)
	c.WalkTrees(nil, nil) // places obj in the tree
	return c
}

type cullSizeable struct {
	SizeableCanvas
	size geo.Size
}

func (f *cullSizeable) Size() geo.Size { return f.size }
