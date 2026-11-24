package widget_test

import (
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFABMenu_New(t *testing.T) {
	items := []*widget.FABMenuItem{
		{Label: "Add", Icon: theme.ContentAddIcon()},
		{Label: "Edit", Icon: theme.DocumentCreateIcon()},
	}
	menu := widget.NewFABMenu(theme.ContentAddIcon(), items)
	assert.NotNil(t, menu)
	assert.Len(t, menu.Items, 2)
	assert.False(t, menu.IsOpen())
}

func TestFABMenu_HideWithoutShow(t *testing.T) {
	menu := widget.NewFABMenu(theme.ContentAddIcon(), nil)
	assert.NotPanics(t, func() { menu.Hide() })
	assert.False(t, menu.IsOpen())
}

func TestFABMenu_IsOpen(t *testing.T) {
	menu := widget.NewFABMenu(theme.ContentAddIcon(), nil)
	assert.False(t, menu.IsOpen())
}

// showFABMenuAt opens a three-item FAB menu at pos on a 412×800 canvas and
// returns the menu's box as laid out, and the canvas.
func showFABMenuAt(t *testing.T, pos geo.Position) (geo.Position, geo.Size, core.Canvas) {
	t.Helper()
	w := test.NewTempWindow(t, widget.NewLabel(""))
	w.SetPadded(false)
	w.Resize(geo.NewSize(412, 800))
	c := w.Canvas()
	menu := widget.NewFABMenu(theme.ContentAddIcon(), []*widget.FABMenuItem{
		{Label: "Note", Icon: theme.DocumentCreateIcon()},
		{Label: "Photo", Icon: theme.FileImageIcon()},
		{Label: "Voice memo", Icon: theme.MediaRecordIcon()},
	})
	menu.Show(c, pos)
	t.Cleanup(menu.Hide)
	pop, ok := c.Overlays().Top().(*widget.PopUp)
	require.True(t, ok, "the menu is the top overlay")
	return pop.Content.Position(), pop.Content.Size(), c
}

func assertOnCanvas(t *testing.T, p geo.Position, s geo.Size, c core.Canvas) {
	t.Helper()
	assert.GreaterOrEqual(t, p.X, float32(0), "left edge")
	assert.GreaterOrEqual(t, p.Y, float32(0), "top edge")
	assert.LessOrEqual(t, p.X+s.Width, c.Size().Width, "right edge")
	assert.LessOrEqual(t, p.Y+s.Height, c.Size().Height, "bottom edge")
}

// A 56 dp FAB 16 dp in from a side of a 412-wide canvas, its top at y 728.
var (
	fabTopRightOfRightFAB = geo.NewPos(412-16, 728)
	fabTopRightOfLeftFAB  = geo.NewPos(16+56, 728)
	fabTopLeftOfLeftFAB   = geo.NewPos(16, 728)
	fabTopLeftOfRightFAB  = geo.NewPos(412-16-56, 728)
)

// Left to right, a FAB at the trailing edge: the menu opens up and to the
// left of pos, its right edge on pos.X, 8 above it (#1424).
func TestFABMenu_Show_LTR_TrailingFAB(t *testing.T) {
	test.NewTempApp(t)
	p, s, c := showFABMenuAt(t, fabTopRightOfRightFAB)
	assert.Equal(t, fabTopRightOfRightFAB.X, p.X+s.Width, "right edge on pos.X")
	assert.Equal(t, fabTopRightOfRightFAB.Y-8, p.Y+s.Height, "bottom edge 8 above pos")
	assertOnCanvas(t, p, s, c)
}

// Left to right, a FAB at the leading edge: opening up and to the left would
// put the menu off the canvas, so it is moved inside, flush with the left
// edge. Both corners the issue tried are covered (#1424).
func TestFABMenu_Show_LTR_LeadingFABStaysOnCanvas(t *testing.T) {
	test.NewTempApp(t)
	for _, pos := range []geo.Position{fabTopRightOfLeftFAB, fabTopLeftOfLeftFAB} {
		p, s, c := showFABMenuAt(t, pos)
		assert.Equal(t, float32(0), p.X, "flush with the left edge, for pos %v", pos)
		assertOnCanvas(t, p, s, c)
	}
}

// Right to left mirrors it: pos is the FAB's top-left corner and the menu
// opens up and to the right, its left edge on pos.X (#1424).
func TestFABMenu_Show_RTL_TrailingFAB(t *testing.T) {
	test.NewTempApp(t)
	bellui.SetLayoutDirection(bellui.DirectionRTL)
	t.Cleanup(func() { bellui.SetLayoutDirection(bellui.DirectionAuto) })
	p, s, c := showFABMenuAt(t, fabTopLeftOfLeftFAB)
	assert.Equal(t, fabTopLeftOfLeftFAB.X, p.X, "left edge on pos.X")
	assert.Equal(t, fabTopLeftOfLeftFAB.Y-8, p.Y+s.Height, "bottom edge 8 above pos")
	assertOnCanvas(t, p, s, c)
}

// Right to left, a FAB at the right (leading) edge: the menu is moved inside,
// flush with the right edge (#1424).
func TestFABMenu_Show_RTL_LeadingFABStaysOnCanvas(t *testing.T) {
	test.NewTempApp(t)
	bellui.SetLayoutDirection(bellui.DirectionRTL)
	t.Cleanup(func() { bellui.SetLayoutDirection(bellui.DirectionAuto) })
	p, s, c := showFABMenuAt(t, fabTopLeftOfRightFAB)
	assert.Equal(t, c.Size().Width, p.X+s.Width, "flush with the right edge")
	assertOnCanvas(t, p, s, c)
}

// A FAB near the top: the menu cannot fit above it, so it is pinned to the
// top edge rather than opening above the canvas.
func TestFABMenu_Show_NearTopStaysOnCanvas(t *testing.T) {
	test.NewTempApp(t)
	p, s, c := showFABMenuAt(t, geo.NewPos(412-16, 40))
	assert.Equal(t, float32(0), p.Y, "pinned to the top edge")
	assertOnCanvas(t, p, s, c)
}
