package editor

import (
	"testing"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

func TestDiagnosticGutter_LineSeverityMap(t *testing.T) {
	g := NewDiagnosticGutter()
	g.SetDiagnostics([]Diagnostic{
		// Single-line warning on line 1 (row 0).
		{Range: TextRange{StartRow: 0, StartCol: 0, EndRow: 0, EndCol: 3}, Severity: SeverityWarning},
		// Error overlapping the same line — most severe must win.
		{Range: TextRange{StartRow: 0, StartCol: 1, EndRow: 0, EndCol: 2}, Severity: SeverityError},
		// Multi-row info on rows 2..4, but ending at col 0 of row 4: row 4 untouched.
		{Range: TextRange{StartRow: 2, StartCol: 0, EndRow: 4, EndCol: 0}, Severity: SeverityInfo},
	})

	if got, want := g.severities[1], SeverityError; got != want {
		t.Errorf("line 1 severity = %v, want %v (most severe)", got, want)
	}
	for _, line := range []int{3, 4} { // rows 2,3 → lines 3,4
		if got, want := g.severities[line], SeverityInfo; got != want {
			t.Errorf("line %d severity = %v, want %v", line, got, want)
		}
	}
	if _, ok := g.severities[5]; ok { // row 4 (line 5) is past the half-open end
		t.Error("a range ending at column 0 should not mark that row's line")
	}
	if _, ok := g.severities[2]; ok {
		t.Error("line 2 has no diagnostic and should carry no severity")
	}
}

func TestDiagnosticGutter_RendersMarkerOnlyOnDiagnosticLines(t *testing.T) {
	test.NewTempApp(t)
	g := NewDiagnosticGutter()
	g.SetDiagnostics([]Diagnostic{
		{Range: TextRange{StartRow: 0, StartCol: 0, EndRow: 0, EndCol: 3}, Severity: SeverityError},
	})
	size := geo.NewSize(g.Width(10, 13), 18)

	if objs := g.Render(1, size, 13); len(objs) != 1 {
		t.Errorf("line 1 has a diagnostic: want 1 marker, got %d", len(objs))
	}
	if objs := g.Render(2, size, 13); objs != nil {
		t.Errorf("line 2 has no diagnostic: want no marker, got %d", len(objs))
	}
}

func TestEditorDiagnosticGutter_FedAndCleared(t *testing.T) {
	test.NewTempApp(t)
	e := NewEditor()
	e.SetText("abc\ndefg")

	g := e.DiagnosticGutter()
	if g == nil {
		t.Fatal("DiagnosticGutter() must return the editor's gutter")
	}
	e.SetDiagnostics(sampleDiagnostics()) // error on line 1, warning on line 2
	if got, want := g.severities[1], SeverityError; got != want {
		t.Errorf("line 1 = %v, want %v", got, want)
	}
	if got, want := g.severities[2], SeverityWarning; got != want {
		t.Errorf("line 2 = %v, want %v", got, want)
	}

	e.SetDiagnostics(nil)
	if len(g.severities) != 0 {
		t.Errorf("clearing diagnostics should remove all markers, got %d", len(g.severities))
	}
}

func TestDiagnosticGutter_ComposesWithLineNumbers(t *testing.T) {
	test.NewTempApp(t)
	e := NewEditor()
	e.SetText("abc\ndefg")
	e.Gutter = NewCompositeGutter(NewLineNumberGutter(), e.DiagnosticGutter())
	e.SetDiagnostics(sampleDiagnostics())

	// The composite renders without error and includes the diagnostic marker on
	// a flagged line (line 1 = error) on top of the line-number text.
	w := test.NewWindow(e)
	defer w.Close()
	w.Resize(geo.NewSize(400, 300))

	objs := e.Gutter.Render(1, geo.NewSize(e.Gutter.Width(2, 13), 18), 13)
	if len(objs) < 2 {
		t.Errorf("composite gutter should render line number + diagnostic marker, got %d objects", len(objs))
	}
}

// shapeOf names the marker silhouette a severity renders, so the test asserts the
// four severities are distinguishable by shape alone — not color (#56).
func shapeOf(t *testing.T, obj core.CanvasObject) string {
	t.Helper()
	switch o := obj.(type) {
	case *canvas.Circle:
		return "circle"
	case *canvas.Polygon:
		switch {
		case o.Sides == 3:
			return "triangle"
		case o.Sides == 4 && o.Angle == 45:
			return "square"
		case o.Sides == 4:
			return "diamond"
		}
	}
	t.Fatalf("unexpected marker object %T", obj)
	return ""
}

func TestDiagnosticGutter_DistinctShapePerSeverity(t *testing.T) {
	test.NewTempApp(t)
	g := NewDiagnosticGutter()
	size := geo.NewSize(g.Width(10, 13), 18)

	want := map[Severity]string{
		SeverityError:   "circle",
		SeverityWarning: "triangle",
		SeverityInfo:    "square",
		SeverityHint:    "diamond",
	}
	seen := map[string]bool{}
	for sev, shape := range want {
		g.SetDiagnostics([]Diagnostic{
			{Range: TextRange{StartRow: 0, StartCol: 0, EndRow: 0, EndCol: 3}, Severity: sev},
		})
		objs := g.Render(1, size, 13)
		if len(objs) != 1 {
			t.Fatalf("severity %v: want 1 marker, got %d", sev, len(objs))
		}
		if got := shapeOf(t, objs[0]); got != shape {
			t.Errorf("severity %v marker = %q, want %q", sev, got, shape)
		}
		seen[shape] = true
	}
	if len(seen) != 4 {
		t.Errorf("severities must use 4 distinct shapes, got %d: %v", len(seen), seen)
	}
}

func TestDiagnosticGutter_MarkerFuncOverride(t *testing.T) {
	test.NewTempApp(t)
	g := NewDiagnosticGutter()
	g.SetDiagnostics([]Diagnostic{
		{Range: TextRange{StartRow: 0, StartCol: 0, EndRow: 0, EndCol: 3}, Severity: SeverityError},
	})
	var gotSev Severity
	custom := canvas.NewRectangle(nil)
	g.MarkerFunc = func(sev Severity, _ geo.Size, _ float32) []core.CanvasObject {
		gotSev = sev
		return []core.CanvasObject{custom}
	}

	objs := g.Render(1, geo.NewSize(g.Width(10, 13), 18), 13)
	if len(objs) != 1 || objs[0] != custom {
		t.Fatalf("MarkerFunc result not used: got %v", objs)
	}
	if gotSev != SeverityError {
		t.Errorf("MarkerFunc severity = %v, want %v", gotSev, SeverityError)
	}
}

func TestDiagnosticGutter_ClickToRevealOptIn(t *testing.T) {
	g := NewDiagnosticGutter()
	diag := Diagnostic{Range: TextRange{StartRow: 0, StartCol: 1, EndRow: 0, EndCol: 4}, Severity: SeverityError}
	g.SetDiagnostics([]Diagnostic{diag})

	// Default: non-interactive — Tapped is a no-op even on a diagnostic line.
	g.Tapped(1) // must not panic with RevealOnTap nil

	var revealed []TextRange
	g.RevealOnTap = func(r TextRange) { revealed = append(revealed, r) }

	g.Tapped(1) // line 1 carries the diagnostic
	g.Tapped(2) // no diagnostic → no reveal
	if len(revealed) != 1 {
		t.Fatalf("want 1 reveal (only the diagnostic line), got %d", len(revealed))
	}
	if revealed[0] != diag.Range {
		t.Errorf("revealed range = %+v, want %+v", revealed[0], diag.Range)
	}
}

// TestDiagnosticGutter_RevealTargetsMostSevere verifies click-to-reveal jumps to
// the most-severe diagnostic on a line — the one the marker represents — even when
// a less-severe one appears first in set order, with ties broken by set order (#97).
func TestDiagnosticGutter_RevealTargetsMostSevere(t *testing.T) {
	g := NewDiagnosticGutter()
	hint := Diagnostic{Range: TextRange{StartRow: 0, StartCol: 0, EndRow: 0, EndCol: 2}, Severity: SeverityHint}
	errRange := TextRange{StartRow: 0, StartCol: 4, EndRow: 0, EndCol: 7}
	err := Diagnostic{Range: errRange, Severity: SeverityError}
	// Hint listed before the error, both covering line 1.
	g.SetDiagnostics([]Diagnostic{hint, err})

	var revealed []TextRange
	g.RevealOnTap = func(r TextRange) { revealed = append(revealed, r) }
	g.Tapped(1)
	if len(revealed) != 1 || revealed[0] != errRange {
		t.Fatalf("reveal should target the most-severe (error) range %+v, got %+v", errRange, revealed)
	}

	// Tie on severity: two errors → the first in set order wins.
	firstErr := TextRange{StartRow: 1, StartCol: 0, EndRow: 1, EndCol: 3}
	g.SetDiagnostics([]Diagnostic{
		{Range: firstErr, Severity: SeverityError},
		{Range: TextRange{StartRow: 1, StartCol: 5, EndRow: 1, EndCol: 9}, Severity: SeverityError},
	})
	revealed = nil
	g.Tapped(2)
	if len(revealed) != 1 || revealed[0] != firstErr {
		t.Errorf("severity tie should reveal the first in set order %+v, got %+v", firstErr, revealed)
	}
}

// TestDiagnosticGutter_CycleOnTap verifies repeated taps of a multi-diagnostic
// line walk through its diagnostics (most-severe first, then set order, wrapping)
// when CycleOnTap is set, and that a different line restarts the walk (#99).
func TestDiagnosticGutter_CycleOnTap(t *testing.T) {
	g := NewDiagnosticGutter()
	errR := TextRange{StartRow: 0, StartCol: 0, EndRow: 0, EndCol: 3}
	warnR := TextRange{StartRow: 0, StartCol: 5, EndRow: 0, EndCol: 8}
	hintR := TextRange{StartRow: 0, StartCol: 10, EndRow: 0, EndCol: 12}
	otherR := TextRange{StartRow: 1, StartCol: 0, EndRow: 1, EndCol: 2}
	// Set order deliberately not severity order; all of err/warn/hint cover line 1.
	diags := []Diagnostic{
		{Range: warnR, Severity: SeverityWarning},
		{Range: errR, Severity: SeverityError},
		{Range: hintR, Severity: SeverityHint},
		{Range: otherR, Severity: SeverityError},
	}

	var revealed []TextRange
	g.RevealOnTap = func(r TextRange) { revealed = append(revealed, r) }

	// Cycling off (default): every tap reveals the most severe (error).
	g.SetDiagnostics(diags)
	g.Tapped(1)
	g.Tapped(1)
	if len(revealed) != 2 || revealed[0] != errR || revealed[1] != errR {
		t.Fatalf("cycling off: every tap should reveal most-severe %+v, got %+v", errR, revealed)
	}

	// Cycling on: walk most-severe first, then by severity (wrapping).
	g.CycleOnTap = true
	g.SetDiagnostics(diags) // resets the cycle
	revealed = nil
	for i := 0; i < 4; i++ {
		g.Tapped(1)
	}
	want := []TextRange{errR, warnR, hintR, errR} // 4th wraps to most-severe
	if len(revealed) != 4 {
		t.Fatalf("cycling on: want 4 reveals, got %d (%+v)", len(revealed), revealed)
	}
	for i, w := range want {
		if revealed[i] != w {
			t.Errorf("cycle step %d = %+v, want %+v", i, revealed[i], w)
		}
	}

	// Tapping a different line restarts the walk at that line's most severe...
	g.SetDiagnostics(diags)
	revealed = nil
	g.Tapped(1) // errR (line 1 most severe)
	g.Tapped(1) // warnR (advance)
	g.Tapped(2) // otherR (different line → index 0)
	g.Tapped(1) // back to line 1 → restart at errR, not hintR
	want = []TextRange{errR, warnR, otherR, errR}
	for i, w := range want {
		if revealed[i] != w {
			t.Errorf("cross-line step %d = %+v, want %+v", i, revealed[i], w)
		}
	}
}

func TestEditorDiagnosticGutter_ClickToRevealWiring(t *testing.T) {
	test.NewTempApp(t)
	e := NewEditor()
	e.SetText("abc\ndefg")
	e.SetDiagnostics(sampleDiagnostics()) // error on line 1, warning on line 2

	g := e.DiagnosticGutter()
	if g.RevealOnTap != nil {
		t.Fatal("click-to-reveal must be off by default")
	}

	e.SetDiagnosticClickToReveal(true)
	if g.RevealOnTap == nil {
		t.Fatal("enabling click-to-reveal must wire RevealOnTap")
	}
	g.Tapped(1) // reveals via Editor.RevealRange; selects the line-1 diagnostic ("abc")
	if got := e.SelectedText(); got != "abc" {
		t.Errorf("tapping a diagnostic line should select its range; SelectedText = %q, want %q", got, "abc")
	}

	e.SetDiagnosticClickToReveal(false)
	if g.RevealOnTap != nil {
		t.Error("disabling click-to-reveal must clear RevealOnTap")
	}
}
