package editor

import (
	"testing"

	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
)

func TestSpliceBounds(t *testing.T) {
	tests := []struct {
		name                  string
		old, new              string
		start, oldEnd, newEnd int
		changed               bool
	}{
		{"identical", "abc", "abc", 0, 0, 0, false},
		{"insert middle", "abcd", "abXcd", 2, 2, 3, true},
		{"insert at start", "abc", "Xabc", 0, 0, 1, true},
		{"insert at end", "abc", "abcX", 3, 3, 4, true},
		{"delete middle", "abXcd", "abcd", 2, 3, 2, true},
		{"replace", "abXYcd", "abZcd", 2, 4, 3, true},
		{"newline insert", "a\nb", "a\nx\nb", 2, 2, 4, true},
		{"repeated runes", "aa", "aaa", 2, 2, 3, true},
		{"empty to text", "", "ab", 0, 0, 2, true},
		{"text to empty", "ab", "", 0, 2, 0, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, oe, ne, ch := spliceBounds([]rune(tc.old), []rune(tc.new))
			if ch != tc.changed || (ch && (s != tc.start || oe != tc.oldEnd || ne != tc.newEnd)) {
				t.Errorf("spliceBounds(%q,%q) = (%d,%d,%d,%v), want (%d,%d,%d,%v)",
					tc.old, tc.new, s, oe, ne, ch, tc.start, tc.oldEnd, tc.newEnd, tc.changed)
			}
		})
	}
}

func TestShiftSpan(t *testing.T) {
	// Anchor [10, 20) in every case.
	tests := []struct {
		name             string
		es, eeOld, eeNew int
		wantS, wantE     int
		keep             bool
	}{
		{"insert before", 5, 5, 8, 13, 23, true},
		{"delete before", 5, 8, 5, 7, 17, true},
		{"insert at start boundary (no absorb)", 10, 10, 13, 13, 23, true},
		{"insert at end boundary (no extend)", 20, 20, 23, 10, 20, true},
		{"insert after", 25, 25, 28, 10, 20, true},
		{"delete after", 25, 28, 25, 10, 20, true},
		{"insert inside (resize)", 15, 15, 18, 10, 23, true},
		{"delete inside (resize)", 12, 15, 12, 10, 17, true},
		{"replace flush with start (inside)", 10, 12, 11, 10, 19, true},
		{"swallowed", 5, 25, 5, 0, 0, false},
		{"head eaten", 5, 15, 7, 7, 12, true},
		{"tail eaten", 15, 25, 15, 10, 15, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, e, keep := shiftSpan(10, 20, tc.es, tc.eeOld, tc.eeNew)
			if keep != tc.keep || (keep && (s != tc.wantS || e != tc.wantE)) {
				t.Errorf("shiftSpan(10,20,%d,%d,%d) = (%d,%d,%v), want (%d,%d,%v)",
					tc.es, tc.eeOld, tc.eeNew, s, e, keep, tc.wantS, tc.wantE, tc.keep)
			}
		})
	}
}

// TestAnchor_ProtectedFollowsInsertedLineAbove is the pre-rebrand tracker #130 headline: a line
// inserted above a protected range shifts it without a host re-publish — and
// protection ENFORCES at the shifted location.
func TestAnchor_ProtectedFollowsInsertedLineAbove(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "aaa\nSECRET\nbbb")
	e.SetProtectedRanges([]TextRange{{StartRow: 1, StartCol: 0, EndRow: 1, EndCol: 6}})

	// Insert a whole line at the top (cursor at document start).
	e.SetCursorPos(0, 0)
	e.entry.InsertText("new\n")

	want := TextRange{StartRow: 2, StartCol: 0, EndRow: 2, EndCol: 6}
	if got := e.ProtectedRanges(); len(got) != 1 || got[0] != want {
		t.Fatalf("ProtectedRanges after insert-above = %v, want [%v]", got, want)
	}

	// Enforcement moved with it: typing inside row 2 vetoes...
	e.SetCursorPos(2, 3)
	e.entry.TypedRune('x')
	if got := e.Text(); got != "new\naaa\nSECRET\nbbb" {
		t.Errorf("edit inside the shifted range should veto, text = %q", got)
	}
	// ...and the old location (row 1) is editable again.
	e.SetCursorPos(1, 1)
	e.entry.TypedRune('x')
	if got := e.Text(); got != "new\naxaa\nSECRET\nbbb" {
		t.Errorf("edit at the vacated row should apply, text = %q", got)
	}
}

func TestAnchor_ProtectedFollowsDeletedLineAbove(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "gone\nSECRET\nbbb")
	e.SetProtectedRanges([]TextRange{{StartRow: 1, StartCol: 0, EndRow: 1, EndCol: 6}})

	// Delete the first line (rows shift up) via an atomic edit.
	if !e.entry.ApplyEdits([]widget.TextEdit{{Start: 0, End: 5}}) {
		t.Fatal("delete-line ApplyEdits should apply")
	}

	want := TextRange{StartRow: 0, StartCol: 0, EndRow: 0, EndCol: 6}
	if got := e.ProtectedRanges(); len(got) != 1 || got[0] != want {
		t.Fatalf("ProtectedRanges after delete-above = %v, want [%v]", got, want)
	}
}

