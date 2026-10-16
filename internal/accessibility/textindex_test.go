//go:build accessibility

package accessibility

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// BuildTextIndex is the standalone (non-caching) builder for consumers that
// hold many indexes at once — the AT-SPI backend keeps one per exposed text
// node (pre-rebrand tracker #986/#993) — so two builds must never alias the IndexText cache or
// each other.
func TestBuildTextIndex_Standalone(t *testing.T) {
	cached := IndexText("cached text")
	a := BuildTextIndex("window A")
	b := BuildTextIndex("hello\nworld wide")

	assert.Equal(t, []rune("window A"), a.rs)
	assert.Equal(t, []rune("hello\nworld wide"), b.rs)
	assert.NotSame(t, &a.rs[0], &b.rs[0], "distinct backing arrays per build")
	assert.Equal(t, []rune("cached text"), cached.rs, "the shared cache is untouched")
	assert.Equal(t, []int{5}, b.newlines)
	assert.Empty(t, a.newlines)
}

func TestA11yUTF16Conversion(t *testing.T) {
	// "a😀b": the emoji is one rune but two UTF-16 code units.
	ix := IndexText("a😀b")

	assert.Equal(t, 4, ix.UTF16Len())
	assert.Equal(t, 0, ix.UTF16FromRune(0))
	assert.Equal(t, 1, ix.UTF16FromRune(1))
	assert.Equal(t, 3, ix.UTF16FromRune(2))
	assert.Equal(t, 4, ix.UTF16FromRune(3))
	assert.Equal(t, 4, ix.UTF16FromRune(99), "clamped")

	assert.Equal(t, 0, ix.RuneFromUTF16(0))
	assert.Equal(t, 1, ix.RuneFromUTF16(1))
	assert.Equal(t, 2, ix.RuneFromUTF16(2), "mid-surrogate snaps to the next boundary")
	assert.Equal(t, 2, ix.RuneFromUTF16(3))
	assert.Equal(t, 3, ix.RuneFromUTF16(4))
	assert.Equal(t, 3, ix.RuneFromUTF16(99), "clamped")
	assert.Equal(t, 0, ix.RuneFromUTF16(-1), "clamped")

	assert.Equal(t, "😀b", ix.Substring(1, 3))
	assert.Equal(t, "", ix.Substring(2, 1), "reversed is empty")
}

func TestA11yLineHelpers(t *testing.T) {
	ix := IndexText("ab\ncd\n\nef")

	assert.Equal(t, 0, ix.LineForRune(0))
	assert.Equal(t, 0, ix.LineForRune(2))
	assert.Equal(t, 1, ix.LineForRune(3))
	assert.Equal(t, 2, ix.LineForRune(6))
	assert.Equal(t, 3, ix.LineForRune(9))

	type lineRange struct {
		start, end int
		ok         bool
	}
	get := func(line int) lineRange {
		s, e, ok := ix.LineRuneRange(line)
		return lineRange{s, e, ok}
	}
	assert.Equal(t, lineRange{0, 3, true}, get(0), "includes the newline")
	assert.Equal(t, lineRange{3, 6, true}, get(1))
	assert.Equal(t, lineRange{6, 7, true}, get(2), "empty line")
	assert.Equal(t, lineRange{7, 9, true}, get(3), "unterminated last line")
	assert.Equal(t, lineRange{0, 0, false}, get(4))
	assert.Equal(t, lineRange{0, 0, false}, get(-1))

	empty := IndexText("")
	s, e, ok := empty.LineRuneRange(0)
	assert.True(t, ok)
	assert.Equal(t, 0, s)
	assert.Equal(t, 0, e)
	assert.Equal(t, 0, empty.UTF16Len())
}

// The pre-rebrand tracker #555 perf gate: with the index cached, a screen-reader-style query
// burst on a multi-megabyte document must not re-scan it.
func BenchmarkA11yTextQueriesBigDoc(b *testing.B) {
	text := strings.Repeat("the quick brown fox jumps over the lazy dog\n", 70_000) // ~3 MB
	IndexText(text)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ix := IndexText(text) // identity hit
		u := ix.UTF16FromRune(len(ix.rs) / 2)
		_ = ix.RuneFromUTF16(u)
		_ = ix.LineForRune(len(ix.rs) - 1)
		_, _, _ = ix.LineRuneRange(35_000)
	}
}

