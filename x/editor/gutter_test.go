package editor

import (
	"testing"

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
	lnW := ln.Width(10, textSize)
	diagW := diag.Width(10, textSize)
	bpW := bp.Width(10, textSize)

	// Tap in the wide line-number margin: toggles the breakpoint, reveals nothing.
	comp.TappedAtX(1, lnW/2)
	if !bp.Lines[1] {
		t.Error("tap in the line-number margin should toggle the breakpoint (#109)")
	}
	if len(revealed) != 0 {
		t.Errorf("margin tap must not reveal a diagnostic, got %d", len(revealed))
	}

	// Tap in the diagnostic marker column: reveals only, no breakpoint toggle.
	comp.TappedAtX(1, lnW+diagW/2)
	if len(revealed) != 1 {
		t.Fatalf("tap in the diagnostic column should reveal once, got %d", len(revealed))
	}
	if !bp.Lines[1] || len(bp.Lines) != 1 { // unchanged from the margin tap
		t.Errorf("diagnostic-column tap must not toggle a breakpoint, lines = %v", bp.Lines)
	}

	// Tap in the breakpoint marker column: toggles it back off, reveals nothing.
	comp.TappedAtX(1, lnW+diagW+bpW/2)
	if bp.Lines[1] {
		t.Error("tap in the breakpoint column should toggle its breakpoint")
	}
	if len(revealed) != 1 {
		t.Errorf("breakpoint-column tap must not reveal, got %d", len(revealed))
	}
}

// TestCompositeGutter_MarginTapNoBreakpointProvider verifies a margin tap is a
// harmless no-op when no breakpoint gutter is present (no panic, nothing to do).
func TestCompositeGutter_MarginTapNoBreakpointProvider(t *testing.T) {
	test.NewTempApp(t)
	ln := NewLineNumberGutter()
	comp := NewCompositeGutter(ln)
	comp.Width(10, 14)
	comp.TappedAtX(1, ln.Width(10, 14)/2) // must not panic
}
