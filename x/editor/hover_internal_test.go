package editor

import (
	"context"
	"strings"
	"testing"

	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
)

// hoverCardText returns all text rendered in the hover card.
func hoverCardText(e *Editor) string {
	var b strings.Builder
	for _, seg := range e.hover.rich.Segments {
		if ts, ok := seg.(*widget.TextSegment); ok {
			b.WriteString(ts.Text)
		}
	}
	return b.String()
}

type testHoverProvider struct {
	content *HoverContent
	calls   int
}

func (p *testHoverProvider) Hover(_ context.Context, _, _ int) (*HoverContent, error) {
	p.calls++
	return p.content, nil
}

// cellCenter returns the viewport-space center of the (row, col) cell.
func cellCenter(e *Editor, row, col int) geo.Position {
	r := e.PositionAt(row, col)
	return geo.NewPos(r.Pos.X+r.Size.Width/2, r.Pos.Y+r.Size.Height/2)
}

func TestPositionAtPoint_RoundTrip(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "abcdef\nghijk")

	for _, tc := range []struct{ row, col int }{{0, 0}, {0, 3}, {0, 5}, {1, 2}} {
		row, col, ok := e.positionAtPoint(cellCenter(e, tc.row, tc.col))
		if !ok || row != tc.row || col != tc.col {
			t.Errorf("positionAtPoint(center of %d,%d) = (%d,%d,%v)", tc.row, tc.col, row, col, ok)
		}
	}

	// A point well below the last row maps to nothing.
	if _, _, ok := e.positionAtPoint(geo.NewPos(5, 100000)); ok {
		t.Error("a point below all rows should not map to a position")
	}
}

func TestHover_TriggerShowsCardAndOutline(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "foobar baz")
	p := &testHoverProvider{content: &HoverContent{
		Markdown: "**foobar** — a demo symbol",
		Range:    TextRange{StartRow: 0, StartCol: 0, EndRow: 0, EndCol: 6},
	}}
	e.SetHoverProvider(p)

	e.hover.triggerAt(cellCenter(e, 0, 2))
	if !e.hover.visible() {
		t.Fatal("hover card should be visible after dwell over a symbol")
	}
	if p.calls != 1 {
		t.Errorf("provider should have been queried once, got %d", p.calls)
	}
	if got := e.hoverOutline.SpansList; len(got) != 1 ||
		got[0].Range != (TextRange{StartRow: 0, StartCol: 0, EndRow: 0, EndCol: 6}) {
		t.Errorf("hovered range should be outlined, got %v", got)
	}
}

func TestHover_PlainTextContentVerbatim(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "foobar baz")
	e.SetHoverProvider(&testHoverProvider{content: &HoverContent{
		PlainText: "see _ref_ now",
		Range:     TextRange{StartRow: 0, StartCol: 0, EndRow: 0, EndCol: 6},
	}})

	e.hover.triggerAt(cellCenter(e, 0, 2))
	if !e.hover.visible() {
		t.Fatal("hover card should show for plaintext content")
	}
	if got := hoverCardText(e); !strings.Contains(got, "_ref_") {
		t.Errorf("plaintext hover content should render verbatim, got %q", got)
	}
}

func TestHover_MarkdownContentStillParsed(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "foobar baz")
	e.SetHoverProvider(&testHoverProvider{content: &HoverContent{
		Markdown: "see _ref_ now",
		Range:    TextRange{StartRow: 0, StartCol: 0, EndRow: 0, EndCol: 6},
	}})

	e.hover.triggerAt(cellCenter(e, 0, 2))
	if got := hoverCardText(e); !strings.Contains(got, "ref") || strings.Contains(got, "_ref_") {
		t.Errorf("markdown hover content should render formatted (underscores consumed), got %q", got)
	}
}

