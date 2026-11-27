package widget_test

import (
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"

	"github.com/stretchr/testify/assert"
)

// alignedButtons is a leading, a centred and a trailing button with an icon,
// and an Accordion, whose headers are leading-aligned buttons.
func alignedButtons() (lead, centre, trail *widget.Button, acc *widget.Accordion, all core.CanvasObject) {
	mk := func(text string, a widget.ButtonAlign) *widget.Button {
		b := widget.NewButtonWithIcon(text, theme.HomeIcon(), nil)
		b.Alignment = a
		return b
	}
	lead = mk("Leading", widget.ButtonAlignLeading)
	centre = mk("Center", widget.ButtonAlignCenter)
	trail = mk("Trailing", widget.ButtonAlignTrailing)
	acc = widget.NewAccordion(widget.NewAccordionItem("Section", widget.NewLabel("Body text")))
	acc.Open(0)
	return lead, centre, trail, acc, container.NewVBox(lead, centre, trail, acc)
}

// contentX is where a button's icon and text start and end, read from the
// renderer: the leftmost and rightmost edge among its visible objects that
// are narrower than the button (its background, shadow and press layer span
// the whole button).
func contentX(t *testing.T, b *widget.Button) (left, right float32) {
	left, right = b.Size().Width, 0
	for _, o := range test.TempWidgetRenderer(t, b).Objects() {
		if !o.Visible() || o.Size().Width >= b.Size().Width {
			continue
		}
		left = min(left, o.Position().X)
		right = max(right, o.Position().X+o.Size().Width)
	}
	return left, right
}

// Leading and Trailing are the reading direction's start and end: right to
// left, a leading button's icon and text sit at the right edge, and an
// Accordion header with them (#779). Centred stays centred, and left to
// right is unchanged.
func TestButtonAlignment_Direction(t *testing.T) {
	for _, c := range []struct {
		name string
		dir  bellui.Direction
	}{{"ltr", bellui.DirectionLTR}, {"rtl", bellui.DirectionRTL}} {
		t.Run(c.name, func(t *testing.T) {
			test.NewTempApp(t)
			bellui.SetLayoutDirection(c.dir)
			defer bellui.SetLayoutDirection(bellui.DirectionAuto)
			lead, centre, trail, _, all := alignedButtons()
			w := test.NewTempWindow(t, all)
			w.Resize(geo.NewSize(300, 260))
			test.AssertRendersToImage(t, "button_align/"+c.name+".png", w.Canvas())
			assertAligned(t, c.dir == bellui.DirectionRTL, lead, centre, trail)
		})
	}
}

// The same holds inside a right-to-left DirectionScope in a left-to-right
// window, including after a Refresh, which lays a button out outside a walk.
func TestButtonAlignment_Scope(t *testing.T) {
	test.NewTempApp(t)
	lead, centre, trail, _, all := alignedButtons()
	w := test.NewTempWindow(t, container.NewDirectionScope(all, bellui.DirectionRTL))
	w.Resize(geo.NewSize(300, 260))
	assertAligned(t, true, lead, centre, trail)
	lead.SetText("Leading, renamed")
	assertAligned(t, true, lead, centre, trail)
}

func assertAligned(t *testing.T, rtl bool, lead, centre, trail *widget.Button) {
	t.Helper()
	width := lead.Size().Width
	ll, lr := contentX(t, lead)
	tl, tr := contentX(t, trail)
	cl, cr := contentX(t, centre)
	assert.InDelta(t, width/2, (cl+cr)/2, 1, "centred content stays centred")
	if rtl {
		assert.Greater(t, ll, width/2, "right to left, leading content is at the right")
		assert.Less(t, tr, width/2, "right to left, trailing content is at the left")
	} else {
		assert.Less(t, lr, width/2, "left to right, leading content is at the left")
		assert.Greater(t, tl, width/2, "left to right, trailing content is at the right")
	}
}
