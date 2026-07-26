package widget

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// Tests for the edit-incremental dense build + render (#921). The core safety
// net is an equivalence oracle: after an edit, the incrementally spliced row
// model, object list and geometry must be indistinguishable from a fresh
// widget built from scratch with the same content. Test-hook counters assert
// the incremental path actually ran, so the comparison is never vacuous.

func TestSpliceGap(t *testing.T) {
	base := func() []int { return []int{0, 1, 2, 3, 4, 5} }

	s := spliceGap(base(), 2, 4, 2) // same size
	copy(s[2:], []int{9, 8})
	assert.Equal(t, []int{0, 1, 9, 8, 4, 5}, s)

	s = spliceGap(base(), 2, 4, 0) // shrink to nothing
	assert.Equal(t, []int{0, 1, 4, 5}, s)

	s = spliceGap(base(), 2, 2, 3) // pure insert
	copy(s[2:], []int{7, 8, 9})
	assert.Equal(t, []int{0, 1, 7, 8, 9, 2, 3, 4, 5}, s)

	s = spliceGap(base(), 0, 6, 1) // replace all
	s[0] = 42
	assert.Equal(t, []int{42}, s)

	s = spliceGap(base(), 5, 6, 4) // grow at the end
	copy(s[5:], []int{9, 9, 9, 9})
	assert.Equal(t, []int{0, 1, 2, 3, 4, 9, 9, 9, 9}, s)
}

// incrTestSegments builds a flat, incremental-eligible document: n lines, each
// a plain run followed by a bold run ending in a newline (the RichEntry shape).
func incrTestSegments(n int) []RichTextSegment {
	segs := make([]RichTextSegment, 0, n*2)
	for i := 0; i < n; i++ {
		segs = append(
			segs,
			&TextSegment{Style: RichTextStyle{Inline: true}, Text: fmt.Sprintf("line %d plain ", i)},
			&TextSegment{Style: RichTextStyle{Inline: true, TextStyle: bellui.TextStyle{Bold: true}}, Text: fmt.Sprintf("bold %d\n", i)},
		)
	}
	return segs
}

// incrSnapshot is a widget-independent capture of a rendered RichText: row
// boundaries with segments mapped to their top-level index, every object's
// text and geometry, and the row geometry caches.
type incrSnapshot struct {
	rows      []string
	objs      []string
	rowYPos   []float32
	rowHeight []float32
	minSize   geo.Size
}

func snapshotRichText(t *testing.T, rt *RichText) incrSnapshot {
	t.Helper()
	segIdx := make(map[RichTextSegment]int, len(rt.Segments))
	for i, seg := range rt.Segments {
		segIdx[seg] = i
	}
	r, ok := test.TempWidgetRenderer(t, rt).(*textRenderer)
	require.True(t, ok)

	snap := incrSnapshot{minSize: r.MinSize()}
	for _, b := range rt.rowBounds {
		ids := make([]string, 0, len(b.segments))
		for _, seg := range b.segments {
			idx, found := segIdx[seg]
			require.True(t, found, "row references a segment not in Segments")
			ids = append(ids, fmt.Sprint(idx))
		}
		snap.rows = append(snap.rows, fmt.Sprintf("segs=%s reuse=%d begin=%d end=%d",
			strings.Join(ids, ","), b.firstSegmentReuse, b.begin, b.end))
	}
	objs := r.origObjs
	if len(objs) == 0 {
		objs = r.Objects()
	}
	for _, obj := range objs {
		if txt, isText := obj.(*canvas.Text); isText {
			snap.objs = append(snap.objs, fmt.Sprintf("%q pos=%v size=%v bold=%v", txt.Text, txt.Position(), txt.Size(), txt.TextStyle.Bold))
		} else {
			snap.objs = append(snap.objs, fmt.Sprintf("obj pos=%v size=%v", obj.Position(), obj.Size()))
		}
	}
	snap.rowYPos = append(snap.rowYPos, rt.rowYPos...)
	snap.rowHeight = append(snap.rowHeight, rt.rowHeight...)
	return snap
}

