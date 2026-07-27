package editor

import (
	"testing"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

func TestCompositeGutterWidth(t *testing.T) {
	test.NewTempApp(t)
	ln := NewLineNumberGutter()
	bp := NewBreakpointGutter(nil)
	comp := NewCompositeGutter(ln, bp)

	w := comp.Width(100, 14)
	lnW := ln.Width(100, 14)
	bpW := bp.Width(100, 14)
	if w != lnW+bpW {
		t.Errorf("composite width = %f, want %f + %f = %f", w, lnW, bpW, lnW+bpW)
	}
}

func TestCompositeGutterRender(t *testing.T) {
	test.NewTempApp(t)
	ln := NewLineNumberGutter()
	bp := NewBreakpointGutter(nil)
	bp.Lines[3] = true // 1-based: line 3
	comp := NewCompositeGutter(ln, bp)

	// Prime the cached totalRows via Width.
	comp.Width(10, 14)

	size := geo.NewSize(80, 14)

	// Line 1: line number only, no breakpoint.
	objs := comp.Render(1, size, 14)
	if len(objs) != 1 { // line number text only
		t.Errorf("line 1: got %d objects, want 1", len(objs))
	}

	// Line 3: line number + breakpoint dot.
	objs = comp.Render(3, size, 14)
	if len(objs) != 2 {
		t.Errorf("line 3: got %d objects, want 2", len(objs))
	}
}

func TestBreakpointToggle(t *testing.T) {
	var lastLines map[int]bool
	bp := NewBreakpointGutter(func(lines map[int]bool) {
		lastLines = lines
	})

	// Toggle on.
	bp.Tapped(5)
	if !bp.Lines[5] {
		t.Error("line 5 should be set after first tap")
	}
	if !lastLines[5] {
		t.Error("OnChanged should report line 5 set")
	}

	// Toggle off.
	bp.Tapped(5)
	if bp.Lines[5] {
		t.Error("line 5 should be unset after second tap")
	}
}

func TestLineNumberGutterMinDigits(t *testing.T) {
	test.NewTempApp(t)
	g := &LineNumberGutter{MinDigits: 5}
	w5 := g.Width(1, 14) // 1 line but 5-digit minimum

	g2 := NewLineNumberGutter() // default 3-digit minimum
	w3 := g2.Width(1, 14)

	if w5 <= w3 {
		t.Errorf("5-digit width (%f) should be > 3-digit width (%f)", w5, w3)
	}
}

// TestCompositeGutter_ColumnAwareTapRouting verifies a tap reaches only the
// interactive provider whose column was clicked, so click-to-reveal and an
// interactive breakpoint gutter in the same composite don't both fire (#95).
func TestCompositeGutter_ColumnAwareTapRouting(t *testing.T) {
	test.NewTempApp(t)
	bp := NewBreakpointGutter(nil)
	diag := NewDiagnosticGutter()
	diag.SetDiagnostics([]Diagnostic{
		{Range: TextRange{StartRow: 0, StartCol: 0, EndRow: 0, EndCol: 3}, Severity: SeverityError},
	})
	var revealed []TextRange
	diag.RevealOnTap = func(r TextRange) { revealed = append(revealed, r) }

	comp := NewCompositeGutter(bp, diag)
	const textSize = 14
	comp.Width(10, textSize) // prime cached totalRows + textSize for hit-testing
	bpW := bp.Width(10, textSize)

	// Tap in the breakpoint (left) column: toggles the breakpoint only.
	comp.TappedAtX(1, bpW/2)
	if !bp.Lines[1] {
		t.Error("tap in breakpoint column should toggle its breakpoint")
	}
	if len(revealed) != 0 {
		t.Errorf("tap in breakpoint column must not reveal a diagnostic, got %d", len(revealed))
	}

	// Tap in the diagnostic (right) column: reveals only.
	comp.TappedAtX(1, bpW+diag.Width(10, textSize)/2)
	if len(revealed) != 1 {
		t.Fatalf("tap in diagnostic column should reveal once, got %d", len(revealed))
	}
	if len(bp.Lines) != 1 { // still just the one from the first tap
		t.Errorf("tap in diagnostic column must not toggle a breakpoint, lines = %v", bp.Lines)
	}

	// A tap past every column is a no-op (no panic, no effect).
	comp.TappedAtX(1, bpW+diag.Width(10, textSize)+100)
	if len(revealed) != 1 || len(bp.Lines) != 1 {
		t.Error("tap outside all columns should be a no-op")
	}
}

// TestGutterWidget_RoutesTapByColumn verifies the gutter widget prefers the
// column-aware path, so an end-to-end tap lands in the clicked column only (#95).
func TestGutterWidget_RoutesTapByColumn(t *testing.T) {
	test.NewTempApp(t)
	bp := NewBreakpointGutter(nil)
	diag := NewDiagnosticGutter()
	diag.SetDiagnostics([]Diagnostic{
		{Range: TextRange{StartRow: 0, StartCol: 0, EndRow: 0, EndCol: 3}, Severity: SeverityError},
	})
	var revealed int
	diag.RevealOnTap = func(TextRange) { revealed++ }

	comp := NewCompositeGutter(bp, diag)
	w := newGutterWidget(comp)
	w.getLineCount = func() int { return 1 } // "abc": one line
	w.stride = 18
	comp.Width(w.rowCount(), editorTextSize()) // prime hit-test cache to the widget's text size
	bpW := bp.Width(w.rowCount(), editorTextSize())

	// Tap row 1 in the diagnostic (right) column.
	w.Tapped(&event.PointEvent{Position: geo.NewPos(bpW+1, 1)})
	if revealed != 1 {
		t.Errorf("right-column tap should reveal, got %d", revealed)
	}
	if len(bp.Lines) != 0 {
		t.Errorf("right-column tap must not toggle a breakpoint, lines = %v", bp.Lines)
	}

	// Tap row 1 in the breakpoint (left) column.
	w.Tapped(&event.PointEvent{Position: geo.NewPos(bpW/2, 1)})
	if !bp.Lines[1] {
		t.Error("left-column tap should toggle the breakpoint")
	}
	if revealed != 1 {
		t.Errorf("left-column tap must not reveal, reveal count = %d", revealed)
	}
}

// TestCompositeGutter_MarginTapTogglesBreakpoint verifies that in the common
// [line numbers, diagnostic, breakpoint] composition, a tap on the wide
// non-interactive line-number margin toggles the breakpoint (VS Code-style),
// while the diagnostic marker column still reveals only — no double-fire (#109).
func TestCompositeGutter_MarginTapTogglesBreakpoint(t *testing.T) {
	test.NewTempApp(t)
	ln := NewLineNumberGutter()
	diag := NewDiagnosticGutter()
	diag.SetDiagnostics([]Diagnostic{
		{Range: TextRange{StartRow: 0, StartCol: 0, EndRow: 0, EndCol: 3}, Severity: SeverityError},
	})
	var revealed []TextRange
	diag.RevealOnTap = func(r TextRange) { revealed = append(revealed, r) }
	bp := NewBreakpointGutter(nil)

	comp := NewCompositeGutter(ln, diag, bp)
	const textSize = 14
	comp.Width(10, textSize) // prime cached totalRows + textSize for hit-testing
	diagW := diag.Width(10, textSize)
	bpW := bp.Width(10, textSize)
	// Rendered order is the canonical one: markers (diagnostic, breakpoint) on
	// the leading edge, then the line-number margin nearest the text.
	lnX := diagW + bpW

	// Tap in the wide line-number margin: toggles the breakpoint, reveals nothing.
	comp.TappedAtX(1, lnX+ln.Width(10, textSize)/2)
	if !bp.Lines[1] {
		t.Error("tap in the line-number margin should toggle the breakpoint (#109)")
	}
	if len(revealed) != 0 {
		t.Errorf("margin tap must not reveal a diagnostic, got %d", len(revealed))
	}

	// Tap in the diagnostic marker column: reveals only, no breakpoint toggle.
	comp.TappedAtX(1, diagW/2)
	if len(revealed) != 1 {
		t.Fatalf("tap in the diagnostic column should reveal once, got %d", len(revealed))
	}
	if !bp.Lines[1] || len(bp.Lines) != 1 { // unchanged from the margin tap
		t.Errorf("diagnostic-column tap must not toggle a breakpoint, lines = %v", bp.Lines)
	}

	// Tap in the breakpoint marker column: toggles it back off, reveals nothing.
	comp.TappedAtX(1, diagW+bpW/2)
	if bp.Lines[1] {
		t.Error("tap in the breakpoint column should toggle its breakpoint")
	}
	if len(revealed) != 1 {
		t.Errorf("breakpoint-column tap must not reveal, got %d", len(revealed))
	}
}

// TestCompositeGutter_CanonicalColumnOrder verifies the composite arranges its
// providers into the conventional editor order regardless of the order they are
// passed: markers on the leading edge, then the line numbers, then the fold
// control beside the text — so the numbers are never pushed away from the code.
func TestCompositeGutter_CanonicalColumnOrder(t *testing.T) {
	test.NewTempApp(t)
	ln := NewLineNumberGutter()
	fold := newFoldGutter(nil, nil, nil)
	bp := NewBreakpointGutter(nil)
	diag := NewDiagnosticGutter()

	comp := NewCompositeGutter(ln, fold, bp, diag)
	want := []GutterProvider{bp, diag, ln, fold} // markers keep their relative order
	got := comp.Providers()
	if len(got) != len(want) {
		t.Fatalf("got %d providers, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("column %d = %T (%p), want %T (%p)", i, got[i], got[i], want[i], want[i])
		}
	}

	// The explicit-order constructor keeps the caller's layout verbatim.
	inOrder := NewCompositeGutterInOrder(ln, fold, bp, diag)
	if p := inOrder.Providers(); p[0] != ln || p[1] != fold || p[2] != bp || p[3] != diag {
		t.Errorf("NewCompositeGutterInOrder reordered its providers: %T %T %T %T", p[0], p[1], p[2], p[3])
	}
}

// TestCompositeGutter_UnknownProviderIsMarker verifies a host provider that does
// not declare a column stays on the leading (marker) edge rather than landing
// between the line numbers and the text.
func TestCompositeGutter_UnknownProviderIsMarker(t *testing.T) {
	test.NewTempApp(t)
	ln := NewLineNumberGutter()
	custom := &staticGutter{width: 6}

	comp := NewCompositeGutter(ln, custom)
	if p := comp.Providers(); p[0] != GutterProvider(custom) || p[1] != GutterProvider(ln) {
		t.Errorf("undeclared provider should sort as a marker, got %T then %T", p[0], p[1])
	}
}

// staticGutter is a minimal host-style provider that declares no column.
type staticGutter struct{ width float32 }

func (g *staticGutter) Width(int, float32) float32 { return g.width }

func (g *staticGutter) Render(int, geo.Size, float32) []core.CanvasObject { return nil }

// TestCompositeGutter_MarginTapNoBreakpointProvider verifies a margin tap is a
// harmless no-op when no breakpoint gutter is present (no panic, nothing to do).
func TestCompositeGutter_MarginTapNoBreakpointProvider(t *testing.T) {
	test.NewTempApp(t)
	ln := NewLineNumberGutter()
	comp := NewCompositeGutter(ln)
	comp.Width(10, 14)
	comp.TappedAtX(1, ln.Width(10, 14)/2) // must not panic
}

// TestGutterWidget_PhantomRowsFollowFoldedContent covers the gutter alignment
// bug behind a collapsed fold: the numbers below the document were positioned
// from the row INDEX (topOffset + row*stride), which ignores that a fold removes
// height without removing rows — so in a short (or folded) document the trailing
// numbers detached from the text, leaving a gap the size of the folded region.
// They must continue directly under the last visible content row.
func TestGutterWidget_PhantomRowsFollowFoldedContent(t *testing.T) {
	test.NewTempApp(t)
	const (
		stride = float32(20)
		top    = float32(5)
		rows   = 12
	)
	g := newGutterWidget(NewLineNumberGutter())
	g.stride = stride
	g.topOffset = top
	g.getLineCount = func() int { return rows }
	// Rows 1..6 are folded away: zero height, sharing the fold point's Y, exactly
	// as the Entry reports them.
	g.getRowPositions = func() ([]float32, []float32) {
		y := make([]float32, rows)
		h := make([]float32, rows)
		next := top
		for i := range y {
			y[i] = next
			if i >= 1 && i <= 6 {
				continue // folded: height stays 0 and the next row shares this Y
			}
			h[i] = stride
			next += stride
		}
		return y, h
	}

	r := &gutterWidgetRenderer{gutter: g}
	r.Layout(geo.NewSize(60, 400))

	got := map[string]float32{}
	for _, o := range r.Objects() {
		if txt, ok := o.(*canvas.Text); ok {
			got[txt.Text] = txt.Position().Y
		}
	}
	// Visible content rows: 1, 8, 9, 10, 11, 12 — six rows from top.
	for i, line := range []string{"1", "8", "9", "10", "11", "12"} {
		want := top + float32(i)*stride
		if y, ok := got[line]; !ok || y != want {
			t.Errorf("line %s at y=%v (present=%v), want %v", line, y, ok, want)
		}
	}
	// The first phantom row continues directly under line 12, with no gap.
	if y, ok := got["13"]; !ok || y != top+6*stride {
		t.Errorf("phantom line 13 at y=%v (present=%v), want %v — numbers below a fold must "+
			"follow the content, not the unfolded row index", y, ok, top+6*stride)
	}
	if y, ok := got["14"]; !ok || y != top+7*stride {
		t.Errorf("phantom line 14 at y=%v (present=%v), want %v", y, ok, top+7*stride)
	}
}
