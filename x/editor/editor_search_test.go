package editor

import (
	"testing"

	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// TestEditorSearchSpanWiring verifies the find bar feeds match ranges into the
// editor's internal SearchMatchSpanDecorator and clears them when hidden.
func TestEditorSearchSpanWiring(t *testing.T) {
	test.NewTempApp(t)
	e := NewEditor()
	w := test.NewWindow(e)
	defer w.Close()
	w.Resize(geo.NewSize(400, 300))
	e.SetText("alpha beta\nalpha gamma\ndelta")

	e.ShowFind()
	e.find.entry.Text = "alpha"
	e.find.updateMatches()

	if got := len(e.searchSpans.Matches); got != 2 {
		t.Fatalf("searchSpans.Matches = %d, want 2", got)
	}
	if e.searchSpans.Active != 0 {
		t.Errorf("searchSpans.Active = %d, want 0", e.searchSpans.Active)
	}
	if got := len(e.searchSpans.Spans()); got != 2 {
		t.Errorf("decorator produced %d spans, want 2", got)
	}

	// Navigating advances the active span.
	e.FindNext()
	if e.searchSpans.Active != 1 {
		t.Errorf("after FindNext searchSpans.Active = %d, want 1", e.searchSpans.Active)
	}

	// Hiding the find bar clears the highlights.
	e.HideFind()
	if e.searchSpans.Matches != nil {
		t.Errorf("after HideFind Matches = %d, want nil", len(e.searchSpans.Matches))
	}
	if e.searchSpans.Active != -1 {
		t.Errorf("after HideFind Active = %d, want -1", e.searchSpans.Active)
	}
}
