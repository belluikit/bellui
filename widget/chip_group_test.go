package widget_test

import (
	"testing"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// chipsOf returns the group's chips, in option order.
func chipsOf(t *testing.T, g *widget.ChipGroup) []*widget.Chip {
	t.Helper()
	var out []*widget.Chip
	for _, o := range test.TempWidgetRenderer(t, g).Objects() {
		c, ok := o.(*widget.Chip)
		require.True(t, ok, "a %T among the chips", o)
		out = append(out, c)
	}
	return out
}

func TestChipGroup_Multiple(t *testing.T) {
	test.NewTempApp(t)
	var got []string
	fired := 0
	g := widget.NewChipGroup([]string{"Rock", "Jazz", "Pop"}, true, func(s []string) { got = s; fired++ })
	chips := chipsOf(t, g)
	require.Len(t, chips, 3)
	for _, c := range chips {
		assert.Equal(t, widget.ChipStyleFilter, c.Style)
	}

	test.Tap(chips[1])
	test.Tap(chips[0])
	assert.Equal(t, []string{"Jazz", "Rock"}, got, "in the order selected")
	assert.True(t, chips[0].Selected)
	assert.True(t, chips[1].Selected)
	assert.False(t, chips[2].Selected)

	test.Tap(chips[1])
	assert.Equal(t, []string{"Rock"}, got)
	assert.False(t, chips[1].Selected)
	assert.Equal(t, 3, fired)
}

func TestChipGroup_Single(t *testing.T) {
	test.NewTempApp(t)
	var got []string
	g := widget.NewChipGroup([]string{"3-5", "6-8", "9-12"}, false, func(s []string) { got = s })
	chips := chipsOf(t, g)

	test.Tap(chips[0])
	assert.Equal(t, []string{"3-5"}, got)
	test.Tap(chips[2])
	assert.Equal(t, []string{"9-12"}, got, "another chip moves the selection")
	assert.False(t, chips[0].Selected)
	assert.True(t, chips[2].Selected)

	test.Tap(chips[2])
	assert.Empty(t, got, "the selected chip deselects")
}

func TestChipGroup_Required(t *testing.T) {
	test.NewTempApp(t)
	for _, multiple := range []bool{false, true} {
		fired := 0
		g := widget.NewChipGroup([]string{"a", "b"}, multiple, func([]string) { fired++ })
		g.Required = true
		g.Selected = []string{"a"}
		g.Refresh()
		chips := chipsOf(t, g)

		test.Tap(chips[0])
		assert.Equal(t, []string{"a"}, g.Selected, "multiple=%v: the last one stays", multiple)
		assert.True(t, chips[0].Selected)
		assert.Zero(t, fired, "multiple=%v: nothing changed, nothing fired", multiple)
	}
}

func TestChipGroup_Disabled(t *testing.T) {
	test.NewTempApp(t)
	fired := 0
	g := widget.NewChipGroup([]string{"a", "b"}, true, func([]string) { fired++ })
	chips := chipsOf(t, g)

	g.Disable()
	for _, c := range chips {
		assert.True(t, c.Disabled())
	}
	test.Tap(chips[0])
	assert.Zero(t, fired)
	assert.Empty(t, g.Selected)

	g.Enable()
	assert.False(t, chips[0].Disabled())
	test.Tap(chips[0])
	assert.Equal(t, 1, fired)
}

func TestChipGroup_OptionsChange(t *testing.T) {
	test.NewTempApp(t)
	g := widget.NewChipGroup([]string{"a", "b", "b", "c"}, true, nil)
	g.Selected = []string{"c"}
	g.Refresh()
	chips := chipsOf(t, g)
	require.Len(t, chips, 3, "duplicates dropped")
	assert.True(t, chips[2].Selected)

	g.Options = []string{"c", "d"}
	g.Refresh()
	chips = chipsOf(t, g)
	require.Len(t, chips, 2)
	assert.Equal(t, "c", chips[0].Text)
	assert.True(t, chips[0].Selected)
	assert.Equal(t, "d", chips[1].Text)
	assert.False(t, chips[1].Selected)
}

// The chips keep their own widths and wrap; the group can get narrower
// than a row once laid out (#1346).
func TestChipGroup_Wraps(t *testing.T) {
	test.NewTempApp(t)
	g := widget.NewChipGroup([]string{"Ocean", "Bugs and plants", "Dinosaurs", "Space", "Animals of the farm"}, true, nil)
	w := test.NewWindow(g)
	defer w.Close()
	w.Resize(geo.NewSize(260, 400))
	chips := chipsOf(t, g)

	var widest float32
	rows := map[float32]bool{}
	for _, c := range chips {
		assert.Equal(t, c.MinSize().Width, c.Size().Width, "%q at its own width", c.Text)
		widest = max(widest, c.MinSize().Width)
		rows[c.Position().Y] = true
		assert.LessOrEqual(t, c.Position().X+c.Size().Width, g.Size().Width, "%q inside the group", c.Text)
	}
	assert.Greater(t, len(rows), 1, "the chips wrap")
	assert.Equal(t, widest, g.MinSize().Width)
	assert.Greater(t, g.MinSize().Height, chips[0].Size().Height, "the rows' height")
}

func TestChipGroup_Interfaces(t *testing.T) {
	var _ core.Disableable = widget.NewChipGroup(nil, false, nil)
}
