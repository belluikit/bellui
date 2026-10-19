package widget

import (
	"runtime"
	"strings"
	"testing"

	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// TestEntryTypingDoesNotRetainDocuments pins #508: every keystroke on a large
// document materialises a new copy of it, and row text taken as substrings of
// that copy kept each one alive, a full document per key.
func TestEntryTypingDoesNotRetainDocuments(t *testing.T) {
	test.NewTempApp(t)
	e := NewMultiLineEntry()
	e.SetText(strings.Repeat("the quick brown fox jumps over the lazy dog line content here\n", 50_000)) // about 3 MB
	w := test.NewWindow(e)
	defer w.Close()
	w.Resize(geo.NewSize(800, 600))
	e.textProvider().updateRowBounds()
	e.TypedRune('x') // settle the first edit's one-off allocations

	heap := func() uint64 {
		runtime.GC()
		runtime.GC()
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		return m.HeapInuse
	}
	before := heap()
	for range 20 {
		e.TypedRune('x')
	}
	grown := int64(heap()) - int64(before)
	// Retaining every copy would hold about 20 × 3 MB.
	if grown > 15<<20 {
		t.Errorf("heap grew %d MB over 20 keystrokes on a 3 MB document; the old copies are being kept", grown>>20)
	}
	runtime.KeepAlive(e)
}
