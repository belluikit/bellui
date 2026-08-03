package widget_test

import (
	"testing"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"

	"github.com/stretchr/testify/assert"
)

func TestSegmentedButton_New(t *testing.T) {
	test.NewTempApp(t)
	sb := widget.NewSegmentedButton([]string{"A", "B", "C"}, nil)
	assert.NotNil(t, sb)
	assert.Len(t, sb.Items, 3)
	assert.Equal(t, 0, sb.Selected)
}

func TestSegmentedButton_MinSize(t *testing.T) {
	test.NewTempApp(t)
	sb := widget.NewSegmentedButton([]string{"One", "Two"}, nil)
	min := sb.MinSize()
	assert.Greater(t, min.Width, float32(0))
	assert.GreaterOrEqual(t, min.Height, float32(40))
}

func TestSegmentedButton_SelectedChange(t *testing.T) {
	test.NewTempApp(t)
	var selected int
	sb := widget.NewSegmentedButton([]string{"X", "Y", "Z"}, func(idx int) { selected = idx })

	assert.Equal(t, 0, sb.Selected)

	sb.OnChanged(2)
	assert.Equal(t, 2, selected)
}

func TestSegmentedButton_Renderer(t *testing.T) {
	test.NewTempApp(t)
	sb := widget.NewSegmentedButton([]string{"A", "B"}, nil)
	assert.NotPanics(t, func() { test.TempWidgetRenderer(t, sb) })
}

func TestSegmentedButton_SegmentTap(t *testing.T) {
	test.NewTempApp(t)
	var got int
	calls := 0
	sb := widget.NewSegmentedButton([]string{"A", "B", "C"}, func(idx int) {
		got = idx
		calls++
	})
	test.NewTempWindow(t, sb)

	// Tapping the third segment must update Selected and fire OnChanged — unlike
	// calling OnChanged directly, which leaves Selected stale.
	segments := test.TempWidgetRenderer(t, sb).Objects()
	test.Tap(segments[2].(event.Tappable))

	assert.Equal(t, 2, sb.Selected)
	assert.Equal(t, 2, got)
	assert.Equal(t, 1, calls)
}

// Items is a public slice, so a host can replace, append to or truncate it
// after the renderer exists. Refresh must re-sync the drawn segments with the
// live slice (#203, #234) — cycleSelection already walked it, so before the
// re-sync the arrow keys could select a segment the renderer never drew.
func TestSegmentedButton_RefreshResyncsItems(t *testing.T) {
	test.NewTempApp(t)
	lang.SetLocaleForTests(t, "en-US")
	var got int
	sb := widget.NewSegmentedButton([]string{"A", "B"}, func(idx int) { got = idx })
	test.NewTempWindow(t, sb)
	r := test.TempWidgetRenderer(t, sb)
	assert.Len(t, r.Objects(), 2)

	// Append, then arrow onto the new segment: it must be drawn and selectable
	// by keyboard and tap alike.
	sb.Items = append(sb.Items, "C")
	sb.Refresh()
	assert.Len(t, r.Objects(), 3)

	sb.Selected = 1
	sb.TypedKey(&event.KeyEvent{Name: event.KeyRight})
	assert.Equal(t, 2, sb.Selected)
	assert.Equal(t, 2, got)
	test.Tap(r.Objects()[2].(event.Tappable))
	assert.Equal(t, 2, sb.Selected)

	// Replacement re-syncs the surviving segments' text.
	sb.Items = []string{"X", "Y"}
	sb.Selected = 0
	sb.Refresh()
	assert.Len(t, r.Objects(), 2)
	assert.Equal(t, "X", r.Objects()[0].(core.Accessible).AccessibleLabel())
	assert.Equal(t, "Y", r.Objects()[1].(core.Accessible).AccessibleLabel())

	// Truncate — the removed segment must not linger, and emptying must not
	// break layout.
	sb.Items = sb.Items[:1]
	sb.Refresh()
	assert.Len(t, r.Objects(), 1)

	sb.Items = nil
	sb.Refresh()
	assert.Empty(t, r.Objects())
}
