package editor

import (
	"sort"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
)

// DiagnosticGutter renders a severity marker in the gutter for each line that
// carries a diagnostic, mirroring [BreakpointGutter] and composing with the
// line-number gutter via [NewCompositeGutter]. A line's marker takes the color —
// and shape — of the most severe diagnostic on it. Each severity has a distinct
// glyph (filled circle = error, triangle = warning, square = info, diamond =
// hint) so the four severities are distinguishable without relying on color (#56).
//
// By default it is non-interactive: jumping to a problem is the problems-list →
// [Editor.RevealRange] path, not a gutter click. Set [DiagnosticGutter.RevealOnTap]
// (or call [Editor.SetDiagnosticClickToReveal]) to make a tapped marker reveal
// the most-severe diagnostic covering that line (#57, #97); enable
// [DiagnosticGutter.CycleOnTap] to walk through a line's other diagnostics on
// repeated taps (#99).
//
// An editor keeps an internal DiagnosticGutter fed automatically from
// [Editor.SetDiagnostics] — obtain it via [Editor.DiagnosticGutter] and drop it
// into your CompositeGutter. It can also be driven standalone via
// [DiagnosticGutter.SetDiagnostics].
type DiagnosticGutter struct {
	// severities maps a 1-based line number to the most severe diagnostic on it.
	severities map[int]Severity
	// lineRanges maps a 1-based line number to the ranges of every diagnostic
	// covering it, ordered most-severe first then by set order — index 0 is the
	// marker's diagnostic and the default reveal target; later entries are the
	// CycleOnTap walk order.
	lineRanges map[int][]TextRange
	// tapLine and tapIndex track the last revealed (line, index) so repeated taps
	// on the same line advance through lineRanges when CycleOnTap is set. tapLine
	// is 0 (no 1-based line) when no cycle is in progress.
	tapLine, tapIndex int

	// DotScale controls the marker diameter relative to text size.
	// Defaults to 0.6 when zero (matching BreakpointGutter).
	DotScale float32

	// MarkerFunc, when non-nil, overrides the default per-severity marker. It is
	// called with the most severe severity on a line, the allocated cell size, and
	// the current text size, and returns the canvas objects to draw (nil draws
	// nothing). Lets a host supply its own glyphs or icons.
	MarkerFunc func(sev Severity, size geo.Size, textSize float32) []core.CanvasObject

	// RevealOnTap, when non-nil, makes the gutter interactive: tapping a line that
	// carries a diagnostic calls it with the range of the most-severe diagnostic
	// covering that line — the one the marker represents. When nil (the default)
	// taps are ignored, so the gutter composes alongside an interactive
	// [BreakpointGutter] without stealing its taps.
	RevealOnTap func(rng TextRange)

	// CycleOnTap, when true, makes repeated taps on the same line walk through all
	// diagnostics covering it (most-severe first, then set order, wrapping) instead
	// of always revealing the most severe. The first tap of a line still reveals
	// the most severe, and tapping a different line restarts the walk. No effect
	// unless RevealOnTap is set; off by default (#99).
	CycleOnTap bool
}

// NewDiagnosticGutter creates an empty diagnostic gutter.
func NewDiagnosticGutter() *DiagnosticGutter {
	return &DiagnosticGutter{severities: map[int]Severity{}, lineRanges: map[int][]TextRange{}}
}

