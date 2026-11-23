package widget_test

import (
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/lang"
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

// newExcludable is a multiple-choice group that opts into the excluded
// state, recording what each callback reports and how often it fires.
func newExcludable(t *testing.T, options ...string) (g *widget.ChipGroup, sel, exc *[]string, selFired, excFired *int) {
	t.Helper()
	lang.SetLocaleForTests(t, "en-US") // the labels below are English
	sel, exc = new([]string), new([]string)
	selFired, excFired = new(int), new(int)
	g = widget.NewChipGroup(options, true, func(s []string) { *sel = s; *selFired++ })
	g.Excludable = true
	g.OnExcludedChanged = func(e []string) { *exc = e; *excFired++ }
	return g, sel, exc, selFired, excFired
}

// Each tap moves a chip off → selected → excluded → off, and the group
// reports both lists (#1383).
func TestChipGroup_ExcludableCycle(t *testing.T) {
	test.NewTempApp(t)
	g, sel, exc, selFired, excFired := newExcludable(t, "Rock", "Jazz", "Pop")
	chips := chipsOf(t, g)

	test.Tap(chips[0]) // Rock: off → selected
	assert.Equal(t, []string{"Rock"}, *sel)
	assert.Equal(t, 1, *selFired)
	assert.Zero(t, *excFired, "the excluded list did not change")
	assert.True(t, chips[0].Selected)

	// During the next tap both lists are already updated when OnChanged
	// fires, so an app reading the group sees one consistent state.
	var seenExcluded []string
	g.OnChanged = func(s []string) { *sel = s; *selFired++; seenExcluded = g.Excluded }
	test.Tap(chips[0]) // Rock: selected → excluded
	assert.Empty(t, *sel)
	assert.Equal(t, []string{"Rock"}, *exc)
	assert.Equal(t, []string{"Rock"}, seenExcluded, "Excluded is set before OnChanged fires")
	assert.Equal(t, 2, *selFired)
	assert.Equal(t, 1, *excFired)
	assert.False(t, chips[0].Selected)
	assert.Equal(t, "Rock, excluded", chips[0].AccessibleLabel())

	test.Tap(chips[1]) // Jazz: off → selected
	test.Tap(chips[1]) // Jazz: selected → excluded
	assert.Equal(t, []string{"Rock", "Jazz"}, g.Excluded, "in the order excluded")

	test.Tap(chips[0]) // Rock: excluded → off
	assert.Equal(t, []string{"Jazz"}, *exc)
	assert.Equal(t, 3, *excFired)
	assert.Equal(t, 4, *selFired, "the selection did not change on the last tap")
	assert.Equal(t, "Rock", chips[0].AccessibleLabel())
	assert.Equal(t, core.AccessibleState(0), chips[0].AccessibleState())
}

// The screen reader's "excluded" comes from the catalogs, so it is read in
// the user's language.
func TestChipGroup_ExcludedLabelTranslated(t *testing.T) {
	test.NewTempApp(t)
	g, _, _, _, _ := newExcludable(t, "Rock")
	lang.SetLocaleForTests(t, "de-DE")
	g.SetExcludedSilently([]string{"Rock"})
	assert.Equal(t, "Rock, ausgeschlossen", chipsOf(t, g)[0].AccessibleLabel())
}

// Both lists can be set from code without firing either callback, and an
// option is never in both (#1383).
func TestChipGroup_ExcludableSetSilently(t *testing.T) {
	test.NewTempApp(t)
	g, _, _, selFired, excFired := newExcludable(t, "Rock", "Jazz", "Pop")
	chips := chipsOf(t, g)

	g.SetSelectedSilently([]string{"Rock", "Jazz"})
	g.SetExcludedSilently([]string{"Jazz", "Pop"})
	assert.Zero(t, *selFired)
	assert.Zero(t, *excFired)
	assert.Equal(t, []string{"Rock"}, g.Selected, "excluding Jazz took it out of the selection")
	assert.Equal(t, []string{"Jazz", "Pop"}, g.Excluded)
	assert.True(t, chips[0].Selected)
	assert.Equal(t, "Jazz, excluded", chips[1].AccessibleLabel())
	assert.Equal(t, "Pop, excluded", chips[2].AccessibleLabel())

	g.SetSelectedSilently([]string{"Pop"})
	assert.Equal(t, []string{"Jazz"}, g.Excluded, "selecting Pop took it out of the exclusion")
	assert.True(t, chips[2].Selected)
	assert.Equal(t, "Pop", chips[2].AccessibleLabel())

	// Setting the fields and refreshing fires nothing either; an option in
	// both lists is drawn selected and leaves Excluded.
	g.Selected = []string{"Rock"}
	g.Excluded = []string{"Rock", "Pop"}
	g.Refresh()
	assert.Zero(t, *selFired)
	assert.Zero(t, *excFired)
	assert.Equal(t, []string{"Pop"}, g.Excluded)
	assert.True(t, chips[0].Selected)
	assert.Equal(t, "Rock", chips[0].AccessibleLabel())
}

// Each setter keeps the lists apart with the list it sets winning, even
// before the group has been drawn.
func TestChipGroup_ExcludableSettersBeforeDrawn(t *testing.T) {
	test.NewTempApp(t)
	g, _, _, _, _ := newExcludable(t, "Rock", "Jazz")
	g.SetExcludedSilently([]string{"Jazz"})
	g.SetSelectedSilently([]string{"Jazz"})
	assert.Empty(t, g.Excluded)
	g.SetExcludedSilently([]string{"Jazz"})
	assert.Empty(t, g.Selected)

	g.SetSelected([]string{"Jazz"})
	assert.Empty(t, g.Excluded)
	g.SetExcluded([]string{"Jazz"})
	assert.Empty(t, g.Selected)
}

// SetExcluded and SetSelected report, each also reporting the other list
// when it takes an option out of it.
func TestChipGroup_ExcludableSetReports(t *testing.T) {
	test.NewTempApp(t)
	g, sel, exc, selFired, excFired := newExcludable(t, "Rock", "Jazz")
	g.SetSelectedSilently([]string{"Rock", "Jazz"})

	g.SetExcluded([]string{"Jazz"})
	assert.Equal(t, []string{"Jazz"}, *exc)
	assert.Equal(t, []string{"Rock"}, *sel)
	assert.Equal(t, 1, *excFired)
	assert.Equal(t, 1, *selFired)

	g.SetSelected([]string{"Rock", "Jazz"})
	assert.Empty(t, *exc)
	assert.Equal(t, 2, *excFired)
	assert.Equal(t, 2, *selFired)
}

// An app refuses a change by putting the lists back from the callback:
// DaySong keeps the last chip from being excluded this way (#1383).
func TestChipGroup_ExcludableRefuse(t *testing.T) {
	test.NewTempApp(t)
	g, _, _, _, _ := newExcludable(t, "Rock", "Jazz")
	g.SetExcludedSilently([]string{"Jazz"})
	g.SetSelectedSilently([]string{"Rock"})
	g.OnExcludedChanged = func(e []string) {
		if len(e) == len(g.Options) {
			g.SetSelectedSilently([]string{e[len(e)-1]})
			g.SetExcludedSilently(e[:len(e)-1])
		}
	}
	chips := chipsOf(t, g)

	test.Tap(chips[0]) // Rock: selected → excluded, refused
	assert.Equal(t, []string{"Rock"}, g.Selected)
	assert.Equal(t, []string{"Jazz"}, g.Excluded)
	assert.True(t, chips[0].Selected)
	assert.Equal(t, "Rock", chips[0].AccessibleLabel())
}

// A Required group's last selected chip stays selected rather than moving
// on to excluded.
func TestChipGroup_ExcludableRequired(t *testing.T) {
	test.NewTempApp(t)
	g, _, _, selFired, excFired := newExcludable(t, "Rock", "Jazz")
	g.Required = true
	g.SetSelectedSilently([]string{"Rock"})
	chips := chipsOf(t, g)

	test.Tap(chips[0])
	assert.Equal(t, []string{"Rock"}, g.Selected)
	assert.Empty(t, g.Excluded)
	assert.Zero(t, *selFired)
	assert.Zero(t, *excFired)
}

// Without the opt-in, or in a single-choice group, a tap toggles as it did
// before, and Excluded draws nothing (#1383).
func TestChipGroup_NotExcludableUnchanged(t *testing.T) {
	test.NewTempApp(t)
	for _, tc := range []struct {
		name                 string
		multiple, excludable bool
	}{
		{"multiple", true, false},
		{"single", false, false},
		{"single, excludable", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			excFired := 0
			g := widget.NewChipGroup([]string{"Rock", "Jazz"}, tc.multiple, nil)
			g.Excludable = tc.excludable
			g.OnExcludedChanged = func([]string) { excFired++ }
			g.Excluded = []string{"Jazz"}
			g.Refresh()
			chips := chipsOf(t, g)
			assert.Equal(t, "Jazz", chips[1].AccessibleLabel(), "not drawn excluded")

			test.Tap(chips[0])
			assert.Equal(t, []string{"Rock"}, g.Selected)
			test.Tap(chips[0])
			assert.Empty(t, g.Selected, "a second tap deselects")
			assert.Equal(t, []string{"Jazz"}, g.Excluded, "untouched")
			assert.Equal(t, "Rock", chips[0].AccessibleLabel())
			assert.Zero(t, excFired)
		})
	}
}

// The three states side by side, in the light and the dark theme (#1383).
func TestChipGroup_ExcludedRenders(t *testing.T) {
	for _, v := range []struct {
		name    string
		variant bellui.ThemeVariant
	}{{"light", theme.VariantLight}, {"dark", theme.VariantDark}} {
		t.Run(v.name, func(t *testing.T) {
			a := test.NewTempApp(t)
			test.ApplyTheme(t, theme.DefaultTheme())
			a.Settings().(interface{ SetThemeVariant(bellui.ThemeVariant) }).SetThemeVariant(v.variant)

			g := widget.NewChipGroup([]string{"Rock", "Jazz", "Pop"}, true, nil)
			g.Excludable = true
			g.Selected = []string{"Rock"}
			g.Excluded = []string{"Jazz"}
			w := test.NewTempWindow(t, g)
			w.Resize(geo.NewSize(300, 60))
			test.AssertRendersToImage(t, "chip_group/excluded_"+v.name+".png", w.Canvas())

			g.Disable()
			test.AssertRendersToImage(t, "chip_group/excluded_disabled_"+v.name+".png", w.Canvas())
		})
	}
}
