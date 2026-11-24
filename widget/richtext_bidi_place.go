package widget

import (
	"slices"
	"sort"
	"unicode"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	paint "github.com/belluikit/bellui/internal/painter"
	"github.com/belluikit/bellui/lang"
)

// bidiRowPlacement returns where each of a row's objects is drawn, as an x from
// the row's left edge, when the row holds right-to-left text: in the row's bidi
// visual order, the order its carets, hit-testing and selection already use
// (rowVisualLayout). Laid side by side, a row's segments read in logical order
// from the left, so a Hebrew sentence split into two segments drew its second
// half first, and the carets were placed on the other half's glyphs (#1441).
// The x includes the row's alignment in lineWidth and leaves out its break
// space (rowBreakSpace), as layoutRowTrimmed does.
//
// It returns nil, and the row keeps the side-by-side layout, for a row with no
// right-to-left text, a row the visual layout does not model (one holding a
// concealed segment), and a row where some segment's letters and digits do
// not stay together in the visual order: a segment that mixes directions can
// have a neighbour's text drawn between its runs, and one object cannot be
// drawn in two places.
func (t *RichText) bidiRowPlacement(bound *rowBoundary, objs []core.CanvasObject, align bellui.TextAlign,
	lineWidth, breakSpace float32, breakLeft bool,
) []float32 {
	if len(objs) < 2 || len(objs) != len(bound.segments) || !rowHasRTL(bound) {
		return nil
	}
	styled, ok := t.boundStyledRuns(bound)
	if !ok {
		return nil
	}
	p := t.cachedBidiPlacement(objs, styled)
	if p.rel == nil {
		return nil
	}

	// The glyphs span the layout less the break space, at whichever end the
	// space is drawn.
	vl, vr := float32(0), p.width
	if breakLeft {
		vl = breakSpace
	} else {
		vr -= breakSpace
	}
	var off float32
	switch lang.ResolveAlign(align) {
	case bellui.TextAlignTrailing:
		off = lineWidth - vr
	case bellui.TextAlignCenter:
		off = (lineWidth - vl - vr) / 2
	default:
		off = -vl
	}
	xs := make([]float32, len(p.rel))
	for i, x := range p.rel {
		xs[i] = off + x
	}
	return xs
}

// bidiPlacement is a row's placement before alignment: each object's x from
// the left of the row's visual layout, width wide. rel is nil when the row
// keeps the side-by-side layout.
type bidiPlacement struct {
	rel   []float32
	width float32
}

// bidiPlaceEntry is a row's cached placement and what it was computed from.
// Shaping the row and each segment again on every layout made laying out a
// long right-to-left document hundreds of times slower, and only the text, its
// styles and the ambient direction change a placement.
type bidiPlaceEntry struct {
	objs   []core.CanvasObject
	runs   []paint.StyledRun
	rtl    bool
	result bidiPlacement
}

// cachedBidiPlacement returns placeBidiRow's result for the row drawn by objs
// with runs, computing it only when the row's objects, runs or the ambient
// direction differ from the last time.
func (t *RichText) cachedBidiPlacement(objs []core.CanvasObject, runs []paint.StyledRun) bidiPlacement {
	rtl := lang.IsRTLDirection()
	if e, ok := t.bidiPlace[objs[0]]; ok && e.rtl == rtl && slices.Equal(e.objs, objs) &&
		slices.EqualFunc(e.runs, runs, sameStyledRun) {
		return e.result
	}
	// Rows are replaced as the text changes; drop what the old rows left
	// once the cache outgrows the rows it serves.
	if t.bidiPlace == nil || len(t.bidiPlace) > 2*len(t.rowBounds)+64 {
		t.bidiPlace = make(map[core.CanvasObject]*bidiPlaceEntry)
	}
	result := placeBidiRow(objs, runs)
	t.bidiPlace[objs[0]] = &bidiPlaceEntry{objs: slices.Clone(objs), runs: runs, rtl: rtl, result: result}
	return result
}

// sameStyledRun reports whether a and b shape alike.
func sameStyledRun(a, b paint.StyledRun) bool {
	return a.Box == b.Box && a.BoxWidth == b.BoxWidth && a.TextSize == b.TextSize &&
		a.Style == b.Style && a.FontSource == b.FontSource && slices.Equal(a.Runes, b.Runes)
}

// placeBidiRow lays out a row's runs in bidi visual order and places each of
// its objects there (bidiRowPlacement).
func placeBidiRow(objs []core.CanvasObject, styled []paint.StyledRun) bidiPlacement {
	layout, ok := styledVisualLayout(styled)
	if !ok {
		return bidiPlacement{}
	}

	type piece struct {
		mid float32
		seg int
	}
	var pieces []piece
	rel := make([]float32, len(objs))
	textOff, box := 0, 0
	for i, run := range styled {
		if run.Box {
			l, r, ok := layout.BoxExtent(box)
			if !ok {
				return bidiPlacement{}
			}
			box++
			pieces = append(pieces, piece{(l + r) / 2, i})
			rel[i] = l
			continue
		}
		if len(run.Runes) == 0 {
			// Nothing is drawn; keep it where its caret is.
			rel[i] = layout.CaretX(textOff)
			continue
		}
		left, anchor := float32(0), -1
		for k, ch := range run.Runes {
			l, r, ok := layout.RuneExtent(textOff + k)
			if !ok {
				return bidiPlacement{}
			}
			if k == 0 || l < left {
				left = l
			}
			if !unicode.IsLetter(ch) && !unicode.IsDigit(ch) {
				continue
			}
			pieces = append(pieces, piece{(l + r) / 2, i})
			if anchor < 0 {
				anchor = k
			}
		}
		x := left
		// A segment drawn on its own can put a space or a punctuation mark at
		// its edge on the other side of its glyphs than the row does: Hebrew
		// followed by a space in a left-to-right base direction ends with the
		// space on the right, but inside a right-to-left sentence it sits on
		// the left. So the object is placed by its first letter or digit, where
		// the object draws it and where the row does, and only letters and
		// digits must stay together in the row's order.
		if anchor >= 0 {
			if own, ok := drawnRuneLeft(objs[i], run.Runes, anchor); ok {
				rowLeft, _, _ := layout.RuneExtent(textOff + anchor)
				x = rowLeft - own
			}
		}
		rel[i] = x
		textOff += len(run.Runes)
	}

	// Each segment's pieces must be consecutive in the visual order.
	sort.SliceStable(pieces, func(a, b int) bool { return pieces[a].mid < pieces[b].mid })
	done := make([]bool, len(objs))
	for k, p := range pieces {
		if k > 0 && pieces[k-1].seg == p.seg {
			continue
		}
		if done[p.seg] {
			return bidiPlacement{}
		}
		done[p.seg] = true
	}
	return bidiPlacement{rel: rel, width: layout.Width}
}

// drawnRuneLeft returns the x, from obj's left edge, at which obj draws rune k
// of runes, when obj is the text object that draws exactly them.
func drawnRuneLeft(obj core.CanvasObject, runes []rune, k int) (float32, bool) {
	txt, ok := obj.(*canvas.Text)
	if !ok || txt.Text != string(runes) {
		return 0, false
	}
	face := paint.CachedFontFace(txt.TextStyle, txt.FontSource, nil)
	own := paint.LineVisualLayout(face.Fonts, runes, txt.TextSize, txt.TextStyle)
	l, _, ok := own.RuneExtent(k)
	return l, ok
}
