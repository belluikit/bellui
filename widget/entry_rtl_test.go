package widget_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
)

// TestEntry_CursorPosition_RTL verifies that the caret pixel position is mapped
// through the bidi visual layout: across the Arabic span the caret x is not
// monotonic in the logical column, whereas a pure-LTR string stays monotonic.
func TestEntry_CursorPosition_RTL(t *testing.T) {
	lang.SetLocaleForTests(t, "en-US")

	entry := widget.NewEntry()
	entry.SetText("abc مرحبا def")
	n := len([]rune(entry.Text))

	xs := make([]float32, n+1)
	for c := 0; c <= n; c++ {
		entry.CursorColumn = c
		xs[c] = entry.CursorPosition().X
	}

	monotonic := true
	for i := 1; i <= n; i++ {
		if xs[i] < xs[i-1] {
			monotonic = false
			break
		}
	}
	assert.False(t, monotonic, "RTL caret x must be non-monotonic across the Arabic span: %v", xs)

	// The Arabic span is logical offsets 4..8 and reads right-to-left, so the
	// caret x for its interior offsets strictly decreases as the logical column
	// increases (5 is visually right of 6, etc.).
	for c := 5; c < 8; c++ {
		assert.Greater(t, xs[c], xs[c+1],
			"Arabic interior caret %d must be visually right of %d (RTL): %v", c, c+1, xs)
	}

	// A pure-LTR string keeps a monotonic caret.
	entry.SetText("abc def ghi")
	m := len([]rune(entry.Text))
	prev := float32(-1)
	for c := 0; c <= m; c++ {
		entry.CursorColumn = c
		x := entry.CursorPosition().X
		assert.GreaterOrEqual(t, x, prev, "LTR caret must be monotonic at column %d", c)
		prev = x
	}
}

// TestEntry_ClickPosition_RTL verifies that click-to-position hit-tests through
// the visual layout: sweeping clicks left-to-right across mixed text yields a
// non-monotonic column sequence (rising through the leading LTR text, falling
// through the reversed Arabic span, rising again).
func TestEntry_ClickPosition_RTL(t *testing.T) {
	lang.SetLocaleForTests(t, "en-US")
	test.NewApp()
	t.Cleanup(func() { test.NewApp() })

	entry := widget.NewEntry()
	entry.Wrapping = bellui.TextWrapOff
	entry.Scroll = container.ScrollNone
	entry.SetText("abc مرحبا def")
	w := test.NewTempWindow(t, entry)
	w.Resize(geo.NewSize(320, 80))
	entry.Resize(entry.MinSize().Max(geo.NewSize(260, 0)))
	entry.Move(geo.NewPos(10, 10))

	var cols []int
	for x := float32(10); x < 200; x += 5 {
		entry.MouseDown(&desktop.MouseEvent{PointEvent: event.PointEvent{Position: geo.NewPos(x, 12)}})
		cols = append(cols, entry.CursorColumn)
	}

	monotonic := true
	for i := 1; i < len(cols); i++ {
		if cols[i] < cols[i-1] {
			monotonic = false
			break
		}
	}
	assert.False(t, monotonic, "clicking across RTL text must map columns non-monotonically: %v", cols)
}

// TestEntry_ArrowKeys_RTLVisual verifies that the Left/Right arrows move the
// caret in visual order: pressing Right repeatedly across mixed text walks the
// caret strictly rightward (monotonic pixel x) even though the logical columns
// jump around inside the reversed Arabic span.
func TestEntry_ArrowKeys_RTLVisual(t *testing.T) {
	lang.SetLocaleForTests(t, "en-US")

	entry := widget.NewEntry()
	entry.SetText("abc مرحبا def")
	n := len([]rune(entry.Text))

	entry.CursorColumn = 0
	right := &event.KeyEvent{Name: event.KeyRight}

	prevX := entry.CursorPosition().X
	var cols []int
	for i := 0; i < n; i++ {
		entry.TypedKey(right)
		cols = append(cols, entry.CursorColumn)
		x := entry.CursorPosition().X
		assert.GreaterOrEqual(t, x, prevX,
			"Right arrow must move the caret visually rightward (step %d, cols=%v)", i, cols)
		prevX = x
	}
	// The visited logical columns are not in ascending order (the Arabic span is
	// traversed right-to-left), proving the movement is visual, not logical.
	ascending := true
	for i := 1; i < len(cols); i++ {
		if cols[i] < cols[i-1] {
			ascending = false
			break
		}
	}
	assert.False(t, ascending, "logical columns visited by Right should not be ascending in RTL: %v", cols)
}

// TestEntry_MultiRow_RTL verifies the caret mapping works on a non-zero row:
// a multi-line entry whose second row mixes directions resolves that row's bidi
// independently (the per-row visual layout is keyed on the row's own runes).
func TestEntry_MultiRow_RTL(t *testing.T) {
	lang.SetLocaleForTests(t, "en-US")

	entry := widget.NewMultiLineEntry()
	entry.SetText("hello\nabc مرحبا def")
	entry.CursorRow = 1

	row := []rune("abc مرحبا def")
	xs := make([]float32, len(row)+1)
	for c := 0; c <= len(row); c++ {
		entry.CursorColumn = c
		xs[c] = entry.CursorPosition().X
	}

	monotonic := true
	for i := 1; i <= len(row); i++ {
		if xs[i] < xs[i-1] {
			monotonic = false
			break
		}
	}
	assert.False(t, monotonic, "RTL caret on row 1 must be non-monotonic: %v", xs)
}
