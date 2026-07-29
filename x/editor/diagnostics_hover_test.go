package editor

import (
	"strings"
	"testing"

	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
)

// TestDiagnostics_PointRangeSquiggle verifies a zero-width (point) diagnostic is
// widened to a visible, non-empty span (#52) while non-zero ranges are unchanged.
func TestDiagnostics_PointRangeSquiggle(t *testing.T) {
	test.NewTempApp(t)
	e := NewEditor()
	e.SetText("foo()")
	e.SetDiagnostics([]Diagnostic{
		{Range: TextRange{StartRow: 0, StartCol: 4, EndRow: 0, EndCol: 4}, Severity: SeverityError, Message: "expected ')'"},
		{Range: TextRange{StartRow: 0, StartCol: 0, EndRow: 0, EndCol: 3}, Severity: SeverityWarning, Message: "unused"},
	})

	spans := e.diagnostics.Spans()
	if len(spans) != 2 {
		t.Fatalf("expected 2 spans, got %d", len(spans))
	}
	// The point range is widened to one cell so it actually paints.
	if got := spans[0].Range; got.IsEmpty() {
		t.Errorf("point diagnostic should widen to a non-empty span, got %v", got)
	}
	if want := (TextRange{StartRow: 0, StartCol: 4, EndRow: 0, EndCol: 5}); spans[0].Range != want {
		t.Errorf("point span = %v, want %v", spans[0].Range, want)
	}
	// The non-zero range is carried through unchanged.
	if want := (TextRange{StartRow: 0, StartCol: 0, EndRow: 0, EndCol: 3}); spans[1].Range != want {
		t.Errorf("non-zero span = %v, want %v", spans[1].Range, want)
	}
}

func TestDiagnosticsAt_IncludesPointRanges(t *testing.T) {
	test.NewTempApp(t)
	e := NewEditor()
	e.SetText("foo()")
	e.SetDiagnostics([]Diagnostic{
		{Range: TextRange{StartRow: 0, StartCol: 4, EndRow: 0, EndCol: 4}, Severity: SeverityError, Message: "expected ')'"},
	})

	if got := e.diagnosticsAt(0, 4); len(got) != 1 {
		t.Errorf("a point diagnostic should be found at its anchor cell, got %d", len(got))
	}
	if got := e.diagnosticsAt(0, 0); len(got) != 0 {
		t.Errorf("no diagnostic at col 0, got %d", len(got))
	}
}

// TestHover_DiagnosticWithoutProvider verifies dwelling over a diagnostic shows
// its message even with no host HoverProvider installed.
func TestHover_DiagnosticWithoutProvider(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "foobar baz")
	e.SetDiagnostics([]Diagnostic{
		{Range: TextRange{StartRow: 0, StartCol: 0, EndRow: 0, EndCol: 6}, Severity: SeverityError, Message: "bad symbol", Source: "lint"},
	})

	e.hover.triggerAt(cellCenter(e, 0, 2))
	if !e.hover.visible() {
		t.Fatal("hover card should show a diagnostic message without a provider")
	}
	md := e.hover.rich.String()
	if !strings.Contains(md, "bad symbol") {
		t.Errorf("card should contain the diagnostic message, got %q", md)
	}
	if !strings.Contains(md, "lint") {
		t.Errorf("card should name the diagnostic source, got %q", md)
	}
}

// TestHover_DiagnosticAndProviderCombine verifies a point with both a diagnostic
// and a provider symbol shows both.
func TestHover_DiagnosticAndProviderCombine(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "foobar baz")
	e.SetHoverProvider(&testHoverProvider{content: &HoverContent{
		Markdown: "**foobar** — a demo symbol",
		Range:    TextRange{StartRow: 0, StartCol: 0, EndRow: 0, EndCol: 6},
	}})
	e.SetDiagnostics([]Diagnostic{
		{Range: TextRange{StartRow: 0, StartCol: 0, EndRow: 0, EndCol: 6}, Severity: SeverityWarning, Message: "shadowed"},
	})

	e.hover.triggerAt(cellCenter(e, 0, 2))
	if !e.hover.visible() {
		t.Fatal("card should be visible")
	}
	md := e.hover.rich.String()
	if !strings.Contains(md, "shadowed") || !strings.Contains(md, "demo symbol") {
		t.Errorf("card should combine diagnostic and provider content, got %q", md)
	}
	// Diagnostics own the squiggle, so no extra outline box is drawn.
	if e.hoverOutline.SpansList != nil {
		t.Errorf("diagnostic hover should not add an outline (squiggle suffices), got %v", e.hoverOutline.SpansList)
	}
}

func TestHover_NoProviderNoDiagnostic(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "abc")
	e.hover.triggerAt(cellCenter(e, 0, 1)) // no provider, no diagnostics
	if e.hover.visible() {
		t.Error("no card without a provider or diagnostic")
	}
}

// TestHover_DiagnosticMessageRendersVerbatim verifies a diagnostic message with
// markdown-significant characters is shown literally, not reinterpreted (#92),
// while the dimmed source annotation still renders.
func TestHover_DiagnosticMessageRendersVerbatim(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "foobar baz")
	e.SetDiagnostics([]Diagnostic{
		{Range: TextRange{StartRow: 0, StartCol: 0, EndRow: 0, EndCol: 6},
			Severity: SeverityError, Message: "use _ptr_ not *x*", Source: "lint"},
	})

	e.hover.triggerAt(cellCenter(e, 0, 2))
	if !e.hover.visible() {
		t.Fatal("hover card should show the diagnostic")
	}
	got := e.hover.rich.String()
	if !strings.Contains(got, "use _ptr_ not *x*") {
		t.Errorf("diagnostic message should render verbatim, got %q", got)
	}
	if !strings.Contains(got, "lint") {
		t.Errorf("source annotation should still render, got %q", got)
	}
}

