package editor

import (
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/test"
)

// redactWorld masks "World" in protSample and returns the editor's caret-opaque
// ranges for inspection.
func redactWorld(e *Editor) {
	e.SetRedactions([]TextRange{worldRange})
}

// --- mode + accessor -----------------------------------------------------

func TestRedaction_DefaultModeMasked(t *testing.T) {
	e := newProtectedEditor(t, protSample)
	if e.RedactionMode() != Masked {
		t.Errorf("default RedactionMode = %v, want Masked", e.RedactionMode())
	}
}

func TestRedactionMode_RoundTrip(t *testing.T) {
	e := newProtectedEditor(t, protSample)
	e.SetRedactionMode(Revealed)
	if e.RedactionMode() != Revealed {
		t.Errorf("RedactionMode = %v, want Revealed", e.RedactionMode())
	}
	e.SetRedactionMode(Masked)
	if e.RedactionMode() != Masked {
		t.Errorf("RedactionMode = %v, want Masked", e.RedactionMode())
	}
}

// --- Redactions() read-back (#158) ---------------------------------------

func TestRedactions_ReadsBackTrackedSpans(t *testing.T) {
	e := newProtectedEditor(t, protSample)
	if got := e.Redactions(); got != nil {
		t.Fatalf("Redactions() with none set = %v, want nil", got)
	}

	redactWorld(e)
	got := e.Redactions()
	if len(got) != 1 || got[0] != worldRange {
		t.Fatalf("Redactions() = %v, want [%v]", got, worldRange)
	}

	// The getter must not alias the tracked slice: a caller mutating its copy
	// must not move the editor's bars.
	got[0].StartCol = 0
	if again := e.Redactions(); again[0] != worldRange {
		t.Errorf("mutating the returned copy leaked into the editor: %v", again[0])
	}

	// An insert before the span shifts it via anchor-tracking (#130); the
	// getter reports the span the editor is actually drawing, not the one the
	// host published — the exact staleness #158's reveal-toggle hits.
	e.entry.CursorColumn = 0
	e.entry.TypedRune('Z')
	want := TextRange{StartRow: 0, StartCol: 7, EndRow: 0, EndCol: 12}
	if shifted := e.Redactions(); len(shifted) != 1 || shifted[0] != want {
		t.Errorf("after an insert above, Redactions() = %v, want [%v]", shifted, want)
	}
}

// --- masked: opaque bars + caret-opaque protected ranges -----------------

func TestRedaction_MaskedDrawsOpaqueBar(t *testing.T) {
	e := newProtectedEditor(t, protSample)
	redactWorld(e)

	spans := e.redaction.Spans()
	if len(spans) != 1 {
		t.Fatalf("masked redaction should draw one bar, got %d", len(spans))
	}
	s := spans[0]
	if s.Range != worldRange || s.Kind != SpanBackground || !s.Opaque {
		t.Errorf("bar span = %+v, want opaque SpanBackground over %v", s, worldRange)
	}
}

func TestRedaction_MaskedSetsCaretOpaqueRanges(t *testing.T) {
	e := newProtectedEditor(t, protSample)
	redactWorld(e)

	prs := e.entry.ProtectedRanges
	if len(prs) != 1 {
		t.Fatalf("masked redaction should set one ProtectedRange, got %d", len(prs))
	}
	if prs[0].Start != 6 || prs[0].End != 11 || !prs[0].CaretOpaque {
		t.Errorf("ProtectedRange = %+v, want {6,11,CaretOpaque}", prs[0])
	}
}

func TestRedaction_MaskedBarsCaretEntry(t *testing.T) {
	e := newProtectedEditor(t, protSample)
	redactWorld(e)

	e.entry.CursorColumn = 6 // leading boundary
	e.entry.TypedKey(&event.KeyEvent{Name: event.KeyRight})
	if e.entry.CursorColumn != 11 {
		t.Errorf("right arrow into a masked span should jump to 11, got %d", e.entry.CursorColumn)
	}
}

func TestRedaction_MaskedEditVetoedSilently(t *testing.T) {
	e := newProtectedEditor(t, protSample)
	var hits []TextRange
	e.OnProtectedEditAttempt(func(r TextRange) { hits = append(hits, r) })
	redactWorld(e) // masked, NOT added to SetProtectedRanges

	e.entry.CursorColumn = 8 // inside the masked span
	e.entry.TypedRune('X')
	if e.Text() != protSample {
		t.Errorf("editing a masked span must be rejected, got %q", e.Text())
	}
	if len(hits) != 0 {
		t.Errorf("a masked-span edit is vetoed silently (no callback), got %v", hits)
	}
}

// --- masked spans are atomic: edge inserts can't shift the secret (#108) -

// A masked span must be atomic: the fork's CaretOpaque ProtectedRanges vetoes
// edits strictly inside but permits boundary inserts, which would shift the
// secret out from under the opaque bar one keystroke at a time. These edits
// must be vetoed too, silently, so the bar stays aligned to the secret.
func TestRedaction_MaskedBoundaryInsertVetoed(t *testing.T) {
	// worldRange masks "World" at columns [6, 11) in "Hello World!".
	cases := []struct {
		name string
		col  int
	}{
		{"start edge", 6}, // before 'W' — would become "Hello ZWorld!", revealing 'd'
		{"end edge", 11},  // after 'd' — would grow against the secret's tail
		{"strictly inside", 8},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newProtectedEditor(t, protSample)
			var hits []TextRange
			e.OnProtectedEditAttempt(func(r TextRange) { hits = append(hits, r) })
			redactWorld(e) // masked, NOT added to SetProtectedRanges

			e.entry.CursorColumn = tc.col
			e.entry.TypedRune('Z')

			if e.Text() != protSample {
				t.Errorf("inserting at col %d of a masked span must be vetoed; the secret shifted: got %q", tc.col, e.Text())
			}
			if len(hits) != 0 {
				t.Errorf("a masked-span edit is vetoed silently (no callback), got %v", hits)
			}
		})
	}
}

