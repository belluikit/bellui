package widget

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
)

func TestStatusBar_Creation(t *testing.T) {
	sb := NewStatusBar(
		NewStatusBarLabel("Ready"),
	)
	assert.NotNil(t, sb)
	assert.Len(t, sb.Items, 1)
}

func TestStatusBar_MinSize(t *testing.T) {
	sb := NewStatusBar(NewStatusBarLabel("Status"))
	w := test.NewTempWindow(t, sb)
	_ = w

	min := sb.MinSize()
	assert.Greater(t, min.Width, float32(0))
	assert.Greater(t, min.Height, float32(0))
}

func TestStatusBar_MultipleItems(t *testing.T) {
	sb := NewStatusBar(
		NewStatusBarLabel("Left"),
		NewStatusBarSpacer(),
		NewStatusBarLabel("Right"),
	)
	w := test.NewTempWindow(t, sb)
	_ = w

	assert.Len(t, sb.Items, 3)
	min := sb.MinSize()
	assert.Greater(t, min.Width, float32(0))
}

func TestStatusBar_WithSeparator(t *testing.T) {
	sb := NewStatusBar(
		NewStatusBarLabel("Section 1"),
		NewStatusBarSeparator(),
		NewStatusBarLabel("Section 2"),
	)
	w := test.NewTempWindow(t, sb)
	_ = w

	assert.Len(t, sb.Items, 3)
}

func TestStatusBar_Append(t *testing.T) {
	sb := NewStatusBar(NewStatusBarLabel("First"))
	w := test.NewTempWindow(t, sb)
	_ = w

	sb.Append(NewStatusBarLabel("Second"))
	assert.Len(t, sb.Items, 2)
}

func TestStatusBar_SetItems(t *testing.T) {
	sb := NewStatusBar(NewStatusBarLabel("Old"))
	w := test.NewTempWindow(t, sb)
	_ = w

	sb.SetItems(NewStatusBarLabel("New1"), NewStatusBarLabel("New2"))
	assert.Len(t, sb.Items, 2)
}

func TestStatusBarLabel_SetText(t *testing.T) {
	label := NewStatusBarLabel("Initial")
	sb := NewStatusBar(label)
	w := test.NewTempWindow(t, sb)
	_ = w

	label.SetText("Updated")
	assert.Equal(t, "Updated", label.Text)
	assert.Equal(t, "Updated", label.label.Text)
}

func TestStatusBarLabel_WithIcon(t *testing.T) {
	label := NewStatusBarLabelWithIcon("Status", theme.InfoIcon())
	assert.NotNil(t, label.Icon)

	obj := label.StatusBarObject()
	assert.NotNil(t, obj)
}

func TestStatusBar_Empty(t *testing.T) {
	sb := NewStatusBar()
	w := test.NewTempWindow(t, sb)
	_ = w

	min := sb.MinSize()
	assert.Greater(t, min.Height, float32(0), "Empty status bar should still have separator height")
}

func TestStatusBar_LongTextDoesNotGrowMinSize(t *testing.T) {
	label := NewStatusBarLabel("Ready")
	sb := NewStatusBar(label)
	w := test.NewTempWindow(t, sb)
	_ = w

	before := sb.MinSize()
	label.SetText(strings.Repeat("error: /a/very/long/path/into/the/project ", 10))
	after := sb.MinSize()

	assert.Equal(t, before, after, "a long status message must not grow the bar's min size (#1041)")
}

func TestStatusBar_NoTruncateTracksText(t *testing.T) {
	label := &StatusBarLabel{Text: "Ready", NoTruncate: true}
	sb := NewStatusBar(label)
	w := test.NewTempWindow(t, sb)
	_ = w

	before := sb.MinSize()
	label.SetText(strings.Repeat("wide ", 40))
	after := sb.MinSize()

	assert.Greater(t, after.Width, before.Width, "NoTruncate restores min size tracking the text")
}

