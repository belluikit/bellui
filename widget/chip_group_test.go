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

// TestChipGroup_Alignment lays out chips that wrap over three rows, the last
// one shorter, and checks where each row sits for every alignment, in both
// directions (#1371).
func TestChipGroup_Alignment(t *testing.T) {
	test.NewTempApp(t)
	options := []string{"aaaa", "bbbb", "cccc", "dddd", "eeee", "ffff", "gggg"}
	names := map[bellui.TextAlign]string{
		bellui.TextAlignLeading: "start", bellui.TextAlignCenter: "center", bellui.TextAlignTrailing: "end",
	}

	for _, rtl := range []bool{false, true} {
		for _, align := range []bellui.TextAlign{bellui.TextAlignLeading, bellui.TextAlignCenter, bellui.TextAlignTrailing} {
			name := names[align]
			if rtl {
				name += "/rtl"
			}
			t.Run(name, func(t *testing.T) {
				dir := bellui.DirectionLTR
				if rtl {
					dir = bellui.DirectionRTL
				}
				bellui.SetLayoutDirection(dir)
				defer bellui.SetLayoutDirection(bellui.DirectionAuto)

				g := widget.NewChipGroup(options, true, nil)
				g.Alignment = align
				chips := chipsOf(t, g)
				gap := 2 * theme.Padding()
				var widest float32
				for _, c := range chips {
					widest = max(widest, c.MinSize().Width)
				}
				// Room for three chips a row but not four: rows of 3, 3, 1.
				width := 3*widest + 3*gap + widest/2
				g.Resize(geo.NewSize(width, 500))

				rows := [][]*widget.Chip{chips[0:3], chips[3:6], chips[6:7]}
				for i, row := range rows {
					y := row[0].Position().Y
					left, right := width, float32(0)
					var rowW float32
					for j, c := range row {
						assert.Equal(t, y, c.Position().Y, "row %d chip %d shares the row", i, j)
						left = min(left, c.Position().X)
						right = max(right, c.Position().X+c.Size().Width)
						rowW += c.Size().Width
					}
					rowW += float32(len(row)-1) * gap
					if i < 2 {
						assert.Less(t, y, chips[3*(i+1)].Position().Y, "row %d ends before the next chip", i)
					}
					assert.InDelta(t, rowW, right-left, 0.01, "row %d is packed", i)

					var wantLeft float32
					switch {
					case align == bellui.TextAlignCenter:
						wantLeft = (width - rowW) / 2
					case (align == bellui.TextAlignLeading) != rtl:
						wantLeft = 0
					default:
						wantLeft = width - rowW
					}
					assert.InDelta(t, wantLeft, left, 0.01, "row %d's left edge", i)

					first := row[0]
					if rtl {
						assert.InDelta(t, right, first.Position().X+first.Size().Width, 0.01, "row %d starts at its right", i)
					} else {
						assert.InDelta(t, left, first.Position().X, 0.01, "row %d starts at its left", i)
					}
				}
			})
		}
	}
}
