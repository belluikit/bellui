package widget

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/test"
)

func TestEntryLineCount_Basic(t *testing.T) {
	test.NewTempApp(t)

	e := NewMultiLineEntry()
	assert.Equal(t, 1, e.LineCount(), "empty entry has one line")

	e.SetText("hello")
	assert.Equal(t, 1, e.LineCount())

	e.SetText("a\nb\nc")
	assert.Equal(t, 3, e.LineCount())

	e.SetText("a\nb\n")
	assert.Equal(t, 3, e.LineCount(), "trailing newline opens an empty last line")
}

func TestEntryLineCount_DirectFieldWrite(t *testing.T) {
	// A never-rendered entry with the Text field written directly reports
	// the field's count (the provider has not synced yet).
	e := &Entry{Text: "a\nb", MultiLine: true}
	assert.Equal(t, 2, e.LineCount())
}

func TestEntryLineCount_LazyPath(t *testing.T) {
	test.NewTempApp(t)

	text := strings.Repeat("line\n", 4999) + "last"
	e, w := newLazyEntry(t, text)
	defer w.Close()

	p := e.textProvider()
	require.True(t, p.lazy)
	require.NotNil(t, p.buf)
	assert.Equal(t, 5000, e.LineCount(), "count served from the buffer's line index")

	// An edit that inserts a newline is reflected immediately.
	e.CursorRow, e.CursorColumn = 0, 4
	e.InsertText("\nsplit")
	assert.Equal(t, 5001, e.LineCount())
}

func TestEntryLineCount_DeferTextSync(t *testing.T) {
	e, w := newDeferSyncEntry(t, "hello\nworld")
	defer w.Close()

	assert.Equal(t, 2, e.LineCount())

	// Deferred edits leave Text stale but the count is live from the buffer.
	e.CursorRow, e.CursorColumn = 1, 5
	e.InsertText("\nsplit\nagain")
	assert.Equal(t, "hello\nworld", e.Text, "Text stale between sync points")
	assert.Equal(t, 4, e.LineCount(), "count reflects the live buffer")

	// A sync point materializes; the count is unchanged.
	assert.Equal(t, "hello\nworld\nsplit\nagain", e.SyncText())
	assert.Equal(t, 4, e.LineCount())
}

func TestEntryLineCount_ExternalWriteWins(t *testing.T) {
	e, w := newDeferSyncEntry(t, "hello\nworld")
	defer w.Close()

	e.CursorRow, e.CursorColumn = 1, 5
	e.InsertText("\nsplit")
	require.Equal(t, 3, e.LineCount())

	// A direct Text write between sync points discards pending edits on the
	// next refresh; LineCount follows the new field immediately (the
	// identity anchor no longer matches).
	e.Text = "one\ntwo\nthree\nfour"
	assert.Equal(t, 4, e.LineCount(), "external write wins over the deferred buffer")
}
