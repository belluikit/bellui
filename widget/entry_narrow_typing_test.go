package widget_test

import (
	"testing"

	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
)

// TestNarrowEntryCharacterOrder tests that typing into a narrow Entry
// produces characters in the correct order.
func TestNarrowEntryCharacterOrder(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()

	entry := widget.NewEntry()
	w := test.NewWindow(entry)
	defer w.Close()

	// Make the entry very narrow (like it was in the HBox layout)
	w.Resize(geo.NewSize(60, 40))

	// Type "hello" character by character
	test.Type(entry, "hello")

	if entry.Text != "hello" {
		t.Errorf("expected 'hello', got %q", entry.Text)
	}

	// Clear and try again with a longer string
	entry.SetText("")
	test.Type(entry, "abcdefghij")

	if entry.Text != "abcdefghij" {
		t.Errorf("expected 'abcdefghij', got %q", entry.Text)
	}
}

// TestNarrowEntryClickThenType tests clicking in a narrow Entry then typing.
// Note: FocusGained on a single-line Entry calls selectAll(), so typing
// replaces the selected text. This is correct Bellui behavior.
func TestNarrowEntryClickThenType(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()

	entry := widget.NewEntry()
	entry.SetText("ABC")
	w := test.NewWindow(entry)
	defer w.Close()

	w.Resize(geo.NewSize(30, 40))

	// test.Type calls FocusGained which selectAll's "ABC",
	// then TypedRune('X') replaces the selection with "X"
	test.Type(entry, "X")

	// Expected: "X" (selection was replaced)
	if entry.Text != "X" {
		t.Errorf("expected 'X' (replaced selection), got %q", entry.Text)
	}

	// Now type more — should append correctly
	entry.TypedRune('Y')
	entry.TypedRune('Z')
	if entry.Text != "XYZ" {
		t.Errorf("expected 'XYZ', got %q", entry.Text)
	}
}

// TestNarrowEntryRapidTyping tests rapid typing into a narrow entry
// to check for race conditions or ordering issues.
func TestNarrowEntryRapidTyping(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()

	entry := widget.NewEntry()
	w := test.NewWindow(entry)
	defer w.Close()

	w.Resize(geo.NewSize(40, 40))

	// Type each character individually (simulates fast typing)
	chars := "0123456789"
	for _, c := range chars {
		entry.TypedRune(c)
	}

	if entry.Text != chars {
		t.Errorf("expected %q, got %q", chars, entry.Text)
	}
}
