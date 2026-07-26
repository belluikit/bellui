package editor

import (
	"testing"

	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// TestRevealRange_SelectsAndMovesCursor verifies RevealRange selects the range
// and leaves the cursor at its end.
func TestRevealRange_SelectsAndMovesCursor(t *testing.T) {
	test.NewTempApp(t)
	e := NewEditor()
	w := test.NewWindow(e)
	defer w.Close()
	w.Resize(geo.NewSize(400, 300))
	e.SetText("abc\ndefg\nhij")

	e.RevealRange(TextRange{StartRow: 1, StartCol: 1, EndRow: 1, EndCol: 4})

	if got := e.SelectedText(); got != "efg" {
		t.Fatalf("SelectedText = %q, want %q", got, "efg")
	}
	if row, col := e.CursorPos(); row != 1 || col != 4 {
		t.Errorf("cursor at (%d,%d), want (1,4)", row, col)
	}
}

// TestRevealRange_PointRange verifies a zero-width range places the cursor with
// no selection — the go-to-definition caret-placement case.
func TestRevealRange_PointRange(t *testing.T) {
	test.NewTempApp(t)
	e := NewEditor()
	w := test.NewWindow(e)
	defer w.Close()
	w.Resize(geo.NewSize(400, 300))
	e.SetText("abc\ndefg\nhij")

	e.RevealRange(TextRange{StartRow: 2, StartCol: 2, EndRow: 2, EndCol: 2})

	if got := e.SelectedText(); got != "" {
		t.Errorf("a point range should select nothing, got %q", got)
	}
	if row, col := e.CursorPos(); row != 2 || col != 2 {
		t.Errorf("cursor at (%d,%d), want (2,2)", row, col)
	}
}
