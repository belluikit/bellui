package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/bellui/test"
)

// The read-only core.AccessibleTexter seam on selectable Label / RichText
// (#556): screen readers read the selection through it; editing is a no-op.

func newShownSelectableLabel(t *testing.T, text string) *Label {
	t.Helper()
	lang.SetLocaleForTests(t, "en-US")
	test.NewTempApp(t)
	l := NewLabel(text)
	l.Selectable = true
	l.Wrapping = bellui.TextWrapOff
	w := test.NewTempWindow(t, l)
	w.Resize(geo.NewSize(320, 120))
	return l
}

func TestLabelAccessibleTexter(t *testing.T) {
	l := newShownSelectableLabel(t, "héllo\nwörld")

	var tx core.AccessibleTexter = l
	assert.Equal(t, "héllo\nwörld", tx.AccessibleText())

	// Nothing selected yet: both offsets are the cursor.
	start, end := tx.AccessibleSelection()
	assert.Equal(t, start, end)

	tx.SetAccessibleSelection(2, 9)
	assert.Equal(t, "llo\nwör", l.SelectedText())
	start, end = tx.AccessibleSelection()
	assert.Equal(t, 2, start)
	assert.Equal(t, 9, end)
	assert.Equal(t, 9, tx.AccessibleCaret(), "caret at the selection end")

	// Reversed and out-of-range offsets normalize and clamp.
	tx.SetAccessibleSelection(100, 6)
	assert.Equal(t, "wörld", l.SelectedText())

	// Equal offsets clear the selection and park the cursor.
	tx.SetAccessibleSelection(3, 3)
	assert.Equal(t, "", l.SelectedText())
	start, end = tx.AccessibleSelection()
	assert.Equal(t, 3, start)
	assert.Equal(t, 3, end)

	// Read-only: replacing is a no-op.
	tx.SetAccessibleSelection(0, 5)
	tx.ReplaceAccessibleSelection("nope")
	assert.Equal(t, "héllo\nwörld", l.Text)
}

func TestLabelAccessibleTexter_NotSelectable(t *testing.T) {
	lang.SetLocaleForTests(t, "en-US")
	test.NewTempApp(t)
	l := NewLabel("static")
	w := test.NewTempWindow(t, l)
	w.Resize(geo.NewSize(200, 60))

	// The text is still readable; selection is inert.
	assert.Equal(t, "static", l.AccessibleText())
	l.SetAccessibleSelection(1, 4)
	start, end := l.AccessibleSelection()
	assert.Equal(t, 0, start)
	assert.Equal(t, 0, end)
	assert.Equal(t, 0, l.AccessibleCaret())
	assert.Equal(t, "", l.SelectedText())
}

func TestRichTextAccessibleTexter(t *testing.T) {
	lang.SetLocaleForTests(t, "en-US")
	test.NewTempApp(t)

	rt := NewRichTextWithText("first line\nsecond line")
	rt.Selectable = true
	rt.Wrapping = bellui.TextWrapOff
	w := test.NewTempWindow(t, rt)
	w.Resize(geo.NewSize(320, 120))

	var tx core.AccessibleTexter = rt
	assert.Equal(t, "first line\nsecond line", tx.AccessibleText())

	tx.SetAccessibleSelection(6, 17)
	assert.Equal(t, "line\nsecond", rt.SelectedText())
	start, end := tx.AccessibleSelection()
	assert.Equal(t, 6, start)
	assert.Equal(t, 17, end)
	assert.Equal(t, 17, tx.AccessibleCaret())

	tx.ReplaceAccessibleSelection("nope")
	assert.Equal(t, "first line\nsecond line", rt.String())
}