// runIncrOracle builds a rendered RichText, applies edit to its segment list,
// refreshes, and asserts the result is identical to a from-scratch widget with
// cloned content. wantIncremental asserts the incremental build+render ran.
func runIncrOracle(t *testing.T, wrap bellui.TextWrap, width float32, segs []RichTextSegment, wantIncremental bool, edit func(rt *RichText)) {
	t.Helper()
	rt := NewRichText(segs...)
	rt.Wrapping = wrap
	test.TempWidgetRenderer(t, rt) // realise the renderer
	rt.Resize(geo.NewSize(width, 600))
	rt.Refresh()
	require.NotNil(t, rt.denseIncr, "document should be incremental-eligible")

	edit(rt)
	rebuildsBefore, rendersBefore := denseIncrRebuilds, denseIncrRenders
	rt.Refresh()
	if wantIncremental {
		assert.Equal(t, rebuildsBefore+1, denseIncrRebuilds, "edit should take the incremental build")
		assert.Equal(t, rendersBefore+1, denseIncrRenders, "edit should take the incremental render")
	}

	// Fresh widget with cloned content — the from-scratch reference.
	clones := make([]RichTextSegment, 0, len(rt.Segments))
	for _, seg := range rt.Segments {
		clones = append(clones, cloneSegment(seg))
	}
	ref := NewRichText(clones...)
	ref.Wrapping = wrap
	test.TempWidgetRenderer(t, ref)
	ref.Resize(geo.NewSize(width, 600))
	ref.Refresh()

	got, want := snapshotRichText(t, rt), snapshotRichText(t, ref)
	assert.Equal(t, want.rows, got.rows, "row model must match a from-scratch build")
	assert.Equal(t, want.objs, got.objs, "objects must match a from-scratch build")
	assert.Equal(t, want.rowYPos, got.rowYPos, "row Y positions must match")
	assert.Equal(t, want.rowHeight, got.rowHeight, "row heights must match")
	assert.Equal(t, want.minSize, got.minSize, "MinSize must match")

	// A second edit must also work against the spliced bookkeeping.
	if ts, ok := rt.Segments[len(rt.Segments)/2].(*TextSegment); ok {
		ts.Text = "changed again " + ts.Text
		rt.Refresh()

		clones = clones[:0]
		for _, seg := range rt.Segments {
			clones = append(clones, cloneSegment(seg))
		}
		ref2 := NewRichText(clones...)
		ref2.Wrapping = wrap
		test.TempWidgetRenderer(t, ref2)
		ref2.Resize(geo.NewSize(width, 600))
		ref2.Refresh()
		got, want = snapshotRichText(t, rt), snapshotRichText(t, ref2)
		assert.Equal(t, want.rows, got.rows, "second edit: row model must match")
		assert.Equal(t, want.objs, got.objs, "second edit: objects must match")
		assert.Equal(t, want.rowYPos, got.rowYPos, "second edit: row Y positions must match")
	}
}

