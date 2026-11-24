package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/test"
)

// A one-line entry scrolls sideways only: a line taller than the entry, as an
// emoji still makes it, is not a reason to scroll it up and down or to show a
// vertical scroll bar (#1412). A multi-line entry still scrolls both ways.
func TestEntry_SingleLineScrollsSidewaysOnly(t *testing.T) {
	test.NewTempApp(t)
	e := NewEntry()
	e.SetText("clear 😀")
	w := test.NewTempWindow(t, e)
	w.Resize(e.MinSize().AddWidthHeight(200, 8))
	assert.Greater(t, e.text.MinSize().Height, e.scroll.Size().Height, "the emoji line is taller than the entry")
	assert.Equal(t, widget.ScrollHorizontalOnly, e.scroll.Direction)

	m := NewMultiLineEntry()
	m.SetText("one\ntwo")
	test.NewTempWindow(t, m)
	assert.Equal(t, widget.ScrollBoth, m.scroll.Direction)
}
