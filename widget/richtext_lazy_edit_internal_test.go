package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// bufferLineStarts collects the buffer's logical-line starts into a slice for
// comparison against a ground-truth rebuild.
func bufferLineStarts(t *RichText) []int {
	starts := make([]int, t.buf.LineCount())
	for i := range starts {
		starts[i] = t.buf.LineStart(i)
	}
	return starts
}

// authoritativeLineStarts forces a full rebuild of the lazy row model from seg.Text
// (dropping the incremental buffer) and returns the freshly-scanned line starts, so
// the incrementally maintained index can be compared against the ground truth.
func authoritativeLineStarts(t *RichText) []int {
	t.lazyStr = "\x00invalid" // force the full-build branch in updateRowBoundsLazy
	t.rowBoundsVersion++
	t.updateRowBoundsLazy(t.Size().Width)
	return bufferLineStarts(t)
}

// TestLazyEdit_IncrementalMatchesRebuild drives the single-segment lazy fast edit
// path (#380) through inserts and deletes — single-line, newline-splitting,
// multibyte, whole-document — and after each edit asserts the incrementally
// maintained buffer and line index agree with a full rebuild from seg.Text.
func TestLazyEdit_IncrementalMatchesRebuild(t *testing.T) {
	e := NewMultiLineEntry()
	e.SetText("Hello world\nSecond line\nThird line here\nLast")
	w := test.NewWindow(e)
	defer w.Close()
	w.Resize(geo.NewSize(400, 300))

	p := e.textProvider()
	p.updateRowBounds()
	seg := p.Segments[0].(*TextSegment)

	if !p.lazyCanEdit() {
		t.Fatal("expected the single-segment lazy edit fast path to be active")
	}

	// ref mirrors the buffer with a trivial []rune splice, an independent oracle for
	// the incrementally maintained content (no hand-computed expected strings).
	ref := []rune(seg.Text)
	splice := func(low, high int, ins []rune) {
		ref = append(ref[:low:low], append(append([]rune{}, ins...), ref[high:]...)...)
	}

	steps := []struct {
		name string
		do   func()
	}{
		{"insert plain char at head", func() { p.insertAt(0, []rune("X")); splice(0, 0, []rune("X")) }},
		{"insert newline (structural split)", func() { p.insertAt(6, []rune("\n")); splice(6, 6, []rune("\n")) }},
		{"insert multibyte run", func() { p.insertAt(2, []rune("é界")); splice(2, 2, []rune("é界")) }},
		{"insert block with two newlines", func() { p.insertAt(0, []rune("A\nB\n")); splice(0, 0, []rune("A\nB\n")) }},
		{"delete across a newline (merge lines)", func() { p.deleteFromTo(0, 2); splice(0, 2, nil) }},
		{"delete within a line", func() { p.deleteFromTo(1, 4); splice(1, 4, nil) }},
		{"delete a multi-line span", func() { p.deleteFromTo(5, 20); splice(5, 20, nil) }},
	}

	for i, s := range steps {
		s.do()
		e.updateText(p.String(), false)
		want := string(ref)

		assert.Equalf(t, want, e.Text, "step %d (%s): Text", i, s.name)
		assert.Equalf(t, want, p.buf.String(), "step %d (%s): buffer vs oracle", i, s.name)
		assert.Equalf(t, seg.Text, p.buf.String(), "step %d (%s): seg.Text vs buffer", i, s.name)
		assert.Equalf(t, seg.Text, string(p.buf.Slice(0, p.buf.Len())), "step %d (%s): seg.Text vs buffer runes", i, s.name)

		got := bufferLineStarts(p)
		assert.Equalf(t, authoritativeLineStarts(p), got, "step %d (%s): incremental line index vs rebuild", i, s.name)
	}
}

// TestLazyEdit_ByteSpliceParity checks that the byte-splice used to maintain seg.Text
// produces exactly what a full []rune->string re-encode would, for multibyte content
// at every rune boundary.
func TestLazyEdit_ByteSpliceParity(t *testing.T) {
	e := NewMultiLineEntry()
	e.SetText("aé界b\ncд\nz")
	w := test.NewWindow(e)
	defer w.Close()
	w.Resize(geo.NewSize(400, 300))
	p := e.textProvider()
	p.updateRowBounds()

	for pos := 0; pos <= p.len(); pos++ {
		p.insertAt(pos, []rune("Ω\n"))
		reencoded := string(p.buf.Slice(0, p.buf.Len()))
		assert.Equalf(t, reencoded, p.Segments[0].(*TextSegment).Text, "insert at rune %d: byte-splice must equal re-encode", pos)
		// undo the insert to keep positions comparable
		p.deleteFromTo(pos, pos+2)
		assert.Equalf(t, "aé界b\ncд\nz", p.buf.String(), "after undo at %d", pos)
	}
}
