//go:build accessibility

package accessibility

import (
	"sort"
	"unicode"
	"unsafe"

	"github.com/belluikit/typesetting/segmenter"
)

// TextIndex indexes one version of a text for the accessibility text
// protocols: the decoded rune slice plus the rune offsets of newlines and of
// astral (non-BMP) runes — the only ones occupying two UTF-16 code units —
// built in a single pass. Every query then runs in O(log) instead of
// re-scanning the document (#555): with a screen reader tracking the caret,
// several queries fire per keystroke, and on multi-megabyte documents
// per-query O(n) decoding dwarfed the sub-millisecond edit path.
//
// It started life inside the glfw driver (shared by the macOS NSAccessibility
// and Windows UIA text backends) and was hoisted here (#992) once follow-up
// work needed a neutral home; any adapter computing text boundaries in Go
// (web, AT-SPI) consumes this same index rather than growing its own.
type TextIndex struct {
	str      string // the indexed text; see IndexText for the identity contract
	rs       []rune
	newlines []int // rune offsets of '\n', ascending
	astral   []int // rune offsets of runes >= 0x10000, ascending

	// Grapheme-cluster boundaries for the character unit (#991), built
	// lazily on the first character-unit query. maybeClusters is set during
	// the index build when any rune could join a multi-rune cluster
	// (combining marks, ZWJ, variation selectors, regional indicators,
	// astral emoji — all >= U+0300 — plus '\r', since "\r\n" is one cluster);
	// without it every rune is its own cluster and the segmenter run and
	// boundary slice are skipped entirely — the common ASCII/plain-BMP case
	// stays allocation-free.
	maybeClusters  bool
	graphemesBuilt bool
	graphemes      []int // rune offsets of cluster starts, ascending
}

// textIndexCache is a single-entry cache: accessibility queries target the
// focused text field, so one entry covers the query burst between edits.
// Owned by the main goroutine like all bridge state.
var textIndexCache TextIndex

// IndexText returns the index for text, rebuilding only when its
// identity changed. The hit check compares the string data pointer, not the
// content — runtime string equality has no same-pointer fast path, so a
// plain == would byte-compare the whole document on every query. The rope
// behind Entry memoizes its string and returns the same object until the
// next splice (see the CLAUDE.md identity contract), so the burst of
// queries between two edits costs one O(n) build; a fresh-but-equal string
// (e.g. a password mask rebuilt per call) safely re-indexes.
func IndexText(text string) *TextIndex {
	if len(textIndexCache.str) == len(text) &&
		unsafe.StringData(textIndexCache.str) == unsafe.StringData(text) {
		return &textIndexCache
	}
	textIndexCache = BuildTextIndex(text)
	return &textIndexCache
}

// BuildTextIndex builds a standalone index (single O(n) pass), independent
// of the shared cache behind IndexText. The Linux AT-SPI backend keeps a
// per-node index for every exposed text field at once — unlike the
// focused-field-only darwin/windows query model the cache assumes — so it
// must not alias one shared entry.
func BuildTextIndex(text string) TextIndex {
	idx := TextIndex{str: text, rs: []rune(text)}
	for i, r := range idx.rs {
		if r == '\n' {
			idx.newlines = append(idx.newlines, i)
		}
		if r >= 0x10000 {
			idx.astral = append(idx.astral, i)
		}
		if r >= 0x0300 || r == '\r' {
			idx.maybeClusters = true
		}
	}
	return idx
}

// ClampRune clamps a rune offset to [0, len].
func (ix *TextIndex) ClampRune(idx int) int {
	if idx < 0 {
		return 0
	}
	if idx > len(ix.rs) {
		return len(ix.rs)
	}
	return idx
}

// RuneLen returns the text length in runes. It is nil-receiver-safe (0): the
// AT-SPI backend serves character-count queries for nodes that have no text
// snapshot yet.
func (ix *TextIndex) RuneLen() int {
	if ix == nil {
		return 0
	}
	return len(ix.rs)
}

// RuneAt returns the rune at offset i, or 0 when i is out of range.
func (ix *TextIndex) RuneAt(i int) rune {
	if ix == nil || i < 0 || i >= len(ix.rs) {
		return 0
	}
	return ix.rs[i]
}

