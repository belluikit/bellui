package widget_test

import (
	"testing"

	"github.com/belluikit/bellui/event"
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
