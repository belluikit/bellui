package widget

import (
	"bytes"
	"log"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/test"
)

// richTextRefreshes counts the RichText refreshes fn causes, from the
// DebugRefresh log.
func richTextRefreshes(t *testing.T, fn func()) int {
	t.Helper()
	var buf bytes.Buffer
	prevOut, prevFlags := log.Writer(), log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	DebugRefresh = true
	defer func() {
		DebugRefresh = false
		log.SetOutput(prevOut)
		log.SetFlags(prevFlags)
	}()
	fn()
	return strings.Count(buf.String(), "[refresh] *widget.RichText")
}

// TestLabelSetTextRefreshesRichTextOnce pins #488: SetText refreshes the
// label's RichText once, not once in SyncState and again in the renderer.
func TestLabelSetTextRefreshesRichTextOnce(t *testing.T) {
	test.NewTempApp(t)
	l := NewLabel("before")
	cache.Renderer(l)

	assert.Equal(t, 1, richTextRefreshes(t, func() { l.SetText("after") }))
	assert.Equal(t, "after", l.provider.String())
}

// TestLabelRendererRefreshAloneSyncs checks the other path: a renderer
// refreshed directly, without BaseWidget.Refresh, still syncs and refreshes.
func TestLabelRendererRefreshAloneSyncs(t *testing.T) {
	test.NewTempApp(t)
	l := NewLabel("before")
	r := cache.Renderer(l)

	l.Text = "direct"
	assert.Equal(t, 1, richTextRefreshes(t, r.Refresh))
	assert.Equal(t, "direct", l.provider.String())
}

// TestEntrySetTextRefreshesTextOnce pins the same for Entry: its text RichText
// is refreshed once per SetText (the placeholder's refresh is separate).
func TestEntrySetTextRefreshesTextOnce(t *testing.T) {
	test.NewTempApp(t)
	e := NewEntry()
	e.SetText("before")
	cache.Renderer(e)

	assert.Equal(t, 2, richTextRefreshes(t, func() { e.SetText("after") }), "one text and one placeholder refresh")
	assert.Equal(t, "after", e.text.String())
}
