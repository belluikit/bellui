package widget_test

import (
	"testing"

	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
)

func TestNarrowEntryCursorPosition(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()

	entry := widget.NewEntry()
	w := test.NewWindow(entry)
	defer w.Close()
	w.Resize(geo.NewSize(40, 40))

	// Focus without selectAll by setting focused directly
	entry.FocusGained()

	for i, c := range "abcde" {
		before := entry.CursorColumn
		entry.TypedRune(c)
		after := entry.CursorColumn
		t.Logf("step %d: typed '%c', cursor before=%d after=%d, text=%q",
			i, c, before, after, entry.Text)
	}

	if entry.Text != "abcde" {
		t.Errorf("CURSOR BUG: expected 'abcde', got %q", entry.Text)
	}
}

func TestNarrowEntryCursorAfterRefresh(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()

	entry := widget.NewEntry()
	w := test.NewWindow(entry)
	defer w.Close()
	w.Resize(geo.NewSize(40, 40))

	entry.FocusGained()

	entry.TypedRune('a')
	t.Logf("after 'a': cursor row=%d col=%d, text=%q", entry.CursorRow, entry.CursorColumn, entry.Text)

	// Manually trigger refresh (simulates what happens in rendering)
	entry.Refresh()
	t.Logf("after refresh: cursor row=%d col=%d, text=%q", entry.CursorRow, entry.CursorColumn, entry.Text)

	entry.TypedRune('b')
	t.Logf("after 'b': cursor row=%d col=%d, text=%q", entry.CursorRow, entry.CursorColumn, entry.Text)

	if entry.Text != "ab" {
		t.Errorf("CURSOR BUG: expected 'ab', got %q", entry.Text)
	}
}

func TestWideEntryNoIssue(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()

	entry := widget.NewEntry()
	w := test.NewWindow(entry)
	defer w.Close()
	w.Resize(geo.NewSize(400, 40)) // wide entry

	entry.FocusGained()

	for _, c := range "abcde" {
		entry.TypedRune(c)
	}

	if entry.Text != "abcde" {
		t.Errorf("wide entry also broken: expected 'abcde', got %q", entry.Text)
	}
}