func TestStatusBar_LabelNaturalWidthWhenRoomAllows(t *testing.T) {
	label := NewStatusBarLabel("All systems nominal")
	sb := NewStatusBar(label)
	w := test.NewTempWindow(t, sb)
	_ = w

	sb.Resize(geo.NewSize(500, sb.MinSize().Height))

	assert.InDelta(t, label.label.IntrinsicSize().Width, label.label.Size().Width, 0.5,
		"with room to spare the label renders at its natural text width")
}

func TestStatusBar_LabelShrinksWhenTight(t *testing.T) {
	label := NewStatusBarLabel(strings.Repeat("shrink me ", 20))
	sb := NewStatusBar(label)
	w := test.NewTempWindow(t, sb)
	_ = w

	sb.Resize(geo.NewSize(150, sb.MinSize().Height))

	assert.Less(t, label.label.Size().Width, label.label.IntrinsicSize().Width)
	assert.LessOrEqual(t, label.label.Size().Width, float32(150))
}

func TestStatusBar_SpacerStillExpands(t *testing.T) {
	left := NewStatusBarLabel("left")
	right := NewStatusBarLabel("right")
	sb := NewStatusBar(left, NewStatusBarSpacer(), right)
	w := test.NewTempWindow(t, sb)
	_ = w

	const width = 400
	sb.Resize(geo.NewSize(width, sb.MinSize().Height))

	rightEdge := right.label.Position().X + right.label.Size().Width
	assert.InDelta(t, width, rightEdge, 1, "spacer pushes the trailing section to the far edge")
	assert.InDelta(t, left.label.IntrinsicSize().Width, left.label.Size().Width, 0.5)
	assert.InDelta(t, right.label.IntrinsicSize().Width, right.label.Size().Width, 0.5)
}

func TestStatusBar_IconSectionFlexes(t *testing.T) {
	label := NewStatusBarLabelWithIcon(strings.Repeat("with icon and a rather long message ", 10), theme.InfoIcon())
	sb := NewStatusBar(label)
	w := test.NewTempWindow(t, sb)
	_ = w

	before := sb.MinSize()
	label.SetText(strings.Repeat("even longer ", 40))
	assert.Equal(t, before, sb.MinSize(), "icon sections truncate too")

	sb.Resize(geo.NewSize(160, sb.MinSize().Height))
	assert.Less(t, label.label.Size().Width, label.label.IntrinsicSize().Width)
	assert.Greater(t, label.icon.Size().Width, float32(0))
}

func TestStatusBar_SetTextRedistributesWithinBar(t *testing.T) {
	label := NewStatusBarLabel("Ready")
	sb := NewStatusBar(label)
	w := test.NewTempWindow(t, sb)
	_ = w

	sb.Resize(geo.NewSize(300, sb.MinSize().Height))
	shortWidth := label.label.Size().Width
	before := sb.MinSize()

	label.SetText("error: /a/much/longer/path/that/should/use/the/slack")
	assert.Equal(t, before, sb.MinSize(), "text change never alters the bar's min size")
	assert.Greater(t, label.label.Size().Width, shortWidth,
		"the section grows into the bar's slack on SetText without an ancestor relayout")

	label.SetText("Ready")
	assert.InDelta(t, shortWidth, label.label.Size().Width, 0.5, "and shrinks back")
}

func TestStatusBar_LongSectionAbsorbsShortage(t *testing.T) {
	long := NewStatusBarLabel(strings.Repeat("a long diagnostic message ", 10))
	short := NewStatusBarLabel("ln 12, col 8")
	sb := NewStatusBar(long, NewStatusBarSpacer(), short)
	w := test.NewTempWindow(t, sb)
	_ = w

	sb.Resize(geo.NewSize(300, sb.MinSize().Height))

	assert.InDelta(t, short.label.IntrinsicSize().Width, short.label.Size().Width, 0.5,
		"the short section keeps its natural width")
	assert.Less(t, long.label.Size().Width, long.label.IntrinsicSize().Width,
		"the long section absorbs the shortage")
	assert.Greater(t, long.label.Size().Width, float32(150), "and gets the lion's share of the bar")
}