// SetDiagnostics rebuilds the per-line maps from diags. Each line a diagnostic's
// range covers collects that diagnostic; the marker takes the most severe, and
// the per-line reveal order is most-severe first then set order — index 0 (the
// marker's diagnostic) is the default reveal target (#97) and the rest is the
// [DiagnosticGutter.CycleOnTap] walk order (#99). Passing nil clears all markers.
func (g *DiagnosticGutter) SetDiagnostics(diags []Diagnostic) {
	// Collect, per line, every covering diagnostic in set order.
	perLine := map[int][]lineDiag{}
	for _, d := range diags {
		endRow := d.Range.EndRow
		// A half-open range ending at column 0 of a later row does not touch
		// that row's text, so it should not mark it.
		if endRow > d.Range.StartRow && d.Range.EndCol == 0 {
			endRow--
		}
		for row := d.Range.StartRow; row <= endRow; row++ {
			line := row + 1 // gutter lines are 1-based
			perLine[line] = append(perLine[line], lineDiag{sev: d.Severity, rng: d.Range})
		}
	}

	m := make(map[int]Severity, len(perLine))
	ranges := make(map[int][]TextRange, len(perLine))
	for line, lds := range perLine {
		// Stable sort by severity (Error == 0 is most severe) keeps set order
		// within a severity, so index 0 is the marker's diagnostic.
		sort.SliceStable(lds, func(i, j int) bool { return lds[i].sev < lds[j].sev })
		rs := make([]TextRange, len(lds))
		for i, ld := range lds {
			rs[i] = ld.rng
		}
		m[line] = lds[0].sev
		ranges[line] = rs
	}
	g.severities = m
	g.lineRanges = ranges
	// The set changed: restart any in-progress cycle.
	g.tapLine, g.tapIndex = 0, 0
}

// lineDiag is a diagnostic's severity and range, collected per line before
// ordering for the marker and the reveal/cycle walk.
type lineDiag struct {
	sev Severity
	rng TextRange
}

func (g *DiagnosticGutter) dotScale() float32 {
	if g.DotScale > 0 {
		return g.DotScale
	}
	return 0.6
}

// Width implements [GutterProvider]. The marker column is sized to fit a dot
// with padding, matching the breakpoint column.
func (g *DiagnosticGutter) Width(_ int, textSize float32) float32 {
	return textSize * g.dotScale() * 1.6
}

// Render implements [GutterProvider]. line is 1-based. It returns a
// severity-shaped, severity-colored marker when that line carries a diagnostic,
// else nothing. A non-nil [DiagnosticGutter.MarkerFunc] replaces the default.
func (g *DiagnosticGutter) Render(line int, size geo.Size, textSize float32) []core.CanvasObject {
	sev, ok := g.severities[line]
	if !ok {
		return nil
	}
	if g.MarkerFunc != nil {
		return g.MarkerFunc(sev, size, textSize)
	}
	return severityMarker(sev, size, textSize, g.dotScale())
}

// severityMarker builds the default gutter marker: a distinct shape per severity
// (filled circle = error, triangle = warning, square = info, diamond = hint) so
// the four severities read apart without relying on color (#56). The default
// 4-gon orientation is a diamond (first vertex at top), so the square is rotated
// 45°. All shapes share the same diameter and are centered in the cell.
func severityMarker(sev Severity, size geo.Size, textSize, scale float32) []core.CanvasObject {
	d := textSize * scale
	col := themeColor(severityColorName(sev))
	var obj core.CanvasObject
	switch sev {
	case SeverityWarning:
		obj = canvas.NewPolygon(3, col) // upright triangle
	case SeverityInfo:
		sq := canvas.NewPolygon(4, col)
		sq.Angle = 45 // axis-aligned square
		obj = sq
	case SeverityHint:
		obj = canvas.NewPolygon(4, col) // diamond
	default: // SeverityError — most severe, the familiar filled dot
		obj = canvas.NewCircle(col)
	}
	obj.Resize(geo.NewSize(d, d))
	obj.Move(geo.NewPos((size.Width-d)/2, (size.Height-d)/2))
	return []core.CanvasObject{obj}
}

// Tapped implements [InteractiveGutter]. When [DiagnosticGutter.RevealOnTap] is
// set and the tapped line (1-based) carries a diagnostic, it reveals the
// most-severe diagnostic covering that line — the one the marker represents — or,
// with [DiagnosticGutter.CycleOnTap] set, the next diagnostic on a repeated tap of
// the same line (wrapping). Otherwise it is a no-op, leaving the gutter
// non-interactive by default.
func (g *DiagnosticGutter) Tapped(line int) {
	if g.RevealOnTap == nil {
		return
	}
	ranges, ok := g.lineRanges[line]
	if !ok || len(ranges) == 0 {
		return
	}
	idx := 0
	if g.CycleOnTap && line == g.tapLine {
		idx = (g.tapIndex + 1) % len(ranges)
	}
	g.tapLine, g.tapIndex = line, idx
	g.RevealOnTap(ranges[idx])
}