func TestHover_DismissesOnMoveAway(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "foobar baz")
	e.SetHoverProvider(&testHoverProvider{content: &HoverContent{
		Markdown: "doc",
		Range:    TextRange{StartRow: 0, StartCol: 0, EndRow: 0, EndCol: 6},
	}})

	e.hover.triggerAt(cellCenter(e, 0, 2))
	if !e.hover.visible() {
		t.Fatal("card should be visible")
	}

	// Still within the hovered range → kept.
	e.hover.onMouseMoved(cellCenter(e, 0, 4))
	if !e.hover.visible() {
		t.Error("moving within the hovered range should keep the card")
	}

	// Move past the range → grace dismiss armed (not immediate), then fires.
	e.hover.onMouseMoved(cellCenter(e, 0, 8))
	e.hover.cancelTimer() // stop the re-arm dwell scheduled by the move
	if !e.hover.visible() {
		t.Error("moving off the range should keep the card until the grace timer fires")
	}
	e.hover.graceFire() // simulate grace expiry (pointer not on card)
	if e.hover.visible() {
		t.Error("the card should dismiss when grace fires off the card")
	}
	if e.hoverOutline.SpansList != nil {
		t.Error("outline should be cleared on dismiss")
	}
}

func TestHover_MouseOutDismisses(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "foobar")
	e.SetHoverProvider(&testHoverProvider{content: &HoverContent{Markdown: "doc"}})

	e.hover.triggerAt(cellCenter(e, 0, 2))
	if !e.hover.visible() {
		t.Fatal("card should be visible")
	}
	// Leaving the text-capture layer arms a grace dismiss (so the pointer can
	// land on the card), and the card closes when it fires off the card.
	e.hover.onMouseOut()
	if !e.hover.visible() {
		t.Error("mouse-out should keep the card until grace fires")
	}
	e.hover.graceFire()
	if e.hover.visible() {
		t.Error("the card should dismiss when grace fires after mouse-out")
	}
}

// TestHover_KeepAliveOverCard covers #27: sliding the pointer onto the card
// cancels the grace dismiss and keeps it alive; leaving the card re-arms it.
func TestHover_KeepAliveOverCard(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "foobar baz")
	e.SetHoverProvider(&testHoverProvider{content: &HoverContent{Markdown: "doc"}})

	e.hover.triggerAt(cellCenter(e, 0, 2))
	if !e.hover.visible() {
		t.Fatal("card should be visible")
	}

	// Pointer leaves the text toward the card: grace armed, then the card's
	// HoverIn lands — grace must be canceled and the card kept.
	e.hover.onMouseOut()
	e.hover.onCardIn()
	e.hover.graceFire() // a stale grace must be a no-op while over the card
	if !e.hover.visible() {
		t.Error("the card should stay alive while the pointer is over it")
	}

	// Leaving the card re-arms grace; it then dismisses.
	e.hover.onCardOut()
	e.hover.graceFire()
	if e.hover.visible() {
		t.Error("the card should dismiss after the pointer leaves it")
	}
}

func TestHover_NoProviderUnchanged(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "abc")
	e.hover.onMouseMoved(cellCenter(e, 0, 1)) // must not panic or show
	if e.hover.visible() {
		t.Error("no hover card without a provider")
	}
}

func TestHover_ClearingProviderDismisses(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "foobar")
	e.SetHoverProvider(&testHoverProvider{content: &HoverContent{
		Markdown: "doc",
		Range:    TextRange{StartRow: 0, StartCol: 0, EndRow: 0, EndCol: 6},
	}})
	e.hover.triggerAt(cellCenter(e, 0, 2))
	if !e.hover.visible() {
		t.Fatal("card should be visible")
	}
	e.SetHoverProvider(nil)
	if e.hover.visible() {
		t.Error("clearing the provider should dismiss the card")
	}
	if e.hoverOutline.SpansList != nil {
		t.Error("outline should be cleared when the provider is removed")
	}
}

func TestHover_EmptyMarkdownShowsNothing(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "foobar")
	e.SetHoverProvider(&testHoverProvider{content: nil}) // provider returns no content
	e.hover.triggerAt(cellCenter(e, 0, 2))
	if e.hover.visible() {
		t.Error("a nil HoverContent should show no card")
	}
}

func TestHover_NotShownWhileAnotherPopoverActive(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "foobar")
	e.SetHoverProvider(&testHoverProvider{content: &HoverContent{Markdown: "doc"}})

	// Simulate a completion popup owning the screen.
	other := newPopover(nil)
	other.interactive = true
	e.activePopovers = []*popover{other}

	e.hover.triggerAt(cellCenter(e, 0, 2))
	if e.hover.visible() {
		t.Error("hover must not replace another active popover")
	}
}