func TestA11yTextIndexCache(t *testing.T) {
	text := strings.Repeat("hello\nwörld", 2)
	first := IndexText(text)
	firstRunes := first.rs

	// Same string object: the cached index is reused, no re-decode.
	again := IndexText(text)
	assert.True(t, &firstRunes[0] == &again.rs[0], "identity hit must not re-decode")

	// A fresh string object re-indexes, even with equal content: the gate is
	// identity, because a content compare would itself be O(document).
	fresh := IndexText(strings.Repeat("hello\nwörld", 2))
	assert.False(t, &firstRunes[0] == &fresh.rs[0], "fresh object re-indexes")
	assert.Equal(t, firstRunes, fresh.rs)

	other := IndexText("changed")
	assert.Equal(t, []rune("changed"), other.rs)
	assert.Equal(t, 0, other.LineForRune(3))
}

func TestA11yWordRange(t *testing.T) {
	ix := IndexText("foo  bar,baz")

	word := func(off int) [2]int {
		s, e := ix.WordRuneRange(off)
		return [2]int{s, e}
	}
	assert.Equal(t, [2]int{0, 5}, word(0), "word includes trailing whitespace")
	assert.Equal(t, [2]int{0, 5}, word(2), "mid word")
	assert.Equal(t, [2]int{0, 5}, word(3), "inside the trailing whitespace")
	assert.Equal(t, [2]int{5, 8}, word(6))
	assert.Equal(t, [2]int{8, 9}, word(8), "punctuation is its own unit")
	assert.Equal(t, [2]int{9, 12}, word(11))
	assert.Equal(t, [2]int{9, 12}, word(12), "end of text snaps to last word")

	s, e := IndexText("").WordRuneRange(0)
	assert.Equal(t, 0, s)
	assert.Equal(t, 0, e)
}

func TestA11yUnitBounds(t *testing.T) {
	ix := IndexText("ab cd\nef")

	bounds := func(unit, off int) [2]int {
		s, e := ix.UnitBounds(unit, off)
		return [2]int{s, e}
	}
	assert.Equal(t, [2]int{1, 2}, bounds(UnitCharacter, 1))
	assert.Equal(t, [2]int{8, 8}, bounds(UnitCharacter, 8), "degenerate at end")
	assert.Equal(t, [2]int{3, 6}, bounds(UnitWord, 4), "word runs to the next word start")
	assert.Equal(t, [2]int{0, 6}, bounds(UnitLine, 2), "line includes its newline")
	assert.Equal(t, [2]int{0, 6}, bounds(UnitParagraph, 2), "paragraph maps to line")
	assert.Equal(t, [2]int{6, 8}, bounds(UnitLine, 7))
	assert.Equal(t, [2]int{0, 8}, bounds(UnitDocument, 4))
}

func TestA11yUnitStep(t *testing.T) {
	ix := IndexText("ab cd\nef gh\nij")

	step := func(unit, off, count int) [2]int {
		newOff, moved := ix.UnitStep(unit, off, count)
		return [2]int{newOff, moved}
	}
	assert.Equal(t, [2]int{2, 2}, step(UnitCharacter, 0, 2))
	assert.Equal(t, [2]int{0, -1}, step(UnitCharacter, 1, -5), "clamped at start")
	assert.Equal(t, [2]int{14, 2}, step(UnitCharacter, 12, 5), "clamped at end")

	assert.Equal(t, [2]int{3, 1}, step(UnitWord, 0, 1), "next unit start")
	assert.Equal(t, [2]int{0, -1}, step(UnitWord, 1, -1), "mid word snaps to its start")
	assert.Equal(t, [2]int{3, -1}, step(UnitWord, 5, -1))

	assert.Equal(t, [2]int{6, 1}, step(UnitLine, 0, 1))
	assert.Equal(t, [2]int{12, 2}, step(UnitLine, 0, 2))
	assert.Equal(t, [2]int{12, 1}, step(UnitLine, 6, 5), "stops at last line")
	assert.Equal(t, [2]int{6, -1}, step(UnitLine, 8, -1), "snaps to line start")
	assert.Equal(t, [2]int{0, -2}, step(UnitLine, 8, -5))

	assert.Equal(t, [2]int{14, 1}, step(UnitDocument, 3, 1))
	assert.Equal(t, [2]int{0, -1}, step(UnitDocument, 3, -1))
	assert.Equal(t, [2]int{0, 0}, step(UnitDocument, 0, -1), "already at start")
}