func TestRichTextIncremental_Equivalence(t *testing.T) {
	test.NewTempApp(t)

	scenarios := map[string]struct {
		wrap            bellui.TextWrap
		width           float32
		segs            func() []RichTextSegment
		wantIncremental bool
		edit            func(rt *RichText)
	}{
		"type mid segment": {
			wrap: bellui.TextWrapOff, width: 400, segs: func() []RichTextSegment { return incrTestSegments(10) },
			wantIncremental: true,
			edit: func(rt *RichText) {
				ts := rt.Segments[8].(*TextSegment)
				ts.Text = ts.Text[:4] + "XY" + ts.Text[4:]
			},
		},
		"edit first segment": {
			wrap: bellui.TextWrapOff, width: 400, segs: func() []RichTextSegment { return incrTestSegments(10) },
			wantIncremental: true,
			edit: func(rt *RichText) {
				ts := rt.Segments[0].(*TextSegment)
				ts.Text = "PRE " + ts.Text
			},
		},
		"edit last segment": {
			wrap: bellui.TextWrapOff, width: 400, segs: func() []RichTextSegment { return incrTestSegments(10) },
			wantIncremental: true,
			edit: func(rt *RichText) {
				ts := rt.Segments[len(rt.Segments)-1].(*TextSegment)
				ts.Text = strings.TrimSuffix(ts.Text, "\n") + " END\n"
			},
		},
		"insert newline": {
			wrap: bellui.TextWrapOff, width: 400, segs: func() []RichTextSegment { return incrTestSegments(10) },
			wantIncremental: true,
			edit: func(rt *RichText) {
				ts := rt.Segments[6].(*TextSegment)
				ts.Text = ts.Text[:5] + "\n" + ts.Text[5:]
			},
		},
		"remove newline": {
			wrap: bellui.TextWrapOff, width: 400, segs: func() []RichTextSegment { return incrTestSegments(10) },
			wantIncremental: true,
			edit: func(rt *RichText) {
				ts := rt.Segments[7].(*TextSegment) // bold run ending in \n
				ts.Text = strings.Replace(ts.Text, "\n", " ", 1)
				// Same-length content leaves the fingerprint unchanged (as for
				// any raw in-place mutation today); bump like the edit methods.
				rt.rowBoundsVersion++
			},
		},
		"insert segment": {
			wrap: bellui.TextWrapOff, width: 400, segs: func() []RichTextSegment { return incrTestSegments(10) },
			wantIncremental: true,
			edit: func(rt *RichText) {
				mid := len(rt.Segments) / 2
				ins := &TextSegment{Style: RichTextStyle{Inline: true, TextStyle: bellui.TextStyle{Italic: true}}, Text: "inserted run "}
				rt.Segments = append(rt.Segments[:mid], append([]RichTextSegment{ins}, rt.Segments[mid:]...)...)
			},
		},
		"remove segment": {
			wrap: bellui.TextWrapOff, width: 400, segs: func() []RichTextSegment { return incrTestSegments(10) },
			wantIncremental: true,
			edit: func(rt *RichText) {
				mid := len(rt.Segments) / 2
				rt.Segments = append(rt.Segments[:mid], rt.Segments[mid+1:]...)
			},
		},
		"replace segment new pointer same length": {
			wrap: bellui.TextWrapOff, width: 400, segs: func() []RichTextSegment { return incrTestSegments(10) },
			wantIncremental: true,
			edit: func(rt *RichText) {
				old := rt.Segments[4].(*TextSegment)
				repl := &TextSegment{Style: old.Style, Text: "same width here "}
				rt.Segments[4] = repl
				// Same-length content leaves the fingerprint unchanged; bump the
				// version the way the mutation methods do.
				rt.rowBoundsVersion++
			},
		},
		"restyle in place plus text change": {
			wrap: bellui.TextWrapOff, width: 400, segs: func() []RichTextSegment { return incrTestSegments(10) },
			wantIncremental: true,
			edit: func(rt *RichText) {
				rt.Segments[2].(*TextSegment).Style.TextStyle.Italic = true
				ts := rt.Segments[12].(*TextSegment)
				ts.Text += "tail"
			},
		},
		"two distant edits": {
			wrap: bellui.TextWrapOff, width: 400, segs: func() []RichTextSegment { return incrTestSegments(12) },
			wantIncremental: true,
			edit: func(rt *RichText) {
				rt.Segments[2].(*TextSegment).Text += "AA"
				rt.Segments[20].(*TextSegment).Text += "BB"
			},
		},
		"append segments at end": {
			wrap: bellui.TextWrapOff, width: 400, segs: func() []RichTextSegment { return incrTestSegments(10) },
			wantIncremental: true,
			edit: func(rt *RichText) {
				rt.Segments = append(rt.Segments,
					&TextSegment{Style: RichTextStyle{Inline: true}, Text: "appended tail"})
			},
		},
		"edit around separator block": {
			wrap: bellui.TextWrapOff, width: 400,
			segs: func() []RichTextSegment {
				segs := incrTestSegments(8)
				mid := len(segs) / 2
				return append(segs[:mid], append([]RichTextSegment{&SeparatorSegment{}}, segs[mid:]...)...)
			},
			wantIncremental: true,
			edit: func(rt *RichText) {
				rt.Segments[6].(*TextSegment).Text += "near the block"
			},
		},
		"wrapped word edit": {
			wrap: bellui.TextWrapWord, width: 150, segs: func() []RichTextSegment { return incrTestSegments(10) },
			// Wrapping may or may not converge incrementally depending on
			// widths — equivalence must hold either way.
			wantIncremental: false,
			edit: func(rt *RichText) {
				ts := rt.Segments[8].(*TextSegment)
				ts.Text = ts.Text + "wrapping tail growth here "
			},
		},
	}

	for name, sc := range scenarios {
		t.Run(name, func(t *testing.T) {
			runIncrOracle(t, sc.wrap, sc.width, sc.segs(), sc.wantIncremental, sc.edit)
		})
	}
}

