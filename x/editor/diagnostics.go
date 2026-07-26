package editor

import "github.com/belluikit/bellui"

// Severity classifies a [Diagnostic]. The values mirror the common LSP
// diagnostic severities (most to least severe) but carry no LSP dependency.
type Severity int

const (
	// SeverityError marks a problem that prevents correct operation.
	SeverityError Severity = iota
	// SeverityWarning marks a likely problem that does not prevent operation.
	SeverityWarning
	// SeverityInfo marks an informational message.
	SeverityInfo
	// SeverityHint marks a subtle suggestion (often rendered faintly).
	SeverityHint
)

// Diagnostic is a single problem reported over a document range. It is
// LSP-shaped but LSP-free: a host can produce diagnostics from a language
// server, a compiler, a linter, or its own validation.
type Diagnostic struct {
	// Range is the character range the diagnostic applies to.
	Range TextRange
	// Severity selects the squiggle/marker color.
	Severity Severity
	// Message is the human-readable problem description (shown on hover).
	Message string
	// Source names the producer, e.g. "protolsp" or "protocompile".
	Source string
}

// SetDiagnostics replaces the editor's diagnostic set, refreshing the squiggle
// overlay and firing the [Editor.OnDiagnosticsChanged] callback. It is
// idempotent and cheap to call on every publish-diagnostics notification;
// passing nil (or an empty slice) clears all diagnostics.
func (e *Editor) SetDiagnostics(diags []Diagnostic) {
	e.diagnostics.diags = diags
	e.diagGutter.SetDiagnostics(diags)
	e.RefreshDecorators()
	e.RefreshGutter()
	if e.onDiagnosticsChanged != nil {
		e.onDiagnosticsChanged(diags)
	}
}

// DiagnosticGutter returns the editor's internal [DiagnosticGutter], kept in
// sync with the diagnostic set on every [Editor.SetDiagnostics]. Compose it into
// the gutter to show severity markers, e.g.:
//
//	e.Gutter = NewCompositeGutter(NewLineNumberGutter(), e.DiagnosticGutter())
func (e *Editor) DiagnosticGutter() *DiagnosticGutter {
	return e.diagGutter
}

// SetDiagnosticClickToReveal toggles click-to-reveal on the editor's internal
// [DiagnosticGutter] (#57). When enabled, tapping a gutter line that carries a
// diagnostic scrolls to and selects the first diagnostic covering that line via
// [Editor.RevealRange]. It is disabled by default, keeping the gutter
// non-interactive so it composes alongside an interactive [BreakpointGutter].
func (e *Editor) SetDiagnosticClickToReveal(enabled bool) {
	if enabled {
		e.diagGutter.RevealOnTap = e.RevealRange
	} else {
		e.diagGutter.RevealOnTap = nil
	}
}

// Diagnostics returns the current diagnostic set (the slice last passed to
// [Editor.SetDiagnostics]).
func (e *Editor) Diagnostics() []Diagnostic {
	return e.diagnostics.diags
}

// OnDiagnosticsChanged installs a callback fired whenever the diagnostic set is
// replaced via [Editor.SetDiagnostics] — the hook a host's problems panel uses
// to stay in sync. Pass nil to clear it.
func (e *Editor) OnDiagnosticsChanged(fn func([]Diagnostic)) {
	e.onDiagnosticsChanged = fn
}

// diagnosticSpanDecorator is the internal [SpanDecorator] that draws one
// severity-colored wavy underline per diagnostic. It is always installed; with
// no diagnostics it draws nothing.
type diagnosticSpanDecorator struct {
	diags []Diagnostic
}

// Spans implements [SpanDecorator].
func (d *diagnosticSpanDecorator) Spans() []DecoratedSpan {
	if len(d.diags) == 0 {
		return nil
	}
	spans := make([]DecoratedSpan, 0, len(d.diags))
	for _, dg := range d.diags {
		spans = append(spans, DecoratedSpan{
			Range:          pointWidened(dg.Range),
			Kind:           SpanUnderline,
			UnderlineStyle: UnderlineWavy,
			Color:          themeColor(severityColorName(dg.Severity)),
		})
	}
	return spans
}

// pointWidened widens a zero-width (point) range to a single cell so a point
// diagnostic — e.g. an LSP "expected ')'" anchored at one position — draws a
// visible squiggle and stays hoverable. Non-empty ranges pass through unchanged.
func pointWidened(r TextRange) TextRange {
	if !r.IsEmpty() {
		return r
	}
	return TextRange{StartRow: r.StartRow, StartCol: r.StartCol, EndRow: r.StartRow, EndCol: r.StartCol + 1}
}

// hasDiagnostics reports whether any diagnostics are currently set.
func (e *Editor) hasDiagnostics() bool {
	return e.diagnostics != nil && len(e.diagnostics.diags) > 0
}

// diagnosticsAt returns the diagnostics whose range covers (row, col), in the
// order they were set. Point ranges are widened (see [pointWidened]) so a point
// diagnostic is hoverable at its anchor cell.
func (e *Editor) diagnosticsAt(row, col int) []Diagnostic {
	if e.diagnostics == nil {
		return nil
	}
	var out []Diagnostic
	for _, dg := range e.diagnostics.diags {
		if pointWidened(dg.Range).Contains(row, col) {
			out = append(out, dg)
		}
	}
	return out
}

// severityColorName maps a [Severity] to its theme color name.
func severityColorName(s Severity) bellui.ThemeColorName {
	switch s {
	case SeverityWarning:
		return ColorNameDiagnosticWarning
	case SeverityInfo:
		return ColorNameDiagnosticInfo
	case SeverityHint:
		return ColorNameDiagnosticHint
	default:
		return ColorNameDiagnosticError
	}
}