func TestHoverCardSegments(t *testing.T) {
	diags := []Diagnostic{
		{Message: "first", Source: "a"},
		{Message: "second"},
	}
	segs := hoverCardSegments(diags, &HoverContent{Markdown: "doc"})
	var text strings.Builder
	for _, seg := range segs {
		if ts, ok := seg.(*widget.TextSegment); ok {
			text.WriteString(ts.Text)
		}
	}
	for _, want := range []string{"first", "(a)", "second", "doc"} {
		if !strings.Contains(text.String(), want) {
			t.Errorf("hover segments missing %q, got %q", want, text.String())
		}
	}
	// Nothing to show → no segments.
	if got := hoverCardSegments(nil, nil); len(got) != 0 {
		t.Errorf("empty inputs should produce no segments, got %d", len(got))
	}
}

// TestHoverCard_DiagnosticNewlineTermination guards the layout fix: each
// diagnostic's inline message+source run must be newline-terminated, so the
// following block (divider / provider content) starts on its own row instead of
// overlapping the diagnostic (a fyne RichText quirk). A diagnostics-only card must
// not end in a blank line.
func TestHoverCard_DiagnosticNewlineTermination(t *testing.T) {
	// Message and source are inline (so they share one line), and each diagnostic
	// ends with a newline.
	segs := diagnosticSegments([]Diagnostic{{Message: "m1", Source: "s1"}, {Message: "m2"}})
	nl := 0
	for _, s := range segs {
		ts, ok := s.(*widget.TextSegment)
		if !ok {
			t.Fatalf("unexpected non-text segment %T", s)
		}
		if ts.Text == "\n" {
			nl++
			continue
		}
		if !ts.Style.Inline {
			t.Errorf("segment %q must be inline to share its diagnostic's line", ts.Text)
		}
	}
	if nl != 2 {
		t.Errorf("want one newline terminator per diagnostic (2), got %d", nl)
	}

	// With a provider block after the diagnostics, the segment immediately before
	// the divider (the first non-text/separator segment) must be a newline.
	card := hoverCardSegments([]Diagnostic{{Message: "msg", Source: "lint"}}, &HoverContent{Markdown: "doc"})
	dividerAt := -1
	for i, s := range card {
		if _, ok := s.(*widget.TextSegment); !ok {
			dividerAt = i
			break
		}
	}
	if dividerAt <= 0 {
		t.Fatalf("expected a divider segment after the diagnostic block, segs=%d dividerAt=%d", len(card), dividerAt)
	}
	if ts, ok := card[dividerAt-1].(*widget.TextSegment); !ok || ts.Text != "\n" {
		t.Errorf("diagnostic block must be newline-terminated before the divider, got %#v", card[dividerAt-1])
	}

	// Diagnostics-only: the trailing newline is trimmed (no blank line).
	only := hoverCardSegments([]Diagnostic{{Message: "msg"}}, nil)
	if last, ok := only[len(only)-1].(*widget.TextSegment); ok && last.Text == "\n" {
		t.Error("a diagnostics-only card should not end with a blank line")
	}
}

// TestHoverShownRange_UnionOfOverlapping verifies the card's keep-alive range is
// the bounding union of every matched diagnostic (plus any provider range), so
// the card persists across the whole hovered region when ranges of different
// extents overlap (#58).
func TestHoverShownRange_UnionOfOverlapping(t *testing.T) {
	// A small inner diagnostic and a larger outer one both cover (0,4).
	inner := Diagnostic{Range: TextRange{StartRow: 0, StartCol: 3, EndRow: 0, EndCol: 5}}
	outer := Diagnostic{Range: TextRange{StartRow: 0, StartCol: 0, EndRow: 0, EndCol: 10}}

	got := hoverShownRange([]Diagnostic{inner, outer}, nil, 0, 4)
	want := TextRange{StartRow: 0, StartCol: 0, EndRow: 0, EndCol: 10}
	if got != want {
		t.Errorf("union range = %+v, want %+v", got, want)
	}
	// The union must keep covering a point outside the first diagnostic but inside
	// the larger one — the regression #58 fixes.
	if !got.Contains(0, 8) {
		t.Errorf("union range %+v should still contain (0,8)", got)
	}

	// Single diagnostic: unchanged (its widened range).
	if got := hoverShownRange([]Diagnostic{inner}, nil, 0, 4); got != inner.Range {
		t.Errorf("single-diagnostic range = %+v, want %+v", got, inner.Range)
	}

	// Provider-only: the provider's explicit range is used.
	prov := TextRange{StartRow: 1, StartCol: 2, EndRow: 1, EndCol: 6}
	if got := hoverShownRange(nil, &HoverContent{Range: prov}, 1, 3); got != prov {
		t.Errorf("provider-only range = %+v, want %+v", got, prov)
	}

	// Diagnostic + provider: union spans both.
	got = hoverShownRange([]Diagnostic{inner}, &HoverContent{Range: TextRange{StartRow: 0, StartCol: 20, EndRow: 0, EndCol: 25}}, 0, 4)
	if want := (TextRange{StartRow: 0, StartCol: 3, EndRow: 0, EndCol: 25}); got != want {
		t.Errorf("diagnostic+provider range = %+v, want %+v", got, want)
	}

	// Nothing: the single hovered cell.
	if got := hoverShownRange(nil, nil, 2, 7); got != (TextRange{StartRow: 2, StartCol: 7, EndRow: 2, EndCol: 8}) {
		t.Errorf("fallback range = %+v, want single cell at (2,7)", got)
	}
}
