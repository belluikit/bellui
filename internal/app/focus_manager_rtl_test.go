package app_test

import (
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/app"

	"github.com/stretchr/testify/assert"
)

func placedFocusable(x, y float32) *focusable {
	f := &focusable{}
	f.Resize(geo.NewSize(20, 20))
	f.Move(geo.NewPos(x, y))
	return f
}

func withLayoutDirection(t *testing.T, d bellui.Direction) {
	t.Helper()
	bellui.SetLayoutDirection(d)
	t.Cleanup(func() { bellui.SetLayoutDirection(bellui.DirectionAuto) })
}

func TestFocusManager_RTL_RowTraversesRightToLeft(t *testing.T) {
	left := placedFocusable(0, 0)
	mid := placedFocusable(30, 0)
	right := placedFocusable(60, 0)
	content := container.NewWithoutLayout(left, mid, right)
	content.Resize(geo.NewSize(100, 20))

	withLayoutDirection(t, bellui.DirectionRTL)
	m := app.NewFocusManager(content)

	// Tab advances right-to-left to follow the visual reading order.
	m.FocusNext()
	assert.Equal(t, right, m.Focused())
	m.FocusNext()
	assert.Equal(t, mid, m.Focused())
	m.FocusNext()
	assert.Equal(t, left, m.Focused())
	m.FocusNext()
	assert.Equal(t, right, m.Focused(), "wraps back to the rightmost")

	// Shift+Tab mirrors that.
	m.FocusPrevious()
	assert.Equal(t, left, m.Focused())
}

func TestFocusManager_LTR_RowUnchanged(t *testing.T) {
	left := placedFocusable(0, 0)
	mid := placedFocusable(30, 0)
	right := placedFocusable(60, 0)
	content := container.NewWithoutLayout(left, mid, right)
	content.Resize(geo.NewSize(100, 20))

	withLayoutDirection(t, bellui.DirectionLTR)
	m := app.NewFocusManager(content)

	m.FocusNext()
	assert.Equal(t, left, m.Focused())
	m.FocusNext()
	assert.Equal(t, mid, m.Focused())
	m.FocusNext()
	assert.Equal(t, right, m.Focused())
}

func TestFocusManager_RTL_GridRowsTopToBottomColumnsRightToLeft(t *testing.T) {
	a := placedFocusable(0, 0)   // top-left
	b := placedFocusable(30, 0)  // top-right
	c := placedFocusable(0, 30)  // bottom-left
	d := placedFocusable(30, 30) // bottom-right
	content := container.NewWithoutLayout(a, b, c, d)
	content.Resize(geo.NewSize(60, 60))

	withLayoutDirection(t, bellui.DirectionRTL)
	m := app.NewFocusManager(content)

	// Rows stay top-to-bottom; within each row the order flips right-to-left:
	// b, a, d, c.
	for _, want := range []*focusable{b, a, d, c} {
		m.FocusNext()
		assert.Equal(t, want, m.Focused())
	}
}

func TestFocusManager_RTL_ColumnUnchanged(t *testing.T) {
	top := placedFocusable(0, 0)
	mid := placedFocusable(0, 30)
	bottom := placedFocusable(0, 60)
	content := container.NewWithoutLayout(top, mid, bottom)
	content.Resize(geo.NewSize(20, 90))

	withLayoutDirection(t, bellui.DirectionRTL)
	m := app.NewFocusManager(content)

	// A vertical column has no rows to reverse, so order is preserved.
	m.FocusNext()
	assert.Equal(t, top, m.Focused())
	m.FocusNext()
	assert.Equal(t, mid, m.Focused())
	m.FocusNext()
	assert.Equal(t, bottom, m.Focused())
}
