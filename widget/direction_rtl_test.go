package widget_test

import (
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"

	"github.com/stretchr/testify/assert"
)

// ButtonGroup lays out its buttons as a horizontal row; under RTL the row is
// reflected so the first button sits on the right.
func TestButtonGroup_RTLMirror(t *testing.T) {
	test.NewTempApp(t)
	t.Cleanup(func() { bellui.SetLayoutDirection(bellui.DirectionAuto) })

	first := widget.NewButton("A", nil)
	last := widget.NewButton("C", nil)
	g := widget.NewButtonGroup(first, widget.NewButton("B", nil), last)
	test.TempWidgetRenderer(t, g)

	bellui.SetLayoutDirection(bellui.DirectionLTR)
	g.Resize(geo.NewSize(300, 40))
	assert.Less(t, first.Position().X, last.Position().X, "LTR: first button on the left")

	bellui.SetLayoutDirection(bellui.DirectionRTL)
	g.Refresh()
	assert.Greater(t, first.Position().X, last.Position().X, "RTL: first button mirrors to the right")
}

// Chip is an icon|label|delete element row; under RTL the leading icon mirrors
// to the right of the label.
func TestChip_RTLMirror(t *testing.T) {
	test.NewTempApp(t)
	t.Cleanup(func() { bellui.SetLayoutDirection(bellui.DirectionAuto) })

	chip := widget.NewChip("Tag", nil)
	chip.Icon = theme.HomeIcon()
	r := test.TempWidgetRenderer(t, chip)

	bellui.SetLayoutDirection(bellui.DirectionLTR)
	chip.Resize(geo.NewSize(160, 32))
	icon, label := chipIconLabelX(r)
	assert.Less(t, icon, label, "LTR: icon left of label")

	bellui.SetLayoutDirection(bellui.DirectionRTL)
	chip.Refresh()
	icon, label = chipIconLabelX(r)
	assert.Greater(t, icon, label, "RTL: icon mirrors to the right of label")
}

// chipIconLabelX returns the x of the chip's icon (*canvas.Image) and label
// (*canvas.Text) from the renderer's objects.
func chipIconLabelX(r core.WidgetRenderer) (iconX, labelX float32) {
	for _, o := range r.Objects() {
		switch obj := o.(type) {
		case *canvas.Image:
			iconX = obj.Position().X
		case *canvas.Text:
			labelX = obj.Position().X
		}
	}
	return iconX, labelX
}
