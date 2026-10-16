package editor

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
)

// TestHoverCapture_LeavesScrollBarStrips pins the pre-rebrand tracker #151 fix: the transparent
// hover-capture layer is inset by the scroll bar strips (trailing + bottom),
// so the Entry's scroll bar underneath still receives the hover that expands
// it to its grabbable width — fyne delivers hover to the topmost hoverable,
// and a full-size capture layer starved the bar permanently.
func TestHoverCapture_LeavesScrollBarStrips(t *testing.T) {
	test.NewTempApp(t)
	e := NewEditor()
	e.SetText(strings.Repeat("a long line of editor text\n", 200))
	w := test.NewWindow(e)
	defer w.Close()
	w.Resize(geo.NewSize(400, 300))

	require.NotNil(t, e.hoverCap)
	bar := theme.ScrollBarSize()

	// The entry and the capture layer's wrapper share the same stack, so the
	// entry's size is the full center area; the capture layer must be inset
	// by exactly one bar width on the trailing and bottom edges.
	assert.Equal(t, geo.Position{}, e.hoverCap.Position())
	assert.Equal(t, e.entry.Size().Width-bar, e.hoverCap.Size().Width)
	assert.Equal(t, e.entry.Size().Height-bar, e.hoverCap.Size().Height)
}
