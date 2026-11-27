package container

import (
	"image/color"
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"

	"github.com/stretchr/testify/assert"
)

// rtlCases runs f under the app-wide right-to-left direction and inside a
// right-to-left DirectionScope in a left-to-right window. The drag handlers
// run outside a layout walk, so the scope case checks they find the
// direction of the object itself.
func rtlCases(t *testing.T, f func(t *testing.T, show func(core.CanvasObject))) {
	for _, scoped := range []bool{false, true} {
		name := "global"
		if scoped {
			name = "scope"
		}
		t.Run(name, func(t *testing.T) {
			test.NewTempApp(t)
			show := func(o core.CanvasObject) {
				w := test.NewTempWindow(t, o)
				if scoped {
					w.SetContent(NewDirectionScope(o, bellui.DirectionRTL))
				}
				w.Resize(geo.NewSize(200, 100))
			}
			if !scoped {
				bellui.SetLayoutDirection(bellui.DirectionRTL)
				defer bellui.SetLayoutDirection(bellui.DirectionAuto)
			}
			f(t, show)
		})
	}
}

// smallRect has a 10 by 10 minimum, small enough not to clamp a share.
func smallRect() *canvas.Rectangle {
	r := canvas.NewRectangle(color.Black)
	r.SetMinSize(geo.NewSize(10, 10))
	return r
}

// Right-to-left, a horizontal Split puts Leading on the right and its
// divider drag follows: dragging right shrinks Leading (#779). A vertical
// Split is unchanged.
func TestSplit_RTL(t *testing.T) {
	rtlCases(t, func(t *testing.T, show func(core.CanvasObject)) {
		lead, trail := smallRect(), smallRect()
		split := NewHSplit(lead, trail)
		split.SetOffset(0.3)
		show(split)
		split.Resize(geo.NewSize(108, 50))

		assert.Greater(t, lead.Position().X, trail.Position().X, "Leading is on the right")
		assert.Equal(t, split.Size().Width, lead.Position().X+lead.Size().Width, "Leading ends at the right edge")
		assert.InDelta(t, 30, lead.Size().Width, 0.01, "Leading keeps its 30% share")

		split.SetOffset(0.5)
		divider := test.TempWidgetRenderer(t, split).(*splitContainerRenderer).divider
		divider.Dragged(&event.DragEvent{
			PointEvent: event.PointEvent{Position: geo.NewPos(13, 9)},
			Dragged:    geo.NewDelta(10, 0),
		})
		divider.DragEnd()
		assert.InDelta(t, 0.4, split.Offset, 1e-9, "dragging right takes from Leading")

		top, bottom := smallRect(), smallRect()
		v := NewVSplit(top, bottom)
		show(v)
		v.Resize(geo.NewSize(108, 108))
		assert.Equal(t, top.Position().X, bottom.Position().X, "a vertical split does not mirror")
		assert.Less(t, top.Position().Y, bottom.Position().Y)
	})
}

// Right-to-left, horizontal Paned panels run from the right, and dragging
// a divider right takes from the panel before it in Panels, which is now
// on its right (#779).
func TestPaned_RTL(t *testing.T) {
	rtlCases(t, func(t *testing.T, show func(core.CanvasObject)) {
		a, b, c := smallRect(), smallRect(), smallRect()
		p := NewHPaned(a, b, c)
		show(p)
		p.Resize(geo.NewSize(316, 50))

		assert.Greater(t, a.Position().X, b.Position().X, "the first panel is rightmost")
		assert.Greater(t, b.Position().X, c.Position().X)
		assert.Equal(t, p.Size().Width, a.Position().X+a.Size().Width, "the first panel ends at the right edge")

		r := test.TempWidgetRenderer(t, p).(*panedRenderer)
		before := a.Size().Width
		r.dividers[0].Dragged(&event.DragEvent{
			PointEvent: event.PointEvent{Position: geo.NewPos(2, 9)},
			Dragged:    geo.NewDelta(20, 0),
		})
		r.dividers[0].DragEnd()
		assert.InDelta(t, before-20, a.Size().Width, 0.5, "dragging right takes from the first panel")
		assert.Greater(t, r.dividers[0].Position().X, b.Position().X, "the divider sits between the first two panels")
		assert.Less(t, r.dividers[0].Position().X, a.Position().X)
	})
}
