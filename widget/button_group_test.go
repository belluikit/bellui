package widget_test

import (
	"testing"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"

	"github.com/stretchr/testify/assert"
)

func TestButtonGroup_New(t *testing.T) {
	test.NewTempApp(t)
	b1 := widget.NewButton("A", nil)
	b2 := widget.NewButton("B", nil)
	g := widget.NewButtonGroup(b1, b2)
	assert.NotNil(t, g)
	assert.Len(t, g.Buttons, 2)
}

func TestButtonGroup_MinSize(t *testing.T) {
	test.NewTempApp(t)
	b1 := widget.NewButton("Save", nil)
	b2 := widget.NewButton("Cancel", nil)
	g := widget.NewButtonGroup(b1, b2)
	min := g.MinSize()
	assert.Greater(t, min.Width, float32(0))
	assert.Greater(t, min.Height, float32(0))
}

func TestButtonGroup_Renderer(t *testing.T) {
	test.NewTempApp(t)
	g := widget.NewButtonGroup(widget.NewButton("OK", nil))
	assert.NotPanics(t, func() { test.TempWidgetRenderer(t, g) })
}

func TestButtonGroup_Empty(t *testing.T) {
	test.NewTempApp(t)
	g := widget.NewButtonGroup()
	assert.NotNil(t, g)
	assert.Empty(t, g.Buttons)
}

// Buttons is a public slice, so a host can replace, append to or truncate it
// after the renderer exists — the config-driven rebuild in belluikit/player#13
// does exactly that. Refresh must re-sync what is drawn with what is measured
// (#203).
func TestButtonGroup_RefreshResyncsButtons(t *testing.T) {
	test.NewTempApp(t)
	g := widget.NewButtonGroup(widget.NewButton("A", nil))
	r := test.TempWidgetRenderer(t, g)
	assert.Len(t, r.Objects(), 1)

	// Wholesale replacement: the original must go, both new ones must draw.
	x, y := widget.NewButton("X", nil), widget.NewButton("Y", nil)
	g.Buttons = []*widget.Button{x, y}
	g.Refresh()
	assert.Equal(t, []core.CanvasObject{x, y}, r.Objects(),
		"the drawn objects follow the current Buttons")

	// Append.
	z := widget.NewButton("Z", nil)
	g.Buttons = append(g.Buttons, z)
	g.Refresh()
	assert.Equal(t, []core.CanvasObject{x, y, z}, r.Objects())

	// Truncate — a removed button must not linger in the object list.
	g.Buttons = g.Buttons[:1]
	g.Refresh()
	assert.Equal(t, []core.CanvasObject{x}, r.Objects())

	// Emptying is legal too, and MinSize agrees with the drawn set.
	g.Buttons = nil
	g.Refresh()
	assert.Empty(t, r.Objects())
	assert.Equal(t, geo.NewSize(0, 0), r.MinSize())
}

// A button that never went through NewButtonGroup still gets extended, so it
// renders instead of silently missing its renderer (#203).
func TestButtonGroup_RefreshExtendsAddedButtons(t *testing.T) {
	test.NewTempApp(t)
	g := widget.NewButtonGroup()
	r := test.TempWidgetRenderer(t, g)

	late := &widget.Button{Text: "late"}
	g.Buttons = []*widget.Button{late}
	g.Refresh()

	assert.Equal(t, []core.CanvasObject{late}, r.Objects())
	assert.NotPanics(t, func() { late.Refresh() }, "the added button was extended")
	assert.Greater(t, r.MinSize().Width, float32(0))
}
