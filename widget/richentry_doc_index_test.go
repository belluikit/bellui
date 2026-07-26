package widget

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
)

// Oracle tests for the richEntryDoc derived memos (#975): after every edit the
// patched position index and wrap-off row model must equal a from-scratch
// recomputation over the same segments, and segAt must agree with the linear
// reference walk it replaced.

// docIndexReference recomputes positions and rows from d.segs with none of the
// memo machinery.
func docIndexReference(d *richEntryDoc) (positions []int, rows []richRow) {
	pos := 0
	positions = append(positions, 0)
	for _, seg := range d.segs {
		pos += segPositions(seg)
		positions = append(positions, pos)
	}
	fresh := &richEntryDoc{segs: d.segs}
	rows = append(rows, fresh.rowModel()...)
	return positions, rows
}

func assertDocIndex(t *testing.T, d *richEntryDoc, step string) {
	t.Helper()
	wantPos, wantRows := docIndexReference(d)
	assert.Equal(t, wantPos, append([]int(nil), d.positions()...), "%s: position index", step)
	assert.Equal(t, wantRows, append([]richRow(nil), d.rowModel()...), "%s: row model", step)

	// segAt must match the linear reference at every position (incl. past-end).
	for offset := 0; offset <= d.len()+1; offset++ {
		wi, wl := docSegAtReference(d, offset)
		gi, gl := d.segAt(offset)
		require.Equal(t, [2]int{wi, wl}, [2]int{gi, gl}, "%s: segAt(%d)", step, offset)
	}
}

// docSegAtReference is the pre-#975 linear segAt.
func docSegAtReference(d *richEntryDoc, offset int) (idx, local int) {
	if offset < 0 {
		offset = 0
	}
	cum := 0
	for i, seg := range d.segs {
		n := segPositions(seg)
		if offset < cum+n {
			return i, offset - cum
		}
		cum += n
	}
	last := len(d.segs) - 1
	if last < 0 {
		return 0, 0
	}
	return last, segPositions(d.segs[last])
}

func TestRichEntryDoc_IndexOracle(t *testing.T) {
	bold := RichTextStyle{Inline: true, TextStyle: bellui.TextStyle{Bold: true}}
	plain := RichTextStyle{Inline: true}

	d := newRichEntryDoc()
	segs := make([]RichTextSegment, 0, 8)
	for i := 0; i < 4; i++ {
		segs = append(
			segs,
			&TextSegment{Style: plain, Text: fmt.Sprintf("plain %d ", i)},
			&TextSegment{Style: bold, Text: fmt.Sprintf("bold %d\n", i)},
		)
	}
	d.setSegments(segs)
	assertDocIndex(t, d, "initial")

	steps := []struct {
		name string
		op   func()
	}{
		{"type mid segment (in-place)", func() { d.insertText(3, []rune("xy"), plain) }},
		{"type at boundary (prev-append)", func() { d.insertText(int(d.positions()[2]), []rune("z"), bold) }},
		{"type newline (in-place, rows invalidate)", func() { d.insertText(5, []rune("a\nb"), plain) }},
		{"type at start", func() { d.insertText(0, []rune("S"), plain) }},
		{"type at end", func() { d.insertText(d.len(), []rune("E"), bold) }},
		{"styled insert (split)", func() {
			d.insertText(7, []rune("it"), RichTextStyle{Inline: true, TextStyle: bellui.TextStyle{Italic: true}})
		}},
		{"backspace (in-place delete)", func() { d.deleteRange(6, 7) }},
		{"delete range in one segment", func() { d.deleteRange(2, 5) }},
		{"delete range across segments", func() { d.deleteRange(1, d.len()/2) }},
		{"delete range with newline", func() { d.deleteRange(d.len()/3, d.len()/3+9) }},
		{"insert atom", func() { d.insertAtom(4, &ImageSegment{Title: "x"}) }},
		{"type beside atom", func() { d.insertText(4, []rune("q"), plain) }},
		{"delete the atom", func() { d.deleteRange(4, 6) }},
		{"restyle range", func() { d.applyStyle(2, 8, bold) }},
		{"replace range", func() { d.replaceRange(3, 6, []RichTextSegment{&TextSegment{Style: plain, Text: "rep\nlaced"}}) }},
		{"delete everything", func() { d.deleteRange(0, d.len()) }},
		{"type into empty", func() { d.insertText(0, []rune("hello\nworld"), plain) }},
	}
	for _, s := range steps {
		s.op()
		assertDocIndex(t, d, s.name)
	}
}