// Pasting (a non-empty replacement at a collapsed selection) at a masked edge
// is the same atomic violation as typing and must also be vetoed.
func TestRedaction_MaskedBoundaryPasteVetoed(t *testing.T) {
	e := newProtectedEditor(t, protSample)
	redactWorld(e)

	clip := test.NewClipboard()
	clip.SetContent("INJECT")
	e.entry.CursorColumn = 6 // start edge of the masked span
	e.entry.TypedShortcut(&bellui.ShortcutPaste{Clipboard: clip})

	if e.Text() != protSample {
		t.Errorf("pasting at a masked span edge must be vetoed, got %q", e.Text())
	}
}

// An edit OUTSIDE every masked span stays editable — atomic protection must not
// freeze the whole document.
func TestRedaction_MaskedAllowsEditsOutsideSpan(t *testing.T) {
	e := newProtectedEditor(t, protSample)
	redactWorld(e)

	e.entry.CursorColumn = 12 // after the trailing '!', clear of "World"
	e.entry.TypedRune('Z')

	if e.Text() != "Hello World!Z" {
		t.Errorf("an edit clear of every masked span must apply, got %q", e.Text())
	}
}

// --- toggle is non-mutating ----------------------------------------------

func TestRedaction_ToggleNoDocumentChangeNoOnChanged(t *testing.T) {
	e := newProtectedEditor(t, protSample)
	redactWorld(e)

	changed := 0
	e.OnChanged = func(string) { changed++ }

	e.SetRedactionMode(Revealed)
	e.SetRedactionMode(Masked)

	if e.Text() != protSample {
		t.Errorf("toggling redaction mode must not mutate the document, got %q", e.Text())
	}
	if changed != 0 {
		t.Errorf("toggling redaction mode must not fire OnChanged, fired %d times", changed)
	}
}

// --- revealed: bars + caret-bar lifted -----------------------------------

func TestRedaction_RevealedDrawsNoBarAndLiftsCaretBar(t *testing.T) {
	e := newProtectedEditor(t, protSample)
	redactWorld(e)
	e.SetRedactionMode(Revealed)

	if spans := e.redaction.Spans(); spans != nil {
		t.Errorf("revealed redaction should draw no bars, got %v", spans)
	}
	if prs := e.entry.ProtectedRanges; len(prs) != 0 {
		t.Errorf("revealing must lift the caret-bar, got %v", prs)
	}

	// Caret may now travel into the span and (absent separate protection) edit it.
	e.entry.CursorColumn = 8
	e.entry.TypedRune('X')
	if e.Text() != "Hello WoXrld!" {
		t.Errorf("a revealed, unprotected span should be editable, got %q", e.Text())
	}
}

// --- redaction and protection are orthogonal -----------------------------

func TestRedaction_RevealKeepsProtectionWhenAlsoProtected(t *testing.T) {
	e := newProtectedEditor(t, protSample)
	var hits []TextRange
	e.OnProtectedEditAttempt(func(r TextRange) { hits = append(hits, r) })

	// Mask AND protect the same span, then reveal it.
	redactWorld(e)
	e.SetProtectedRanges([]TextRange{worldRange})
	e.SetRedactionMode(Revealed)

	// Caret-bar lifted (revealed), but the BeforeChange protection still vetoes
	// edits and fires the callback.
	e.entry.CursorColumn = 8
	e.entry.TypedRune('X')
	if e.Text() != protSample {
		t.Errorf("a revealed-but-protected span must still reject edits, got %q", e.Text())
	}
	if len(hits) != 1 || hits[0] != worldRange {
		t.Errorf("protection callback should fire on the revealed span, got %v", hits)
	}
}

func TestRedaction_ClearRemovesBarsAndCaretBar(t *testing.T) {
	e := newProtectedEditor(t, protSample)
	redactWorld(e)
	e.SetRedactions(nil)

	if spans := e.redaction.Spans(); spans != nil {
		t.Errorf("cleared redactions should draw no bars, got %v", spans)
	}
	if prs := e.entry.ProtectedRanges; len(prs) != 0 {
		t.Errorf("cleared redactions should lift the caret-bar, got %v", prs)
	}
}

// --- reconciliation on text change ---------------------------------------

func TestRedaction_CaretBarTracksOffsetOnEdit(t *testing.T) {
	// Redaction on row 1; an edit on row 0 leaves its (row,col) valid but shifts
	// its flat offset. reconcile must keep the caret-bar aligned.
	e := newProtectedEditor(t, "abc\nWorld\nxyz")
	e.SetRedactions([]TextRange{{StartRow: 1, StartCol: 0, EndRow: 1, EndCol: 5}})

	if prs := e.entry.ProtectedRanges; len(prs) != 1 || prs[0].Start != 4 || prs[0].End != 9 {
		t.Fatalf("initial caret-bar = %+v, want {4,9}", e.entry.ProtectedRanges)
	}

	e.entry.CursorRow, e.entry.CursorColumn = 0, 0
	e.entry.TypedRune('X') // "Xabc\nWorld\nxyz" — row 1 begin shifts +1

	if prs := e.entry.ProtectedRanges; len(prs) != 1 || prs[0].Start != 5 || prs[0].End != 10 {
		t.Errorf("caret-bar after edit = %+v, want {5,10}", e.entry.ProtectedRanges)
	}
}
