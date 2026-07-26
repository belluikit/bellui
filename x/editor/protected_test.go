package editor

import (
	"runtime"
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// "Hello World!" rune indices:
//
//	H0 e1 l2 l3 o4 ' '5 W6 o7 r8 l9 d10 !11   (len 12)
//
// "World" occupies the rune range [6, 11), i.e. (row 0, col 6) .. (row 0, col 11).
const protSample = "Hello World!"

var worldRange = TextRange{StartRow: 0, StartCol: 6, EndRow: 0, EndCol: 11}

// newProtectedEditor builds an editor showing text inside a laid-out window so
// the entry's edit paths run as they would on screen.
func newProtectedEditor(t *testing.T, text string) *Editor {
	t.Helper()
	test.NewTempApp(t)
	e := NewEditor()
	w := test.NewWindow(e)
	t.Cleanup(w.Close)
	w.Resize(geo.NewSize(400, 300))
	e.SetText(text)
	return e
}

// protectWorld protects "World" and records every rejected range.
func protectWorld(e *Editor) *[]TextRange {
	var hits []TextRange
	e.OnProtectedEditAttempt(func(r TextRange) { hits = append(hits, r) })
	e.SetProtectedRanges([]TextRange{worldRange})
	return &hits
}

// --- every interactive edit path is rejected -----------------------------

func TestProtected_Type(t *testing.T) {
	e := newProtectedEditor(t, protSample)
	hits := protectWorld(e)

	e.entry.CursorColumn = 8 // inside "World"
	e.entry.TypedRune('X')

	if e.Text() != protSample {
		t.Errorf("typing inside a protected span must be rejected, got %q", e.Text())
	}
	if len(*hits) != 1 || (*hits)[0] != worldRange {
		t.Errorf("OnProtectedEditAttempt = %v, want one hit of %v", *hits, worldRange)
	}
}

func TestProtected_BoundaryInsertAllowed(t *testing.T) {
	e := newProtectedEditor(t, protSample)
	hits := protectWorld(e)

	e.entry.CursorColumn = 6 // exactly at the leading boundary
	e.entry.TypedRune('X')
	if e.Text() != "Hello XWorld!" {
		t.Errorf("insertion at the leading boundary should be allowed, got %q", e.Text())
	}
	// Anchor-tracking (#130): the boundary insert lands outside the span, which
	// shifts right to keep protecting "World" (it used to drift onto "XWorl").
	if got := e.ProtectedRanges(); len(got) != 1 || got[0] != (TextRange{StartRow: 0, StartCol: 7, EndRow: 0, EndCol: 12}) {
		t.Errorf("the span should shift right of the boundary insert, got %v", got)
	}

	// SetText is a document swap: ranges are left as published, so re-publish
	// for the restored document (the #130 contract).
	e.SetText(protSample)
	e.SetProtectedRanges([]TextRange{worldRange})
	e.entry.CursorColumn = 11 // exactly at the trailing boundary
	e.entry.TypedRune('Y')
	if e.Text() != "Hello WorldY!" {
		t.Errorf("insertion at the trailing boundary should be allowed, got %q", e.Text())
	}
	if len(*hits) != 0 {
		t.Errorf("boundary insertions must not fire OnProtectedEditAttempt, got %v", *hits)
	}
}

func TestProtected_Backspace(t *testing.T) {
	e := newProtectedEditor(t, protSample)
	protectWorld(e)

	e.entry.CursorColumn = 7 // backspace would delete 'W' at [6,7)
	e.entry.TypedKey(&event.KeyEvent{Name: event.KeyBackspace})
	if e.Text() != protSample {
		t.Errorf("backspace into a protected span must be rejected, got %q", e.Text())
	}

	// Backspace just before the span deletes the space and is allowed.
	e.entry.CursorColumn = 6
	e.entry.TypedKey(&event.KeyEvent{Name: event.KeyBackspace})
	if e.Text() != "HelloWorld!" {
		t.Errorf("backspace before the span should be allowed, got %q", e.Text())
	}
}

func TestProtected_Delete(t *testing.T) {
	e := newProtectedEditor(t, protSample)
	protectWorld(e)

	e.entry.CursorColumn = 6 // delete would remove 'W' at [6,7)
	e.entry.TypedKey(&event.KeyEvent{Name: event.KeyDelete})
	if e.Text() != protSample {
		t.Errorf("delete into a protected span must be rejected, got %q", e.Text())
	}
}

func TestProtected_DeleteWord(t *testing.T) {
	e := newProtectedEditor(t, protSample)
	protectWorld(e)

	mod := event.KeyModifierShortcutDefault
	if runtime.GOOS == "darwin" {
		mod = event.KeyModifierAlt
	}
	e.entry.CursorColumn = 11 // word-delete would erase "World"
	e.entry.TypedShortcut(&desktop.CustomShortcut{Modifier: mod, KeyName: event.KeyBackspace})
	if e.Text() != protSample {
		t.Errorf("word-delete overlapping a protected span must be rejected, got %q", e.Text())
	}
}

func TestProtected_Newline(t *testing.T) {
	e := newProtectedEditor(t, protSample)
	protectWorld(e)

	e.entry.CursorColumn = 8 // newline strictly inside the span
	e.entry.TypedKey(&event.KeyEvent{Name: event.KeyReturn})
	if e.Text() != protSample {
		t.Errorf("newline inside a protected span must be rejected, got %q", e.Text())
	}
}

func TestProtected_Paste(t *testing.T) {
	e := newProtectedEditor(t, protSample)
	protectWorld(e)

	clip := test.NewClipboard()
	clip.SetContent("INJECT")

	e.entry.CursorColumn = 8 // caret inside the span
	e.entry.TypedShortcut(&bellui.ShortcutPaste{Clipboard: clip})
	if e.Text() != protSample {
		t.Errorf("paste into a protected span must be rejected, got %q", e.Text())
	}

	// Paste over a selection overlapping the span is also rejected.
	e.entry.Select(0, 4, 0, 8)
	e.entry.TypedShortcut(&bellui.ShortcutPaste{Clipboard: clip})
	if e.Text() != protSample {
		t.Errorf("paste over a selection overlapping the span must be rejected, got %q", e.Text())
	}
}

func TestProtected_Cut(t *testing.T) {
	e := newProtectedEditor(t, protSample)
	protectWorld(e)

	clip := test.NewClipboard()
	e.entry.Select(0, 6, 0, 11) // select "World"
	e.entry.TypedShortcut(&bellui.ShortcutCut{Clipboard: clip})
	if e.Text() != protSample {
		t.Errorf("cutting a protected selection must not delete it, got %q", e.Text())
	}
}

func TestProtected_SelectionDelete(t *testing.T) {
	e := newProtectedEditor(t, protSample)
	protectWorld(e)

	e.entry.Select(0, 4, 0, 9) // overlaps the protected span
	e.entry.TypedKey(&event.KeyEvent{Name: event.KeyDelete})
	if e.Text() != protSample {
		t.Errorf("deleting a selection overlapping the span must be rejected, got %q", e.Text())
	}
}

func TestProtected_ProgrammaticSelectOverwrite(t *testing.T) {
	e := newProtectedEditor(t, protSample)
	protectWorld(e)

	// find-replace style: select a region overlapping the span and type over it.
	e.entry.Select(0, 0, 0, 7)
	e.entry.TypedRune('!')
	if e.Text() != protSample {
		t.Errorf("select+overwrite touching the span must be rejected, got %q", e.Text())
	}
}

func TestProtected_IMECommit(t *testing.T) {
	e := newProtectedEditor(t, protSample)
	hits := protectWorld(e)

	e.entry.CursorColumn = 8 // caret inside the span
	e.entry.ComposeCommit("文")
	if e.Text() != protSample {
		t.Errorf("IME commit inside a protected span must be rejected, got %q", e.Text())
	}
	if len(*hits) != 1 {
		t.Errorf("IME commit rejection must fire OnProtectedEditAttempt once, got %v", *hits)
	}
}

// --- edits elsewhere are unaffected --------------------------------------

func TestProtected_UnaffectedElsewhere(t *testing.T) {
	e := newProtectedEditor(t, protSample)
	hits := protectWorld(e)

	e.entry.CursorColumn = 2 // well outside the span
	e.entry.TypedRune('X')
	if e.Text() != "HeXllo World!" {
		t.Errorf("an edit outside any protected span must proceed, got %q", e.Text())
	}
	if len(*hits) != 0 {
		t.Errorf("an allowed edit must not fire OnProtectedEditAttempt, got %v", *hits)
	}
}

func TestProtected_NoRangesBehaviorUnchanged(t *testing.T) {
	e := newProtectedEditor(t, protSample)
	// No SetProtectedRanges call at all.

	e.entry.CursorColumn = 8
	e.entry.TypedRune('X')
	if e.Text() != "Hello WoXrld!" {
		t.Errorf("with no protected ranges, editing must be unrestricted, got %q", e.Text())
	}
}

func TestProtected_ClearingRestoresEditing(t *testing.T) {
	e := newProtectedEditor(t, protSample)
	protectWorld(e)
	e.SetProtectedRanges(nil) // clear

	e.entry.CursorColumn = 8
	e.entry.TypedRune('X')
	if e.Text() != "Hello WoXrld!" {
		t.Errorf("clearing protected ranges must restore editing, got %q", e.Text())
	}
	if e.ProtectedRanges() != nil {
		t.Errorf("ProtectedRanges after clear = %v, want nil", e.ProtectedRanges())
	}
}

// --- undo stays clean after a rejected edit ------------------------------

func TestProtected_UndoDoesNotResurrect(t *testing.T) {
	e := newProtectedEditor(t, protSample)

	// A legitimate edit first (no protection yet).
	e.entry.CursorColumn = 0
	e.entry.TypedRune('A') // "AHello World!"
	if e.Text() != "AHello World!" {
		t.Fatalf("setup edit failed, got %q", e.Text())
	}

	// Protect "World" (now shifted one rune right) and attempt a vetoed edit.
	e.SetProtectedRanges([]TextRange{{StartRow: 0, StartCol: 7, EndRow: 0, EndCol: 12}})
	e.entry.CursorColumn = 9
	e.entry.TypedRune('X') // rejected, never recorded
	if e.Text() != "AHello World!" {
		t.Fatalf("vetoed edit must not apply, got %q", e.Text())
	}

	// Undo unwinds only the legitimate 'A'; the vetoed edit is not on the stack.
	e.entry.Undo()
	if e.Text() != protSample {
		t.Errorf("undo must not resurrect a vetoed edit, got %q", e.Text())
	}
}

// --- accessor semantics --------------------------------------------------

func TestProtectedRanges_RoundTripAndCopy(t *testing.T) {
	e := newProtectedEditor(t, protSample)
	if got := e.ProtectedRanges(); got != nil {
		t.Errorf("ProtectedRanges with none set = %v, want nil", got)
	}

	in := []TextRange{worldRange, {StartRow: 0, StartCol: 0, EndRow: 0, EndCol: 5}}
	e.SetProtectedRanges(in)

	got := e.ProtectedRanges()
	if len(got) != 2 || got[0] != worldRange {
		t.Fatalf("ProtectedRanges round-trip = %v", got)
	}
	// Mutating the returned slice must not affect stored state.
	got[0] = TextRange{}
	if e.ProtectedRanges()[0] != worldRange {
		t.Error("ProtectedRanges must return a copy, not the backing slice")
	}
	// Mutating the input slice after setting must not affect stored state.
	in[0] = TextRange{}
	if e.ProtectedRanges()[0] != worldRange {
		t.Error("SetProtectedRanges must defensively copy its argument")
	}
}

// --- multi-line / offset conversion (pure) -------------------------------

func TestRangeToOffsets(t *testing.T) {
	const text = "ab\ncdef\nghi" // a0 b1 \n c d e f \n g h i
	tests := []struct {
		name             string
		r                TextRange
		wantStart, wantE int
	}{
		{"single line", TextRange{0, 0, 0, 2}, 0, 2},
		{"second line", TextRange{1, 1, 1, 3}, 4, 6},   // 'd'..'f' -> offsets 4..6
		{"crosses lines", TextRange{0, 1, 2, 1}, 1, 9}, // b .. g
		{"col past EOL clamps", TextRange{0, 0, 0, 99}, 0, 2},
		{"row past EOF clamps", TextRange{5, 0, 5, 0}, 11, 11},
		{"negative clamps to 0", TextRange{-1, -3, 0, 0}, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotS, gotE := rangeToOffsets(text, tt.r)
			if gotS != tt.wantStart || gotE != tt.wantE {
				t.Errorf("rangeToOffsets(%v) = (%d,%d), want (%d,%d)", tt.r, gotS, gotE, tt.wantStart, tt.wantE)
			}
		})
	}
}