// Character unit = grapheme cluster (pre-rebrand tracker #991): screen-reader character stepping
// must move one user-perceived character, matching Android's BreakIterator
// semantics — an emoji ZWJ family, a flag pair or a letter plus combining
// marks is one step, and no offset ever lands inside a cluster.
func TestTextIndex_GraphemeCharacterUnit(t *testing.T) {
	// "a", e+combining-acute (2 runes), ZWJ family (5 runes), flag (2 runes), "b"
	family := "\U0001F468‍\U0001F469‍\U0001F467" // 👨‍👩‍👧
	flag := "\U0001F1E9\U0001F1EA"               // 🇩🇪
	text := "a" + "é" + family + flag + "b"
	ix := IndexText(text)

	// Cluster starts: a=0, é=1, family=3, flag=8, b=10 (rune offsets).
	wantStarts := []int{0, 1, 3, 8, 10}
	total := len([]rune(text))

	// Forward stepping visits exactly the cluster boundaries.
	off := 0
	var visited []int
	for {
		next, moved := ix.UnitStep(UnitCharacter, off, 1)
		if moved == 0 {
			break
		}
		visited = append(visited, next)
		off = next
	}
	assert.Equal(t, []int{1, 3, 8, 10, 11}, visited)
	assert.Equal(t, total, off)

	// Backward stepping retraces the starts.
	visited = nil
	for {
		next, moved := ix.UnitStep(UnitCharacter, off, -1)
		if moved == 0 {
			break
		}
		visited = append(visited, next)
		off = next
	}
	assert.Equal(t, []int{10, 8, 3, 1, 0}, visited)

	// Bounds of an offset inside the family cluster snap to the whole cluster.
	s, e := ix.UnitBounds(UnitCharacter, 5)
	assert.Equal(t, 3, s)
	assert.Equal(t, 8, e)
	s, e = ix.GraphemeBounds(1)
	assert.Equal(t, 1, s)
	assert.Equal(t, 3, e, "letter + combining mark is one cluster")
	for i, want := range wantStarts {
		gs, _ := ix.GraphemeBounds(want)
		assert.Equal(t, want, gs, "start %d", i)
	}

	// Stepping by several units at once crosses whole clusters.
	next, moved := ix.UnitStep(UnitCharacter, 0, 3)
	assert.Equal(t, 8, next)
	assert.Equal(t, 3, moved)
}

func TestTextIndex_GraphemeFastPathASCII(t *testing.T) {
	ix := IndexText("plain ascii text")
	assert.False(t, ix.maybeClusters, "no combining-capable runes → fast path")
	assert.Nil(t, ix.graphemeStarts())

	// Fast path still steps per rune.
	next, moved := ix.UnitStep(UnitCharacter, 0, 5)
	assert.Equal(t, 5, next)
	assert.Equal(t, 5, moved)
	s, e := ix.UnitBounds(UnitCharacter, 3)
	assert.Equal(t, 3, s)
	assert.Equal(t, 4, e)
}

func TestTextIndex_GraphemeCRLF(t *testing.T) {
	ix := IndexText("a\r\nb")
	// "\r\n" is a single cluster per UAX #29.
	s, e := ix.GraphemeBounds(1)
	assert.Equal(t, 1, s)
	assert.Equal(t, 3, e)
	next, moved := ix.UnitStep(UnitCharacter, 1, 1)
	assert.Equal(t, 3, next)
	assert.Equal(t, 1, moved)
}