// UTF16Len returns the text length in UTF-16 code units.
func (ix *TextIndex) UTF16Len() int {
	return len(ix.rs) + len(ix.astral)
}

// UTF16FromRune converts a rune offset (clamped) to the offset of the same
// boundary in UTF-16 code units: every astral rune before it adds one extra
// unit.
func (ix *TextIndex) UTF16FromRune(idx int) int {
	idx = ix.ClampRune(idx)
	return idx + sort.SearchInts(ix.astral, idx)
}

// RuneFromUTF16 converts a UTF-16 code-unit offset to a rune offset,
// snapping to the following boundary when it lands inside a surrogate pair.
func (ix *TextIndex) RuneFromUTF16(u16 int) int {
	if u16 <= 0 {
		return 0
	}
	if u16 >= ix.UTF16Len() {
		return len(ix.rs)
	}
	return sort.Search(len(ix.rs), func(i int) bool { return ix.UTF16FromRune(i) >= u16 })
}

// LineForRune returns the zero-based logical line the rune offset falls on:
// the number of newlines before it.
func (ix *TextIndex) LineForRune(idx int) int {
	return sort.SearchInts(ix.newlines, ix.ClampRune(idx))
}

// LineRuneRange returns the rune range [start, end) of the zero-based
// logical line, including its terminating newline (the platform text
// protocols' convention). ok is false for a line beyond the content.
func (ix *TextIndex) LineRuneRange(line int) (start, end int, ok bool) {
	if line < 0 || line > len(ix.newlines) {
		return 0, 0, false
	}
	if line > 0 {
		start = ix.newlines[line-1] + 1
	}
	if line < len(ix.newlines) {
		end = ix.newlines[line] + 1
	} else {
		end = len(ix.rs)
	}
	return start, end, true
}

// Substring returns the text of the clamped rune range [a, b).
func (ix *TextIndex) Substring(a, b int) string {
	a, b = ix.ClampRune(a), ix.ClampRune(b)
	if a >= b {
		return ""
	}
	return string(ix.rs[a:b])
}

// graphemeStarts returns the ascending rune offsets of grapheme-cluster
// starts, or nil when every rune is its own cluster (see maybeClusters).
// Built lazily via the UAX #29 segmenter on the first character-unit query
// and cached with the index.
func (ix *TextIndex) graphemeStarts() []int {
	if !ix.maybeClusters {
		return nil
	}
	if !ix.graphemesBuilt {
		var seg segmenter.Segmenter
		seg.Init(ix.rs)
		it := seg.GraphemeIterator()
		for it.Next() {
			ix.graphemes = append(ix.graphemes, it.Grapheme().Offset)
		}
		ix.graphemesBuilt = true
	}
	return ix.graphemes
}

// GraphemeBounds returns the rune range [start, end) of the grapheme cluster
// containing the clamped offset — the character unit of the platform text
// protocols (#991): screen-reader character navigation must step one user-
// perceived character (an emoji ZWJ family, a letter plus combining marks),
// never land inside a cluster. At the end of the text it returns the empty
// (len, len) range.
func (ix *TextIndex) GraphemeBounds(off int) (start, end int) {
	off = ix.ClampRune(off)
	if off == len(ix.rs) {
		return off, off
	}
	gs := ix.graphemeStarts()
	if gs == nil {
		return off, off + 1
	}
	// The cluster containing off starts at the last boundary <= off.
	i := sort.SearchInts(gs, off+1) - 1
	start = gs[i]
	if i+1 < len(gs) {
		end = gs[i+1]
	} else {
		end = len(ix.rs)
	}
	return start, end
}

// --- text units (UIA Text pattern, #487) ----------------------------------
// The UIA text protocol navigates by abstract units (character, word, line,
// document) rather than exposing offsets to the client, so the bridge keeps
// range endpoints as rune offsets and resolves unit boundaries here.

// wordClass buckets a rune for word-boundary purposes: a word is a
// maximal run of runes in the same class.
func wordClass(r rune) int {
	switch {
	case unicode.IsSpace(r):
		return 0
	case unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsMark(r) || r == '_':
		return 1
	}
	return 2 // punctuation and symbols
}

