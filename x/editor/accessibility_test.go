package editor

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// accessibleNodes walks the editor's rendered tree the way the driver's
// semantic walker does — descending containers and widget renderers — and
// returns every accessible node found (#171).
func accessibleNodes(t *testing.T, root core.CanvasObject) []core.AccessibleNode {
	t.Helper()
	children := func(o core.CanvasObject) []core.CanvasObject {
		switch c := o.(type) {
		case *bellui.Container:
			return c.Objects
		case core.Widget:
			return test.TempWidgetRenderer(t, c).Objects()
		}
		return nil
	}
	var nodes []core.AccessibleNode
	core.WalkAccessibleTree(root, children, func(n core.AccessibleNode, _ int) bool {
		nodes = append(nodes, n)
		return false
	})
	return nodes
}

// textBoxNode returns the editor's single multi-line text-box node.
func textBoxNode(t *testing.T, e *Editor) core.AccessibleNode {
	t.Helper()
	var boxes []core.AccessibleNode
	for _, n := range accessibleNodes(t, e) {
		if n.Role == core.RoleMultiLineTextBox {
			boxes = append(boxes, n)
		}
	}
	require.Len(t, boxes, 1, "editor must expose exactly one multi-line text box")
	return boxes[0]
}

// TestAccessibility_EntrySurfacesAsTextBox verifies the walker reaches the
// embedded Entry through the Editor wrapper: the wrapper itself carries no
// role, and the one multi-line text-box node satisfies the full desktop text
// protocol over the whole document (#171).
func TestAccessibility_EntrySurfacesAsTextBox(t *testing.T) {
	test.NewTempApp(t)
	e := NewEditor()
	w := test.NewWindow(e)
	defer w.Close()
	w.Resize(geo.NewSize(400, 300))
	e.SetText("alpha\nbeta\ngamma")

	// The wrapper must not be accessible — a role here would nest a second
	// text box around the Entry and the document would be announced twice.
	_, ok := core.SnapshotAccessible(e)
	assert.False(t, ok, "Editor wrapper must not implement core.Accessible")

	node := textBoxNode(t, e)
	texter, ok := node.Object.(core.AccessibleTexter)
	require.True(t, ok, "text-box node must implement core.AccessibleTexter")
	assert.Equal(t, "alpha\nbeta\ngamma", texter.AccessibleText())
}

// TestAccessibility_TexterReadsWholeVirtualizedDocument verifies a screen
// reader reads and navigates the full document regardless of which lines are
// realized in the viewport — the reason #171 needs no per-line nodes.
func TestAccessibility_TexterReadsWholeVirtualizedDocument(t *testing.T) {
	test.NewTempApp(t)
	e := NewEditor()
	w := test.NewWindow(e)
	defer w.Close()
	w.Resize(geo.NewSize(400, 200)) // a handful of visible rows

	lines := make([]string, 5000)
	for i := range lines {
		lines[i] = "line"
	}
	doc := strings.Join(lines, "\n")
	e.SetText(doc)

	node := textBoxNode(t, e)
	texter := node.Object.(core.AccessibleTexter)
	assert.Equal(t, doc, texter.AccessibleText(),
		"whole document must be reachable as text, not just realized lines")
}

// TestAccessibility_TexterEditsThroughGatedPaths verifies selection, caret
// and voice edits route through the Entry's normal gated edit paths — and
// no-op when the editor is read-only.
func TestAccessibility_TexterEditsThroughGatedPaths(t *testing.T) {
	test.NewTempApp(t)
	e := NewEditor()
	w := test.NewWindow(e)
	defer w.Close()
	w.Resize(geo.NewSize(400, 300))
	e.SetText("alpha\nbeta")

	texter := textBoxNode(t, e).Object.(core.AccessibleTexter)

	texter.SetAccessibleSelection(0, 5)
	start, end := texter.AccessibleSelection()
	assert.Equal(t, 0, start)
	assert.Equal(t, 5, end)

	texter.ReplaceAccessibleSelection("delta")
	assert.Equal(t, "delta\nbeta", e.Text())
	assert.Equal(t, 5, texter.AccessibleCaret(), "caret sits after the replacement")

	e.SetReadOnly(true)
	texter.SetAccessibleSelection(0, 5)
	texter.ReplaceAccessibleSelection("nope")
	assert.Equal(t, "delta\nbeta", e.Text(), "read-only editor must reject voice edits")
}

// TestAccessibility_LabelDelegation verifies SetAccessibilityLabel names the
// node the semantic tree actually exposes (the Entry), not the silent wrapper.
func TestAccessibility_LabelDelegation(t *testing.T) {
	test.NewTempApp(t)
	e := NewEditor()
	w := test.NewWindow(e)
	defer w.Close()
	w.Resize(geo.NewSize(400, 300))

	e.SetAccessibilityLabel("main.proto")
	assert.Equal(t, "main.proto", e.AccessibilityLabel())
	assert.Equal(t, "main.proto", textBoxNode(t, e).Label)
}

// TestAccessibility_DiagnosticsAnnouncements verifies the live-region summary:
// announced on change only, polite, with a "No problems" clear — and silence
// for an editor that never had diagnostics.
func TestAccessibility_DiagnosticsAnnouncements(t *testing.T) {
	test.NewTempApp(t)
	e := NewEditor()
	e.SetText("abc\ndefg")

	var got []string
	var assertive []bool
	e.announce = func(text string, a bool) {
		got = append(got, text)
		assertive = append(assertive, a)
	}

	e.SetDiagnostics(nil)
	assert.Empty(t, got, "clearing a never-populated set must stay silent")

	diags := []Diagnostic{
		{Range: TextRange{StartRow: 0, EndRow: 0, EndCol: 3}, Severity: SeverityError, Message: "bad"},
		{Range: TextRange{StartRow: 1, StartCol: 1, EndRow: 1, EndCol: 4}, Severity: SeverityWarning, Message: "iffy"},
		{Range: TextRange{StartRow: 1, StartCol: 1, EndRow: 1, EndCol: 4}, Severity: SeverityWarning, Message: "also iffy"},
	}
	e.SetDiagnostics(diags)
	require.Equal(t, []string{"1 error, 2 warnings"}, got)
	assert.False(t, assertive[0], "diagnostics republish per keystroke; must not interrupt speech")

	e.SetDiagnostics(diags)
	assert.Len(t, got, 1, "idempotent republish must not re-announce")

	e.SetDiagnostics(nil)
	require.Equal(t, []string{"1 error, 2 warnings", "No problems"}, got)

	e.SetDiagnostics(nil)
	assert.Len(t, got, 2, "repeated clear must not re-announce")
}

func TestDiagnosticsSummary(t *testing.T) {
	assert.Equal(t, "", diagnosticsSummary(nil))
	assert.Equal(t, "1 error", diagnosticsSummary([]Diagnostic{{Severity: SeverityError}}))
	assert.Equal(t, "2 info messages, 1 hint", diagnosticsSummary([]Diagnostic{
		{Severity: SeverityInfo}, {Severity: SeverityHint}, {Severity: SeverityInfo},
	}))
	assert.Equal(t, "1 error, 1 warning, 1 info message, 1 hint", diagnosticsSummary([]Diagnostic{
		{Severity: SeverityHint}, {Severity: SeverityInfo}, {Severity: SeverityWarning}, {Severity: SeverityError},
	}))
}
