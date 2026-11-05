package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
)

// settledCanvas is a canvas over content whose min-size walk has settled, so
// the next pass walks only what changed.
func settledCanvas(t *testing.T, content core.CanvasObject) *Canvas {
	t.Helper()
	content.Resize(geo.NewSize(360, 200))
	c := &Canvas{}
	impl := &markingCanvas{sizedCanvas: sizedCanvas{Canvas: test.NewCanvas()}, c: c}
	impl.SetContent(content)
	c.Initialize(impl, func() {})
	c.SetContentTreeAndFocusMgr(content)
	for range 3 { // the first passes measure everything; then only changes
		c.EnsureMinSize()
	}
	require.Equal(t, 1, c.MinSizeVisits(), "the walk has not settled")
	return c
}

// TestWidgetShownLaterIsLaidOut is player#278: a widget hidden before any walk
// has no node, so its own refresh marks nothing. Shown later, it must still be
// laid out by the next pass, not by the full walk up to fullMinSizeEvery later.
// Until then it sits at size 0, and a button's centred label hangs off its
// left edge at a negative offset.
func TestWidgetShownLaterIsLaidOut(t *testing.T) {
	test.NewTempApp(t)
	newer, older := widget.NewButton("Newer", nil), widget.NewButton("Older", nil)
	newer.Hide()
	older.Hide()
	c := settledCanvas(t, container.NewVBox(container.NewHBox(newer, older), widget.NewLabel("list")))

	older.Show()
	c.EnsureMinSize()
	assert.Equal(t, older.MinSize(), older.Size(), "Older, shown, was not laid out")
	assert.Equal(t, geo.Position{}, older.Position(), "Older does not start the row")
}

// TestWidgetHiddenLaterGivesUpItsSpace is the other direction: a sibling of a
// widget hidden later moves into the space it left.
func TestWidgetHiddenLaterGivesUpItsSpace(t *testing.T) {
	test.NewTempApp(t)
	newer, older := widget.NewButton("Newer", nil), widget.NewButton("Older", nil)
	c := settledCanvas(t, container.NewVBox(container.NewHBox(newer, older), widget.NewLabel("list")))
	require.Greater(t, older.Position().X, float32(0))

	newer.Hide()
	c.EnsureMinSize()
	assert.Equal(t, geo.Position{}, older.Position(), "Older did not move into the space Newer left")
}