// TestRichTextIncremental_SmallDocFullPath pins that documents under the
// checkpoint threshold never record incremental state.
func TestRichTextIncremental_SmallDocFullPath(t *testing.T) {
	test.NewTempApp(t)
	rt := NewRichText(incrTestSegments(2)...) // 4 segments < denseIncrMinSegments
	test.TempWidgetRenderer(t, rt)
	rt.Resize(geo.NewSize(300, 300))
	rt.Refresh()
	assert.Nil(t, rt.denseIncr)
}

// TestRichEntryIncremental_TypingEquivalence drives real RichEntry edits —
// typing at a style boundary, newline, undo/redo — and checks the provider
// stays equivalent to a from-scratch render of the same document.
func TestRichEntryIncremental_TypingEquivalence(t *testing.T) {
	test.NewTempApp(t)

	e := NewRichEntry()
	e.SetSegments(incrTestSegments(10))
	w := test.NewWindow(e)
	defer w.Close()
	w.Resize(geo.NewSize(400, 300))

	check := func(step string) {
		t.Helper()
		clones := make([]RichTextSegment, 0, len(e.provider.Segments))
		for _, seg := range e.provider.Segments {
			clones = append(clones, cloneSegment(seg))
		}
		ref := NewRichText(clones...)
		ref.Wrapping = e.provider.Wrapping
		ref.Scroll = e.provider.Scroll
		test.TempWidgetRenderer(t, ref)
		ref.Resize(e.provider.Size())
		ref.Refresh()
		got, want := snapshotRichText(t, &e.provider), snapshotRichText(t, ref)
		assert.Equal(t, want.rows, got.rows, "%s: rows", step)
		assert.Equal(t, want.objs, got.objs, "%s: objects", step)
		assert.Equal(t, want.rowYPos, got.rowYPos, "%s: row Y positions", step)
	}

	before := denseIncrRenders

	// Type at a style boundary (start of a plain run, inheriting bold).
	e.moveCaret(e.doc.len()/2, false)
	e.insertRunes([]rune{'x'})
	check("boundary type")
	e.insertRunes([]rune{'y', 'z'})
	check("continued typing")

	// Newline changes the row count.
	e.insertRunes([]rune{'\n'})
	check("newline")

	// Deletion via selection replace.
	e.moveCaret(5, false)
	e.moveCaret(9, true)
	e.insertRunes([]rune{'Q'})
	check("type over selection")

	// Undo / redo replay through the same incremental machinery.
	e.Undo()
	check("undo")
	e.Redo()
	check("redo")

	assert.Greater(t, denseIncrRenders, before, "RichEntry edits should ride the incremental render")
}
