package widget_test

import (
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Compose tests for #149: ProtectedRanges (F3) and HiddenRanges (F4) set on the
// same Entry. Folding is pure view state and does not move rune offsets, so
// protection (rune offsets) still applies to text inside a folded region and the
// caret skips both folded and caret-opaque spans. These exercise the
// interactions that were only ever tested in isolation.
//
// Sample rows (wrap off, one logical line per row):
//
//	row0 alpha   [0,5)   \n5
//	row1 bravo   [6,11)  \n11
//	row2 charlie [12,19) \n19
//	row3 delta   [20,25) \n25
//	row4 echo    [26,30)
const phSample = "alpha\nbravo\ncharlie\ndelta\necho"

func newFoldEntry(t *testing.T) *widget.Entry {
	t.Helper()
	test.NewTempApp(t)
	e := widget.NewMultiLineEntry()
	e.Wrapping = bellui.TextWrapOff
	e.SetText(phSample)
	return e
}

// A ProtectedRange whose offsets fall entirely inside a HiddenRange still vetoes
// an edit touching that span, even though the row is folded away.
func TestEntry_Compose_ProtectedInsideHidden_EditVetoed(t *testing.T) {
	e := newFoldEntry(t)
	e.HiddenRanges = []widget.LineRange{{Start: 1, End: 2}}          // fold "bravo"
	e.ProtectedRanges = []widget.ProtectedRange{{Start: 6, End: 11}} // protect "bravo"
	e.Refresh()

	// Selecting the folded text reads it back (folding does not move offsets) ...
	e.Select(1, 0, 1, 5)
	assert.Equal(t, "bravo", e.SelectedText(), "selection includes folded text")

	// ... and overwriting it is vetoed by the protected range.
	e.TypedRune('X')
	assert.Equal(t, phSample, e.Text, "edit over a protected span inside a fold is vetoed")
}

// With the same fold+protection layout, an edit at the fold boundary that is
// outside the protected span still proceeds (a fresh entry avoids stale
// selection state from a prior vetoed edit).
func TestEntry_Compose_ProtectedInsideHidden_BoundaryEditAllowed(t *testing.T) {
	e := newFoldEntry(t)
	e.HiddenRanges = []widget.LineRange{{Start: 1, End: 2}}          // fold "bravo"
	e.ProtectedRanges = []widget.ProtectedRange{{Start: 6, End: 11}} // protect "bravo"
	e.Refresh()

	e.CursorRow, e.CursorColumn = 0, 5 // end of "alpha", just before the protected span
	e.TypedRune('Z')
	assert.Equal(t, "alphaZ\nbravo\ncharlie\ndelta\necho", e.Text,
		"an edit at the fold boundary outside the protected span is allowed")
}

// A CaretOpaque span overlapping a folded row: arrow nav (both skip helpers run
// back-to-back in TypedKey) converges to a visible, non-opaque caret position.
func TestEntry_Compose_CaretOpaqueOverFold_NavConverges(t *testing.T) {
	e := newFoldEntry(t)
	e.HiddenRanges = []widget.LineRange{{Start: 2, End: 3}}                              // fold "charlie"
	e.ProtectedRanges = []widget.ProtectedRange{{Start: 12, End: 19, CaretOpaque: true}} // opaque "charlie"
	e.Refresh()

	// From the row above the fold, arrow-down must clear both the folded row and
	// the caret-opaque span, landing on the next visible, non-opaque row.
	e.CursorRow, e.CursorColumn = 1, 0
	e.TypedKey(&event.KeyEvent{Name: event.KeyDown})

	assert.Equal(t, 3, e.CursorRow, "caret converges to the first visible row past the fold")
	pos := e.CursorTextOffset()
	assert.False(t, pos > 12 && pos < 19, "caret must not rest strictly inside the opaque span")
}

// A selection spanning a fold and a protected range: SelectedText includes the
// hidden text, and replacing that selection is vetoed.
func TestEntry_Compose_SelectionSpansFoldAndProtected_Vetoed(t *testing.T) {
	e := newFoldEntry(t)
	e.HiddenRanges = []widget.LineRange{{Start: 2, End: 3}}           // fold "charlie"
	e.ProtectedRanges = []widget.ProtectedRange{{Start: 20, End: 25}} // protect "delta"
	e.Refresh()

	// Select bravo..delta — the selection crosses the folded "charlie" row.
	e.Select(1, 0, 3, 5)
	assert.Equal(t, "bravo\ncharlie\ndelta", e.SelectedText(),
		"selection across a fold still includes the hidden text")

	e.TypedRune('!')
	assert.Equal(t, phSample, e.Text,
		"replacing a selection that overlaps a protected range is vetoed")
}

// Reconcile order: a SetText that invalidates both folds and protected offsets
// leaves a consistent state — no panic, and subsequent edits still behave.
func TestEntry_Compose_SetTextReconcilesBoth(t *testing.T) {
	e := newFoldEntry(t)
	e.HiddenRanges = []widget.LineRange{{Start: 1, End: 3}}
	e.ProtectedRanges = []widget.ProtectedRange{{Start: 12, End: 19}}
	e.Refresh()

	// Shrink the document well below all the stale offsets/rows.
	require.NotPanics(t, func() {
		e.SetText("x")
		e.Refresh()
	}, "SetText with stale fold and protected ranges must not panic")

	// Folds clamp to the single remaining row; an edit outside the (now
	// out-of-range) protected span still works.
	e.CursorRow, e.CursorColumn = 0, 0
	e.TypedRune('A')
	assert.Equal(t, "Ax", e.Text, "editing remains consistent after reconciliation")
}

// Guard: setting both features must not regress either one in isolation.
func TestEntry_Compose_NoIsolationRegression(t *testing.T) {
	e := newFoldEntry(t)
	e.HiddenRanges = []widget.LineRange{{Start: 1, End: 2}}
	e.ProtectedRanges = []widget.ProtectedRange{{Start: 6, End: 11}}
	e.Refresh()

	// Fold still collapses its row geometry.
	yPos, heights := e.RowPositions()
	require.GreaterOrEqual(t, len(heights), 5)
	assert.Equal(t, float32(0), heights[1], "folded row keeps zero height")
	assert.Equal(t, yPos[1], yPos[2], "row after the fold shifts up onto it")

	// Protection still allows edits elsewhere (no selection active).
	e.CursorRow, e.CursorColumn = 0, 2
	e.TypedRune('X')
	assert.Equal(t, "alXpha\nbravo\ncharlie\ndelta\necho", e.Text,
		"an unrelated edit is unaffected by either feature")
}
