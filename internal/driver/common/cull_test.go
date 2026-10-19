package common

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/internal/driver"
	intwidget "github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
)

var cullClipPos, cullClipSize = geo.NewPos(0, 0), geo.NewSize(800, 600)

// TestCullOffClip pins #464's rule: only a clip-owner wholly outside the clip,
// shadow included, is culled.
func TestCullOffClip(t *testing.T) {
	test.NewTempApp(t)
	// A rounded clip, as a Card is under the default theme (the test theme
	// gives cards no corner radius).
	card := &shadowedClip{reach: 1.5}
	card.ExtendBaseWidget(card)
	card.Resize(geo.NewSize(180, 200))
	cache.Renderer(card) // a rounded clip is known from its renderer

	assert.False(t, CullOffClip(card, geo.NewPos(10, 10), cullClipPos, cullClipSize), "a card in view")
	assert.True(t, CullOffClip(card, geo.NewPos(10, 1000), cullClipPos, cullClipSize), "a card far below the clip")
	assert.True(t, CullOffClip(card, geo.NewPos(-500, 10), cullClipPos, cullClipSize), "a card far left of the clip")
	assert.False(t, CullOffClip(card, geo.NewPos(10, 600+cullMargin/2), cullClipPos, cullClipSize),
		"a card just below the clip, whose shadow may reach into it")

	rect := canvas.NewRectangle(color.Black)
	rect.Resize(geo.NewSize(50, 50))
	assert.False(t, CullOffClip(rect, geo.NewPos(10, 1000), cullClipPos, cullClipSize),
		"an object that owns no clip may draw outside its bounds and is never culled")

	scroll := container.NewVScroll(widget.NewLabel("x"))
	scroll.Resize(geo.NewSize(100, 100))
	assert.True(t, CullOffClip(scroll, geo.NewPos(10, 1000), cullClipPos, cullClipSize), "a scroll far below the clip")
}

// TestCullOffClipWidensByOwnShadow checks that a clip-owner's own shadow,
// however large, keeps it from being culled while the shadow reaches the clip.
func TestCullOffClipWidensByOwnShadow(t *testing.T) {
	test.NewTempApp(t)
	big := &shadowedClip{reach: 200}
	big.ExtendBaseWidget(big)
	big.Resize(geo.NewSize(100, 100))
	cache.Renderer(big)
	assert.Equal(t, float32(200*2+3), shadowReach(big))
	assert.False(t, CullOffClip(big, geo.NewPos(10, 600+300), cullClipPos, cullClipSize), "its shadow reaches 403 px up into the clip")
	assert.True(t, CullOffClip(big, geo.NewPos(10, 600+cullMargin+404), cullClipPos, cullClipSize), "beyond its shadow's reach")
}

// TestCullMarginCoversShadows pins cullMargin's assumption: the shadows of
// the elevation levels widgets use inside window content reach no further. A
// clip-owner's own shadow is measured, so this guards shadows of descendants
// that are not clip-owners themselves. DialogLevel is drawn in overlays, which
// the paint walk never culls.
func TestCullMarginCoversShadows(t *testing.T) {
	for _, level := range []intwidget.ElevationLevel{
		intwidget.BaseLevel, intwidget.CardLevel, intwidget.ButtonLevel, intwidget.MenuLevel,
		intwidget.PopUpLevel, intwidget.SubmergedContentLevel,
	} {
		blur := float32(level) * 1.5 // shadowRenderer.createShadows
		reach := blur*2 + float32(level)*0.5
		assert.LessOrEqual(t, reach, float32(cullMargin), "elevation level %d's shadow reaches %v px", level, reach)
	}
}

// TestWalkCulledSkipsChildren checks the culled walk: a culled object is
// visited, before and after, but its children are not; and culling stops
// below a transformed object.
func TestWalkCulledSkipsChildren(t *testing.T) {
	inner := canvas.NewRectangle(color.Black)
	culled := container.NewWithoutLayout(inner)
	kept := canvas.NewRectangle(color.White)
	root := container.NewWithoutLayout(culled, kept)
	cull := func(obj core.CanvasObject, _, _ geo.Position, _ geo.Size) bool { return obj == culled }

	var before, after []core.CanvasObject
	driver.WalkVisibleObjectTreeCulled(root, cull,
		func(o core.CanvasObject, _, _ geo.Position, _ geo.Size) bool {
			before = append(before, o)
			return false
		},
		func(o core.CanvasObject, _ geo.Position, _ core.CanvasObject) { after = append(after, o) })
	assert.Equal(t, []core.CanvasObject{root, culled, kept}, before, "the culled object is visited, its child is not")
	assert.Equal(t, []core.CanvasObject{culled, kept, root}, after)

	// Below a transform, nothing is culled. Only custom widgets can carry one
	// over children today; canvas primitives have none.
	moved := &transformedWidget{child: culled}
	moved.ExtendBaseWidget(moved)
	before = before[:0]
	driver.WalkVisibleObjectTreeCulled(container.NewWithoutLayout(moved), cull,
		func(o core.CanvasObject, _, _ geo.Position, _ geo.Size) bool {
			before = append(before, o)
			return false
		}, nil)
	require.Len(t, before, 4)
	assert.Equal(t, inner, before[3], "below a transformed object the walk reaches every child")
}

// transformedWidget is a widget with a rotation over its one child.
type transformedWidget struct {
	widget.BaseWidget
	child core.CanvasObject
}

func (w *transformedWidget) Transform() geo.Matrix   { return geo.NewRotation(0.5) }
func (w *transformedWidget) SetTransform(geo.Matrix) {}

func (w *transformedWidget) CreateRenderer() core.WidgetRenderer {
	return &shadowedClipRenderer{objects: []core.CanvasObject{w.child}}
}

// shadowedClip is a rounded-clip widget casting a large box shadow.
type shadowedClip struct {
	widget.BaseWidget
	reach float32
}

func (s *shadowedClip) CreateRenderer() core.WidgetRenderer {
	shadow := canvas.NewBoxShadow(color.Black, s.reach)
	shadow.OffsetY = 3
	return &shadowedClipRenderer{objects: []core.CanvasObject{shadow}}
}

type shadowedClipRenderer struct{ objects []core.CanvasObject }

func (r *shadowedClipRenderer) Destroy()                         {}
func (r *shadowedClipRenderer) Layout(geo.Size)                  {}
func (r *shadowedClipRenderer) MinSize() geo.Size                { return geo.NewSize(10, 10) }
func (r *shadowedClipRenderer) Objects() []core.CanvasObject     { return r.objects }
func (r *shadowedClipRenderer) Refresh()                         {}
func (r *shadowedClipRenderer) RoundedClipCornerRadius() float32 { return 8 }
