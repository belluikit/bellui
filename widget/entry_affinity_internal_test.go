package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/bellui/test"
)

// affinityText has an LTR→RTL boundary at logical column 4 (after "abc ").
const affinityText = "abc مرحبا def"

// TestEntry_AffinityChangesCaretX proves the cursorUpstream flag is plumbed into
// the rendered caret position: at a direction boundary the upstream and
// downstream affinities map column 4 to two different x positions.
func TestEntry_AffinityChangesCaretX(t *testing.T) {
	lang.SetLocaleForTests(t, "en-US")
	test.NewTempApp(t)

	e := NewEntry()
	e.SetText(affinityText)
	e.CursorColumn = 4 // the LTR→RTL boundary

	e.cursorUpstream = false
	down := e.CursorPosition().X
	e.cursorUpstream = true
	up := e.CursorPosition().X

	assert.NotEqual(t, down, up, "affinity must change the caret x at a boundary")
	assert.Less(t, up, down, "upstream caret should sit left of downstream at this boundary")

	// A non-boundary column is unaffected by affinity.
	e.CursorColumn = 2
	e.cursorUpstream = false
	a := e.CursorPosition().X
	e.cursorUpstream = true
	b := e.CursorPosition().X
	assert.Equal(t, a, b, "affinity must not move the caret away from a boundary")
}

// TestEntry_TypingSetsUpstreamAffinity confirms inserting a character leaves the
// caret with upstream affinity so it hugs the just-typed glyph.
func TestEntry_TypingSetsUpstreamAffinity(t *testing.T) {
	lang.SetLocaleForTests(t, "en-US")
	test.NewTempApp(t)

	e := NewEntry()
	e.SetText("abc")
	e.CursorColumn = 3
	e.cursorUpstream = false

	e.TypedRune('d')
	assert.True(t, e.cursorUpstream, "typing should set upstream affinity")
}

// TestEntry_ArrowResetsAffinity confirms a navigation key clears a stale upstream
// affinity back to the forward default.
func TestEntry_ArrowResetsAffinity(t *testing.T) {
	lang.SetLocaleForTests(t, "en-US")
	test.NewTempApp(t)

	e := NewEntry()
	e.SetText(affinityText)
	e.CursorColumn = 0
	e.cursorUpstream = true // simulate a leftover upstream affinity

	e.TypedKey(&event.KeyEvent{Name: event.KeyRight})
	assert.False(t, e.cursorUpstream, "arrow navigation should reset to downstream affinity")
}

// TestEntry_ClickAffinityFromSide sweeps clicks across the boundary and confirms
// that hit-testing resolves both affinities for the same logical column: clicks
// landing on the seam (left) side of column 4 report upstream affinity, while
// clicks on the RTL (right) side report downstream. Reading cursorUpstream
// directly avoids reconstructing the entry's coordinate transforms.
func TestEntry_ClickAffinityFromSide(t *testing.T) {
	lang.SetLocaleForTests(t, "en-US")
	test.NewApp()
	t.Cleanup(func() { test.NewApp() })

	e := NewEntry()
	e.Wrapping = bellui.TextWrapOff
	e.SetText(affinityText)
	w := test.NewTempWindow(t, e)
	w.Resize(geo.NewSize(420, 80))
	e.Resize(e.MinSize().Max(geo.NewSize(380, 0)))
	e.Move(geo.NewPos(10, 10))

	sawUpstream, sawDownstream := false, false
	for x := float32(10); x < 360; x += 2 {
		e.MouseDown(&desktop.MouseEvent{PointEvent: event.PointEvent{Position: geo.NewPos(x, 12)}})
		if e.CursorColumn == 4 {
			if e.cursorUpstream {
				sawUpstream = true
			} else {
				sawDownstream = true
			}
		}
	}

	assert.True(t, sawUpstream, "a click on the seam side of the boundary should set upstream affinity")
	assert.True(t, sawDownstream, "a click on the RTL side of the boundary should set downstream affinity")
}
