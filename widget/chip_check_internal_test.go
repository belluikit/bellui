package widget

import (
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A selected filter chip shows a check mark at its leading edge, as MD3's
// does (#1364); unselected, it doesn't, and it grows by one icon and its gap
// for the mark.
func TestFilterChip_CheckMarkWhileSelected(t *testing.T) {
	test.NewTempApp(t)
	c := NewFilterChip("Pop", nil)
	r := test.TempWidgetRenderer(t, c).(*chipRenderer)
	require.NotNil(t, r.check)
	assert.False(t, r.check.Visible(), "unselected: no check mark")
	unselected := c.MinSize()

	test.Tap(c)
	assert.True(t, c.Selected)
	assert.True(t, r.check.Visible(), "selected: a check mark")
	iconSz := c.Theme().Size(theme.SizeNameIconSize) * 0.75
	assert.InDelta(t, unselected.Width+iconSz+8, c.MinSize().Width, 0.01, "the chip widens by the mark and its gap")

	c.Resize(c.MinSize())
	r.Layout(c.Size())
	assert.Less(t, r.check.Position().X, r.label.Position().X, "the mark leads the label")

	test.Tap(c)
	assert.False(t, r.check.Visible(), "deselected: the mark goes")
	assert.InDelta(t, unselected.Width, c.MinSize().Width, 0.01)
}

// With an icon of its own, the mark takes the icon's place: the chip keeps
// its width, and the icon comes back when the chip is deselected.
func TestFilterChip_CheckMarkReplacesTheIcon(t *testing.T) {
	test.NewTempApp(t)
	c := NewFilterChip("Pop", nil)
	c.Icon = theme.HomeIcon()
	r := test.TempWidgetRenderer(t, c).(*chipRenderer)
	require.NotNil(t, r.icon)
	before := c.MinSize()

	test.Tap(c)
	assert.True(t, r.check.Visible())
	assert.False(t, r.icon.Visible(), "the icon gives way to the mark")
	assert.InDelta(t, before.Width, c.MinSize().Width, 0.01, "no wider: the mark is in the icon's place")

	test.Tap(c)
	assert.True(t, r.icon.Visible())
	assert.False(t, r.check.Visible())
}

// Disabled, a selected chip keeps its mark, drawn in the disabled colour.
func TestFilterChip_CheckMarkDisabled(t *testing.T) {
	test.NewTempApp(t)
	c := NewFilterChip("Pop", nil)
	c.Selected = true
	c.Disable()
	r := test.TempWidgetRenderer(t, c).(*chipRenderer)
	r.Refresh()
	assert.True(t, r.check.Visible())
	assert.Equal(t, theme.NewDisabledResource(theme.ConfirmIcon()).Name(), r.check.Resource.Name())
}

// Under RTL the mark leads from the right, as the rest of the row mirrors.
func TestFilterChip_CheckMarkRTL(t *testing.T) {
	test.NewTempApp(t)
	bellui.SetLayoutDirection(bellui.DirectionRTL)
	t.Cleanup(func() { bellui.SetLayoutDirection(bellui.DirectionAuto) })
	c := NewFilterChip("Pop", nil)
	c.Selected = true
	w := test.NewWindow(c)
	defer w.Close()
	c.Resize(c.MinSize())
	r := test.TempWidgetRenderer(t, c).(*chipRenderer)
	r.Refresh()
	inDirectionOf(c.super(), func() { r.Layout(c.Size()) })
	assert.Greater(t, r.check.Position().X, r.label.Position().X, "under RTL the mark is right of the label")
}

// Only filter chips carry a mark: an assist chip has none to show.
func TestChip_NoCheckMarkUnlessFilter(t *testing.T) {
	test.NewTempApp(t)
	c := NewChip("Help", nil)
	c.Selected = true
	r := test.TempWidgetRenderer(t, c).(*chipRenderer)
	assert.Nil(t, r.check)
}

// In a chip group, selecting a chip widens it by its mark, and the group lays
// its chips out again: the next chip moves along by the mark and its gap.
func TestChipGroup_RelaysOutForTheCheckMark(t *testing.T) {
	test.NewTempApp(t)
	g := NewChipGroup([]string{"Pop", "Jazz"}, true, nil)
	w := test.NewWindow(g)
	defer w.Close()
	w.Resize(geo.NewSize(400, 200))
	require.Len(t, g.chips, 2)
	before := g.chips[1].Position().X

	g.SetSelected([]string{"Pop"})
	iconSz := g.chips[0].Theme().Size(theme.SizeNameIconSize) * 0.75
	assert.InDelta(t, before+iconSz+8, g.chips[1].Position().X, 0.01, "the second chip moved along by the first one's mark")
	assert.True(t, test.TempWidgetRenderer(t, g.chips[0]).(*chipRenderer).check.Visible())
}

// An excluded chip shows a cross in the check mark's place and strikes its
// label through, so the state does not rest on colour; it widens for the
// cross as for the mark (#1383).
func TestFilterChip_CrossWhileExcluded(t *testing.T) {
	test.NewTempApp(t)
	c := NewFilterChip("Pop", nil)
	c.Icon = theme.HomeIcon()
	r := test.TempWidgetRenderer(t, c).(*chipRenderer)
	require.NotNil(t, r.cross)
	assert.False(t, r.cross.Visible())
	assert.False(t, r.label.TextStyle.Strikethrough)
	withIcon := c.MinSize()

	c.excluded = true
	c.Refresh()
	assert.True(t, r.cross.Visible(), "excluded: a cross")
	assert.False(t, r.check.Visible())
	assert.False(t, r.icon.Visible(), "the icon gives way to the cross")
	assert.True(t, r.label.TextStyle.Strikethrough, "the label is struck through")
	assert.InDelta(t, withIcon.Width, c.MinSize().Width, 0.01)

	// Disabled, the cross and the strike stay: only the colours dim.
	c.Disable()
	assert.True(t, r.cross.Visible())
	assert.True(t, r.label.TextStyle.Strikethrough)
	c.Enable()

	// Selected wins over excluded.
	c.Selected = true
	c.Refresh()
	assert.False(t, r.cross.Visible())
	assert.True(t, r.check.Visible())
	assert.False(t, r.label.TextStyle.Strikethrough)

	c.Selected, c.excluded, c.Icon = false, true, nil
	plain := NewFilterChip("Pop", nil)
	c.Refresh()
	iconSz := c.Theme().Size(theme.SizeNameIconSize) * 0.75
	assert.InDelta(t, plain.MinSize().Width+iconSz+8, c.MinSize().Width, 0.01, "the chip widens by the cross and its gap")
	c.Resize(c.MinSize())
	r.Layout(c.Size())
	assert.Less(t, r.cross.Position().X, r.label.Position().X, "the cross leads the label")
}
