package editor

import (
	"fmt"
	"strings"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
)

// Accessibility model: the Editor deliberately does NOT implement
// [core.Accessible]. Its embedded Entry already carries the full desktop text
// protocol — [core.Accessible] with RoleMultiLineTextBox plus
// [core.AccessibleTexter] (whole-document read, caret/selection tracking, and
// voice edits through the gated, undo-recorded paths) — and the fyne semantic
// walker descends widget renderers, so the Entry surfaces as the editor's one
// text-box node. A role on the wrapper would nest a second text box around it
// and screen readers would announce the document twice.
//
// The pieces fycoder adds on top:
//   - [Editor.SetAccessibilityLabel] names that node (e.g. the file name).
//   - [Editor.SetDiagnostics] announces diagnostic changes via the
//     live-region path (bellui.AnnounceToScreenReader).
//
// Lock in that the entry keeps satisfying the text protocol by embedding.
var (
	_ core.Accessible         = (*editorEntry)(nil)
	_ core.AccessibleTexter   = (*editorEntry)(nil)
	_ core.AccessibleStateful = (*editorEntry)(nil)
)

// SetAccessibilityLabel names the editor's document for assistive technology
// — typically the file name (e.g. "main.proto"). The name is spoken by screen
// readers when the editor gains focus. It shadows the embedded BaseWidget
// method to target the Entry, which is the node the semantic tree exposes;
// a label set on the wrapper itself would never be read.
func (e *Editor) SetAccessibilityLabel(label string) {
	e.entry.SetAccessibilityLabel(label)
}

// AccessibilityLabel returns the label set via [Editor.SetAccessibilityLabel].
func (e *Editor) AccessibilityLabel() string {
	return e.entry.AccessibilityLabel()
}

// announceDiagnostics pushes a transient live-region summary ("1 error, 2
// warnings") to a running screen reader when the diagnostic picture changes.
// Announcements are polite (non-interrupting): diagnostics republish on every
// keystroke in LSP-driven hosts, and barging in over the typed-character echo
// would make the editor unusable with a screen reader. Clearing a previously
// announced set says "No problems"; an editor that never had diagnostics
// stays silent.
func (e *Editor) announceDiagnostics(diags []Diagnostic) {
	summary := diagnosticsSummary(diags)
	if summary == e.lastDiagSummary {
		return
	}
	e.lastDiagSummary = summary
	msg := summary
	if msg == "" {
		msg = "No problems"
	}
	if e.announce != nil {
		e.announce(msg, false)
		return
	}
	bellui.AnnounceToScreenReader(msg, false)
}

// diagnosticsSummary renders a diagnostic set as a short spoken summary,
// listing only the severities present ("2 errors, 1 hint"). Empty set = "".
func diagnosticsSummary(diags []Diagnostic) string {
	var counts [4]int
	for _, d := range diags {
		if d.Severity >= SeverityError && d.Severity <= SeverityHint {
			counts[d.Severity]++
		}
	}
	names := [4][2]string{
		{"error", "errors"},
		{"warning", "warnings"},
		{"info message", "info messages"},
		{"hint", "hints"},
	}
	var parts []string
	for sev, n := range counts {
		if n == 0 {
			continue
		}
		word := names[sev][0]
		if n > 1 {
			word = names[sev][1]
		}
		parts = append(parts, fmt.Sprintf("%d %s", n, word))
	}
	return strings.Join(parts, ", ")
}