func TestAnchor_UndoShiftsBack(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "aaa\nSECRET")
	orig := TextRange{StartRow: 1, StartCol: 0, EndRow: 1, EndCol: 6}
	e.SetProtectedRanges([]TextRange{orig})

	e.SetCursorPos(0, 0)
	e.entry.InsertText("new\n")
	if got := e.ProtectedRanges()[0]; got.StartRow != 2 {
		t.Fatalf("precondition: range should shift to row 2, got %v", got)
	}

	// Undo replays history without BeforeChange; the diff-based tracker must
	// still see it and shift the range back.
	e.entry.Undo()
	if got := e.ProtectedRanges(); len(got) != 1 || got[0] != orig {
		t.Errorf("ProtectedRanges after undo = %v, want [%v]", got, orig)
	}
}

func TestAnchor_SameLineInsertShiftsColumns(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "ab SECRET")
	e.SetProtectedRanges([]TextRange{{StartRow: 0, StartCol: 3, EndRow: 0, EndCol: 9}})

	e.SetCursorPos(0, 2)
	e.entry.TypedRune('X') // "abX SECRET"

	want := TextRange{StartRow: 0, StartCol: 4, EndRow: 0, EndCol: 10}
	if got := e.ProtectedRanges(); len(got) != 1 || got[0] != want {
		t.Errorf("ProtectedRanges after same-line insert = %v, want [%v]", got, want)
	}
}

func TestAnchor_EditAfterRangeLeavesItAlone(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "SECRET\nbbb")
	orig := TextRange{StartRow: 0, StartCol: 0, EndRow: 0, EndCol: 6}
	e.SetProtectedRanges([]TextRange{orig})

	e.SetCursorPos(1, 3)
	e.entry.TypedRune('x')

	if got := e.ProtectedRanges(); len(got) != 1 || got[0] != orig {
		t.Errorf("ProtectedRanges after edit below = %v, want [%v]", got, orig)
	}
}

func TestAnchor_SwallowedRangeIsDropped(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "aaa\nDOOMED\nbbb")
	// A fold region (no edit veto) so the swallowing delete can actually apply.
	e.SetFoldRegions([]FoldRegion{{Range: TextRange{StartRow: 1, StartCol: 0, EndRow: 1, EndCol: 6}}})

	// Delete from mid-row0 through mid-row2, swallowing row 1 entirely.
	if !e.entry.ApplyEdits([]widget.TextEdit{{Start: 2, End: 12}}) {
		t.Fatal("swallowing delete should apply")
	}

	if got := e.FoldRegions(); len(got) != 0 {
		t.Errorf("a swallowed fold region must be dropped, got %v", got)
	}
}

func TestAnchor_RedactionFollowsInsertedLineAbove(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "aaa\nHUSH")
	e.SetRedactions([]TextRange{{StartRow: 1, StartCol: 0, EndRow: 1, EndCol: 4}})

	e.SetCursorPos(0, 0)
	e.entry.InsertText("new\n")

	// The caret-bar offsets re-derive from the shifted span: "HUSH" now starts
	// at rune offset 8 ("new\naaa\n").
	pr := e.entry.ProtectedRanges
	if len(pr) != 1 || pr[0].Start != 8 || pr[0].End != 12 {
		t.Fatalf("caret-bar ranges after insert-above = %v, want [{8 12 ...}]", pr)
	}
	// And the masked span still vetoes edits at its new home.
	e.SetCursorPos(2, 2)
	e.entry.TypedRune('x')
	if got := e.Text(); got != "new\naaa\nHUSH" {
		t.Errorf("edit inside the shifted redaction should veto, text = %q", got)
	}
}

func TestAnchor_FoldRegionFollowsAndStaysCollapsed(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "head\nfold1\nfold2\ntail")
	region := TextRange{StartRow: 1, StartCol: 0, EndRow: 2, EndCol: 5}
	e.SetFoldRegions([]FoldRegion{{Range: region}})
	e.Fold(region)
	if folded := e.FoldedRanges(); len(folded) != 1 {
		t.Fatalf("precondition: region should be folded, got %v", folded)
	}

	e.SetCursorPos(0, 0)
	e.entry.InsertText("new\n")

	regions := e.FoldRegions()
	want := TextRange{StartRow: 2, StartCol: 0, EndRow: 3, EndCol: 5}
	if len(regions) != 1 || regions[0].Range != want {
		t.Fatalf("FoldRegions after insert-above = %v, want range %v", regions, want)
	}
	if !regions[0].Collapsed {
		t.Error("the shifted region must stay collapsed")
	}
	if folded := e.FoldedRanges(); len(folded) != 1 || folded[0] != want {
		t.Errorf("FoldedRanges after shift = %v, want [%v]", folded, want)
	}
}

func TestAnchor_SetTextResetsWithoutShifting(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "aaa\nSECRET")
	orig := TextRange{StartRow: 1, StartCol: 0, EndRow: 1, EndCol: 6}
	e.SetProtectedRanges([]TextRange{orig})

	// A programmatic document swap must not be treated as an edit: ranges stay
	// as published (the host re-publishes for the new document, as before).
	e.SetText("completely\ndifferent\ncontent")
	if got := e.ProtectedRanges(); len(got) != 1 || got[0] != orig {
		t.Fatalf("SetText must not shift ranges, got %v", got)
	}

	// And the next real edit diffs against the NEW snapshot, not the old text.
	e.SetCursorPos(0, 0)
	e.entry.InsertText("x\n")
	want := TextRange{StartRow: 2, StartCol: 0, EndRow: 2, EndCol: 6}
	if got := e.ProtectedRanges(); len(got) != 1 || got[0] != want {
		t.Errorf("first edit after SetText shifted to %v, want [%v]", got, want)
	}
}