func TestWordAt(t *testing.T) {
	text := "foo bar_baz qux"
	if w, rng := wordAt(text, 0, 8); w != "bar_baz" ||
		rng != (TextRange{StartRow: 0, StartCol: 4, EndRow: 0, EndCol: 11}) {
		t.Errorf("wordAt col 8 = %q %v, want \"bar_baz\" [4,11)", w, rng)
	}
	// A run of whitespace yields no word.
	if w, _ := wordAt("   x", 0, 1); w != "" {
		t.Errorf("wordAt over whitespace should be empty, got %q", w)
	}
}

func TestWordHoverProvider(t *testing.T) {
	text := "print value"
	p := &WordHoverProvider{
		Docs: map[string]string{"print": "writes to stdout"},
		Text: func() string { return text },
	}
	got, err := p.Hover(context.Background(), 0, 2)
	if err != nil || got == nil || got.Markdown != "writes to stdout" {
		t.Fatalf("expected docs for 'print', got %v (err %v)", got, err)
	}
	if got.Range != (TextRange{StartRow: 0, StartCol: 0, EndRow: 0, EndCol: 5}) {
		t.Errorf("range should cover 'print', got %v", got.Range)
	}
	// An undocumented word returns nothing.
	if got, _ := p.Hover(context.Background(), 0, 7); got != nil {
		t.Errorf("undocumented word should return nil, got %v", got)
	}
}

// --- #29: keyboard "show hover" ------------------------------------------

func wordHoverEditor(t *testing.T) *Editor {
	t.Helper()
	e := renderedPopoverEditor(t, "foobar baz")
	e.SetHoverProvider(&WordHoverProvider{
		Docs: map[string]string{"foobar": "the foobar symbol"},
		Text: e.Text,
	})
	return e
}

func TestHover_ShowAtCursor(t *testing.T) {
	test.NewTempApp(t)
	e := wordHoverEditor(t)
	e.SetCursorPos(0, 2) // inside "foobar"

	e.ShowHoverAtCursor()
	if !e.hover.visible() {
		t.Fatal("ShowHoverAtCursor should show the card at the cursor")
	}
	if !e.hover.keyboardShown {
		t.Error("keyboardShown should be set for a keyboard-invoked card")
	}

	// A cursor move dismisses the keyboard-anchored card.
	e.entry.OnCursorChanged()
	if e.hover.visible() {
		t.Error("a cursor move should dismiss the keyboard hover card")
	}
}

func TestHover_ShowAtCursorNoProviderNoop(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "foobar")
	e.SetCursorPos(0, 2)
	e.ShowHoverAtCursor()
	if e.hover.visible() {
		t.Error("ShowHoverAtCursor should be a no-op with no provider/diagnostics")
	}
}

func TestChord_ShowsHoverOnKThenI(t *testing.T) {
	test.NewTempApp(t)
	e := wordHoverEditor(t)
	e.SetCursorPos(0, 2)
	mod := event.KeyModifierShortcutDefault

	if !e.handleChordShortcut(&desktop.CustomShortcut{Modifier: mod, KeyName: event.KeyK}) {
		t.Fatal("the leading chord key should be consumed")
	}
	if !e.chordPending {
		t.Error("the chord should be pending after K")
	}
	if !e.handleChordShortcut(&desktop.CustomShortcut{Modifier: mod, KeyName: event.KeyI}) {
		t.Fatal("the second chord key should be consumed")
	}
	if e.chordPending {
		t.Error("the chord should be cleared after completion")
	}
	if !e.hover.visible() {
		t.Error("completing the chord should show the hover card")
	}
}

func TestChord_ForeignShortcutCancels(t *testing.T) {
	test.NewTempApp(t)
	e := wordHoverEditor(t)
	mod := event.KeyModifierShortcutDefault

	e.handleChordShortcut(&desktop.CustomShortcut{Modifier: mod, KeyName: event.KeyK})
	// A different shortcut clears the pending chord and is not consumed (so it
	// runs normally).
	if e.handleChordShortcut(&desktop.CustomShortcut{Modifier: mod, KeyName: event.KeyS}) {
		t.Error("a foreign shortcut should fall through, not be consumed")
	}
	if e.chordPending {
		t.Error("a foreign shortcut should clear the pending chord")
	}
	// I after the cancel must not fire hover (no pending chord).
	if e.handleChordShortcut(&desktop.CustomShortcut{Modifier: mod, KeyName: event.KeyI}) {
		t.Error("I with no pending chord should not be consumed")
	}
	if e.hover.visible() {
		t.Error("hover should not show without a completed chord")
	}
}
