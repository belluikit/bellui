package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
)

// TestEntryRenderer_RoundedClip pins the #467 fix: the entry's renderer
// exposes its background radius through the rounded-clip seam, so the GPU
// paint walk clips the subtree (notably the scroll bar's square strip) to
// the entry's rounded outline.
func TestEntryRenderer_RoundedClip(t *testing.T) {
	test.NewTempApp(t)

	e := NewMultiLineEntry()
	w := test.NewWindow(e)
	defer w.Close()

	r, ok := cache.Renderer(e).(interface{ RoundedClipCornerRadius() float32 })
	require.True(t, ok, "entryRenderer implements the rounded clipping interface")
	assert.Equal(t, e.Theme().Size(theme.SizeNameInputRadius)+e.Theme().Size(theme.SizeNameInputBorder),
		r.RoundedClipCornerRadius(),
		"clip radius is concentric with the inset background box (radius + border)")
	assert.Greater(t, r.RoundedClipCornerRadius(), float32(0),
		"the default theme rounds entries, so the clip is active")
}