// wordStartsAt reports whether a word unit begins at rune offset i: units
// follow the platform convention of a word plus its trailing whitespace, so
// a unit starts at the first non-space rune after whitespace and at each
// transition between word characters and punctuation.
func (ix *TextIndex) wordStartsAt(i int) bool {
	if i == 0 {
		return true
	}
	c, p := wordClass(ix.rs[i]), wordClass(ix.rs[i-1])
	return c != 0 && (p == 0 || p != c)
}

// WordRuneRange returns the rune range [start, end) of the word unit
// containing the clamped offset. At the end of the text it returns the last
// unit; in empty text it returns (0, 0).
func (ix *TextIndex) WordRuneRange(off int) (start, end int) {
	if len(ix.rs) == 0 {
		return 0, 0
	}
	off = ix.ClampRune(off)
	if off == len(ix.rs) {
		off--
	}
	start = off
	for start > 0 && !ix.wordStartsAt(start) {
		start--
	}
	end = off + 1
	for end < len(ix.rs) && !ix.wordStartsAt(end) {
		end++
	}
	return start, end
}

// Unit constants mirror the UIA TextUnit values the backends pass through;
// unknown units (format, page) are treated as the closest coarser unit by
// UnitBounds/UnitStep.
const (
	UnitCharacter = 0
	UnitWord      = 2
	UnitLine      = 3
	UnitParagraph = 4
	UnitDocument  = 6
)

// UnitBounds returns the rune range [start, end) of the unit containing the
// offset. Paragraphs map to lines (bellui text has no separate paragraph
// structure); anything coarser than paragraph means the whole document.
func (ix *TextIndex) UnitBounds(unit, off int) (start, end int) {
	off = ix.ClampRune(off)
	switch unit {
	case UnitCharacter:
		return ix.GraphemeBounds(off)
	case UnitWord:
		return ix.WordRuneRange(off)
	case UnitLine, UnitParagraph:
		s, e, _ := ix.LineRuneRange(ix.LineForRune(off))
		return s, e
	}
	return 0, len(ix.rs)
}

// UnitStep moves the offset by count units (negative = backwards) and
// returns the new offset plus the number of units actually crossed. An
// offset moves between unit start boundaries, clamped to the text.
func (ix *TextIndex) UnitStep(unit, off, count int) (newOff, moved int) {
	off = ix.ClampRune(off)
	step := func(dir int) (int, bool) {
		switch unit {
		case UnitCharacter:
			// One character = one grapheme cluster (#991): forward moves to
			// the end of the cluster containing the offset, backward to the
			// start of the cluster containing the previous rune — both snap
			// an intra-cluster offset out to a cluster boundary.
			if dir > 0 {
				if off >= len(ix.rs) {
					return off, false
				}
				_, end := ix.GraphemeBounds(off)
				return end, true
			}
			if off <= 0 {
				return off, false
			}
			start, _ := ix.GraphemeBounds(off - 1)
			return start, true
		case UnitWord:
			start, end := ix.WordRuneRange(off)
			if dir > 0 {
				if end >= len(ix.rs) {
					return off, false
				}
				next, _ := ix.WordRuneRange(end)
				return next, next != off
			}
			if off > start {
				return start, true
			}
			if start == 0 {
				return off, false
			}
			prev, _ := ix.WordRuneRange(start - 1)
			return prev, true
		case UnitLine, UnitParagraph:
			line := ix.LineForRune(off)
			start, _, _ := ix.LineRuneRange(line)
			if dir > 0 {
				next, _, ok := ix.LineRuneRange(line + 1)
				if !ok || line >= len(ix.newlines) {
					return off, false
				}
				return next, true
			}
			if off > start {
				return start, true
			}
			prev, _, ok := ix.LineRuneRange(line - 1)
			if !ok {
				return off, false
			}
			return prev, true
		}
		// Document unit: snap to an extremity once.
		if dir > 0 {
			if off >= len(ix.rs) {
				return off, false
			}
			return len(ix.rs), true
		}
		if off <= 0 {
			return off, false
		}
		return 0, true
	}

	dir := 1
	if count < 0 {
		dir, count = -1, -count
	}
	for ; moved < count; moved++ {
		next, ok := step(dir)
		if !ok {
			break
		}
		off = next
	}
	return off, moved * dir
}
