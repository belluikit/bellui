package widget_test

import (
	"testing"

	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
)

func TestEntryRowBoundary(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()

	entry := widget.NewEntry()
	w := test.NewWindow(entry)
	defer w.Close()
	w.Resize(geo.NewSize(400, 40))

	// Don't call FocusGained (which does selectAll)
	// Just directly type runes
	entry.TypedRune('a')
	t.Logf("after 'a': Text=%q, CursorRow=%d, CursorCol=%d",
		entry.Text, entry.CursorRow, entry.CursorColumn)

	pos := entry.CursorTextOffset()
	t.Logf("CursorTextOffset = %d (should be 1)", pos)

	entry.TypedRune('b')
	t.Logf("after 'b': Text=%q, CursorRow=%d, CursorCol=%d",
		entry.Text, entry.CursorRow, entry.CursorColumn)

	pos = entry.CursorTextOffset()
	t.Logf("CursorTextOffset = %d (should be 2)", pos)

	if entry.Text != "ab" {
		t.Errorf("BUG: expected 'ab', got %q", entry.Text)
	}
}

func TestEntryWithoutWindow(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()

	// Entry WITHOUT a window - no rendering at all
	entry := widget.NewEntry()

	entry.TypedRune('a')
	t.Logf("no window - after 'a': Text=%q, CursorCol=%d", entry.Text, entry.CursorColumn)

	entry.TypedRune('b')
	t.Logf("no window - after 'b': Text=%q, CursorCol=%d", entry.Text, entry.CursorColumn)

	if entry.Text != "ab" {
		t.Errorf("BUG even without window: expected 'ab', got %q", entry.Text)
	}
}
