package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/mobile"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
)

// partShape is where a press shows and how each corner is rounded: top-left,
// top-right, bottom-right, bottom-left.
type partShape struct {
	pos     geo.Position
	size    geo.Size
	corners [4]float32
}

// pressedPart presses target at at and returns the shape its pressed state
// takes (#1156).
func pressedPart(t *testing.T, target pressEvents, t0 *widget.PressTarget, at geo.Position) partShape {
	t.Helper()
	target.TouchDown(&mobile.TouchEvent{PointEvent: event.PointEvent{Position: at}})
	defer target.TouchCancel(nil)
	require.True(t, widget.PressFeedbackOf(t0).Pressed(), "no pressed state")
	r := widget.PressFeedbackOf(t0).Ripple
	assert.Zero(t, r.CornerRadius, "a part's shape is set per corner")
	return partShape{r.Position(), r.Size(), [4]float32{
		r.TopLeftCornerRadius, r.TopRightCornerRadius,
		r.BottomRightCornerRadius, r.BottomLeftCornerRadius,
	}}
}

// Each segment's pressed state takes the segment's own shape: the end ones
// rounded on their outer side only, the middle one square, so nothing spills
// past the outline and no gap shows at a seam (#1156).
func TestSegment_PressFeedback(t *testing.T) {
	test.NewTempApp(t)
	b := NewSegmentedButton([]string{"Day", "Week", "Month"}, func(int) {})
	b.Resize(b.MinSize())
	segs := cache.Renderer(b).(*segButtonRenderer).segments
	require.Len(t, segs, 3)
	pill := test.Theme().Size(theme.SizeNameButtonMinHeight) / 2

	first, middle, last := segs[0], segs[1], segs[2]
	assertPressCycle(t, first, &first.pressTarget, first.HoverOut)
	assertTapFromCode(t, &first.pressTarget, func() { first.PerformAccessibleAction(core.ActionSelect) })

	got := pressedPart(t, first, &first.pressTarget, geo.NewPos(4, 4))
	assert.Equal(t, partShape{geo.Position{}, first.Size(), [4]float32{pill, 0, 0, pill}}, got, "first segment")
	got = pressedPart(t, middle, &middle.pressTarget, geo.NewPos(4, 4))
	assert.Equal(t, partShape{geo.Position{}, middle.Size(), [4]float32{}}, got, "middle segment")
	got = pressedPart(t, last, &last.pressTarget, geo.NewPos(4, 4))
	assert.Equal(t, partShape{geo.Position{}, last.Size(), [4]float32{0, pill, pill, 0}}, got, "last segment")

	// Under RTL the first segment sits on the right, rounded there.
	lang.SetLocaleForTests(t, "ar")
	first.Refresh()
	got = pressedPart(t, first, &first.pressTarget, geo.NewPos(4, 4))
	assert.Equal(t, [4]float32{0, pill, pill, 0}, got.corners, "first segment under RTL")
}

// A split button's two halves each show their own pressed state, rounded on
// the outer side only (#1156).
func TestSplitButton_PressFeedback(t *testing.T) {
	test.NewTempApp(t)
	s := NewSplitButton("Save", func() {}, nil)
	s.Resize(geo.NewSize(160, 40))
	assertPressCycle(t, s, &s.pressTarget, s.HoverOut)
	assertTapFromCode(t, &s.pressTarget, func() { s.PerformAccessibleAction(core.ActionPress) })
	const pill = 20

	got := pressedPart(t, s, &s.pressTarget, geo.NewPos(10, 20))
	assert.Equal(t, partShape{geo.Position{}, geo.NewSize(124, 40), [4]float32{pill, 0, 0, pill}}, got, "primary half")
	got = pressedPart(t, s, &s.pressTarget, geo.NewPos(150, 20))
	assert.Equal(t, partShape{geo.NewPos(124, 0), geo.NewSize(36, 40), [4]float32{0, pill, pill, 0}}, got, "dropdown half")

	// Under RTL the dropdown half is at the left.
	lang.SetLocaleForTests(t, "ar")
	s.Refresh()
	got = pressedPart(t, s, &s.pressTarget, geo.NewPos(10, 20))
	assert.Equal(t, partShape{geo.Position{}, geo.NewSize(36, 40), [4]float32{pill, 0, 0, pill}}, got, "dropdown half under RTL")
	got = pressedPart(t, s, &s.pressTarget, geo.NewPos(150, 20))
	assert.Equal(t, partShape{geo.NewPos(36, 0), geo.NewSize(124, 40), [4]float32{0, pill, pill, 0}}, got, "primary half under RTL")

	// The ripple is not mirrored with the rest: a layout keeps it on the half.
	s.TouchDown(&mobile.TouchEvent{PointEvent: event.PointEvent{Position: geo.NewPos(150, 20)}})
	cache.Renderer(s).Layout(s.Size())
	assert.Equal(t, geo.NewPos(36, 0), widget.PressFeedbackOf(&s.pressTarget).Ripple.Position(), "the layout mirrored the ripple")
	s.TouchCancel(nil)

	s.Disable()
	s.TouchDown(&mobile.TouchEvent{})
	assert.False(t, widget.PressFeedbackOf(&s.pressTarget).Pressed(), "a disabled split button showed a press")
}
