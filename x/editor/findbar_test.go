package editor

import (
	"testing"

	"github.com/belluikit/bellui/test"
)

func TestFindBarUpdateMatches(t *testing.T) {
	test.NewTempApp(t)
	var selected [4]int
	var reported []TextRange
	var reportedActive int
	fb := newFindBar(
		func() string { return "hello world\nhello again\ngoodbye" },
		func(sr, sc, er, ec int) { selected = [4]int{sr, sc, er, ec} },
		func() {},
		func(m []TextRange, active int) { reported, reportedActive = m, active },
	)

	// Set query text directly on the internal field (avoids Entry.SetText
	// which requires a running Bellui app).
	fb.entry.Text = ""
	fb.updateMatches()
	if fb.matches.Len() != 0 {
		t.Fatalf("expected 0 matches, got %d", fb.matches.Len())
	}

	// Case-insensitive search.
	fb.entry.Text = "hello"
	fb.updateMatches()
	if fb.matches.Len() != 2 {
		t.Fatalf("expected 2 matches for 'hello', got %d", fb.matches.Len())
	}
	if fb.matches.Item(0).row != 0 || fb.matches.Item(0).col != 0 {
		t.Errorf("first match at (%d,%d), want (0,0)", fb.matches.Item(0).row, fb.matches.Item(0).col)
	}
	if fb.matches.Item(1).row != 1 || fb.matches.Item(1).col != 0 {
		t.Errorf("second match at (%d,%d), want (1,0)", fb.matches.Item(1).row, fb.matches.Item(1).col)
	}

	// Verify selection was set for first match.
	if selected != [4]int{0, 0, 0, 5} {
		t.Errorf("selection = %v, want [0 0 0 5]", selected)
	}

	// Matches are reported as spans with the first match active.
	if len(reported) != 2 {
		t.Fatalf("reported %d ranges, want 2", len(reported))
	}
	if reportedActive != 0 {
		t.Errorf("reported active = %d, want 0", reportedActive)
	}
	if got := reported[0]; got != (TextRange{StartRow: 0, StartCol: 0, EndRow: 0, EndCol: 5}) {
		t.Errorf("reported[0] = %+v, want {0 0 0 5}", got)
	}

	// Case-sensitive search.
	fb.caseSwitch.On = true
	fb.entry.Text = "Hello"
	fb.updateMatches()
	if fb.matches.Len() != 0 {
		t.Fatalf("expected 0 case-sensitive matches for 'Hello', got %d", fb.matches.Len())
	}

	// FindNext wraps around.
	fb.caseSwitch.On = false
	fb.entry.Text = "hello"
	fb.updateMatches()
	fb.FindNext()
	if fb.currentIndex() != 1 {
		t.Errorf("after FindNext currentMatch = %d, want 1", fb.currentIndex())
	}
	if reportedActive != 1 {
		t.Errorf("after FindNext reported active = %d, want 1", reportedActive)
	}
	fb.FindNext()
	if fb.currentIndex() != 0 {
		t.Errorf("after wrap FindNext currentMatch = %d, want 0", fb.currentIndex())
	}

	// FindPrev wraps around.
	fb.FindPrev()
	if fb.currentIndex() != 1 {
		t.Errorf("after FindPrev currentMatch = %d, want 1", fb.currentIndex())
	}

	// No results.
	fb.entry.Text = "zzz"
	fb.updateMatches()
	if fb.matches.Len() != 0 {
		t.Fatalf("expected 0 matches for 'zzz', got %d", fb.matches.Len())
	}
	// Empty match set reports no spans and an inactive index.
	if len(reported) != 0 {
		t.Errorf("reported %d ranges for no results, want 0", len(reported))
	}
	if reportedActive != -1 {
		t.Errorf("reported active = %d for no results, want -1", reportedActive)
	}
}

// TestFindBarActiveMatchStableAcrossResearch verifies the identity-stable
// current match (#165): after navigating to a later match, re-running the search
// (an OnChanged re-type, a case toggle, a FindNext with a rebuilt set) keeps the
// highlight on the same match by identity instead of snapping back to the first.
func TestFindBarActiveMatchStableAcrossResearch(t *testing.T) {
	test.NewTempApp(t)
	var reportedActive int
	fb := newFindBar(
		func() string { return "ab ab ab ab" }, // four matches on one row
		func(int, int, int, int) {},
		func() {},
		func(_ []TextRange, active int) { reportedActive = active },
	)

	fb.entry.Text = "ab"
	fb.updateMatches()
	fb.FindNext()
	fb.FindNext() // active is now the third match (index 2)
	if fb.currentIndex() != 2 {
		t.Fatalf("setup: currentIndex = %d, want 2", fb.currentIndex())
	}

	// Re-running the identical search must not reset the active match: the
	// third match survives by identity and stays highlighted.
	fb.updateMatches()
	if fb.currentIndex() != 2 {
		t.Errorf("after re-search currentIndex = %d, want 2 (identity-stable)", fb.currentIndex())
	}
	if reportedActive != 2 {
		t.Errorf("after re-search reported active = %d, want 2", reportedActive)
	}

	// A match that no longer exists falls back to the first match.
	fb.entry.Text = "goodbye"
	fb.updateMatches() // 0 matches
	fb.entry.Text = "ab"
	fb.updateMatches()
	if fb.currentIndex() != 0 {
		t.Errorf("after the active match vanished, currentIndex = %d, want 0", fb.currentIndex())
	}
}

func TestFindBarHasQuery(t *testing.T) {
	test.NewTempApp(t)
	fb := newFindBar(
		func() string { return "" },
		func(int, int, int, int) {},
		func() {},
		func([]TextRange, int) {},
	)
	if fb.HasQuery() {
		t.Error("empty query should return false")
	}
	fb.entry.Text = "x"
	if !fb.HasQuery() {
		t.Error("non-empty query should return true")
	}
}