func TestEditTouchesRange(t *testing.T) {
	// Protected span [6, 11).
	tests := []struct {
		name       string
		start, end int
		want       bool
	}{
		{"insertion strictly inside", 8, 8, true},
		{"insertion at start boundary", 6, 6, false},
		{"insertion at end boundary", 11, 11, false},
		{"deletion overlapping start", 5, 7, true},
		{"deletion overlapping end", 10, 12, true},
		{"deletion fully covering", 0, 12, true},
		{"deletion just before", 4, 6, false},
		{"deletion just after", 11, 12, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := editTouchesRange(tt.start, tt.end, 6, 11); got != tt.want {
				t.Errorf("editTouchesRange(%d,%d,6,11) = %v, want %v", tt.start, tt.end, got, tt.want)
			}
		})
	}

	if editTouchesRange(2, 2, 5, 5) {
		t.Error("an empty protected span must protect nothing")
	}
}

// editTouchesRangeAtomic backs masked redactions: identical to editTouchesRange
// except a boundary insertion is also a touch, so a masked span freezes at its
// edges and the secret can't be shifted out from under its bar (#108).
func TestEditTouchesRangeAtomic(t *testing.T) {
	// Masked span [6, 11).
	tests := []struct {
		name       string
		start, end int
		want       bool
	}{
		{"insertion strictly inside", 8, 8, true},
		{"insertion at start boundary", 6, 6, true}, // differs from non-atomic
		{"insertion at end boundary", 11, 11, true}, // differs from non-atomic
		{"deletion overlapping start", 5, 7, true},
		{"deletion overlapping end", 10, 12, true},
		{"deletion fully covering", 0, 12, true},
		{"deletion just before", 4, 6, false},
		{"deletion just after", 11, 12, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := editTouchesRangeAtomic(tt.start, tt.end, 6, 11); got != tt.want {
				t.Errorf("editTouchesRangeAtomic(%d,%d,6,11) = %v, want %v", tt.start, tt.end, got, tt.want)
			}
		})
	}

	if editTouchesRangeAtomic(5, 5, 5, 5) {
		t.Error("an empty masked span must protect nothing")
	}
}

// TestProtected_MultiLineRange protects a span crossing rows and verifies an
// edit inside it (on the second row) is rejected while an edit on an unprotected
// row proceeds.
func TestProtected_MultiLineRange(t *testing.T) {
	e := newProtectedEditor(t, "line one\nsecret two\nline three")
	// Protect "two" on row 1 (cols 7..10) through into nothing — just that word.
	e.SetProtectedRanges([]TextRange{{StartRow: 1, StartCol: 0, EndRow: 1, EndCol: 10}})

	e.entry.CursorRow = 1
	e.entry.CursorColumn = 3 // inside the protected row
	e.entry.TypedRune('X')
	if e.Text() != "line one\nsecret two\nline three" {
		t.Errorf("edit inside a protected multi-row span must be rejected, got %q", e.Text())
	}

	e.entry.CursorRow = 0
	e.entry.CursorColumn = 0 // unprotected row
	e.entry.TypedRune('Z')
	if e.Text() != "Zline one\nsecret two\nline three" {
		t.Errorf("edit on an unprotected row must proceed, got %q", e.Text())
	}
}
