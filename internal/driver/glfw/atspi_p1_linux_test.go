//go:build accessibility && linux && !android && !wasm && !js

package glfw

import (
	"testing"

	"github.com/belluikit/bellui/internal/accessibility"
)

func TestTextRuneDiff(t *testing.T) {
	cases := []struct {
		name           string
		prev, cur      string
		delOff, delLen int
		delStr         string
		insOff, insLen int
		insStr         string
	}{
		{"append", "hel", "hello", 3, 0, "", 3, 2, "lo"},
		{"delete-tail", "hello", "hel", 3, 2, "lo", 3, 0, ""},
		{"insert-middle", "helo", "hello", 3, 0, "", 3, 1, "l"},
		{"replace", "cat", "cot", 1, 1, "a", 1, 1, "o"},
		{"no-change", "same", "same", 4, 0, "", 4, 0, ""},
		{"unicode-append", "café", "caféé", 4, 0, "", 4, 1, "é"},
		{"full-replace", "abc", "xyz", 0, 3, "abc", 0, 3, "xyz"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			del, ins := textRuneDiff(c.prev, c.cur)
			if del.offset != c.delOff || del.length != c.delLen || del.str != c.delStr {
				t.Errorf("deleted = %+v; want off=%d len=%d str=%q", del, c.delOff, c.delLen, c.delStr)
			}
			if ins.offset != c.insOff || ins.length != c.insLen || ins.str != c.insStr {
				t.Errorf("inserted = %+v; want off=%d len=%d str=%q", ins, c.insOff, c.insLen, c.insStr)
			}
		})
	}
}

func TestGranToUnit(t *testing.T) {
	cases := map[uint32]int{
		0: accessibility.UnitCharacter, // CHAR
		1: accessibility.UnitWord,      // WORD
		2: accessibility.UnitLine,      // SENTENCE → line (closest modelled)
		3: accessibility.UnitLine,      // LINE
		4: accessibility.UnitParagraph, // PARAGRAPH
		9: accessibility.UnitLine,      // unknown → line
	}
	for gran, want := range cases {
		if got := granToUnit(gran); got != want {
			t.Errorf("granToUnit(%d) = %d; want %d", gran, got, want)
		}
	}
}

// TestBuildTextIndex_PerNodeIndependent guards the pre-rebrand tracker #986 aliasing fix at the
// consumer: the Linux backend keeps a per-node text index for every exposed
// field at once, so building one index must not disturb another (the shared
// cache accessibility.IndexText returns would alias them — correct only for
// the focused-field query model the darwin/windows backends use). The
// pointer-level backing-array check lives with the index
// (accessibility.TestBuildTextIndex_Standalone); this covers the observable
// contract through the exported API.
func TestBuildTextIndex_PerNodeIndependent(t *testing.T) {
	a := accessibility.BuildTextIndex("window A")
	b := accessibility.BuildTextIndex("hello\nworld wide")

	if got := a.Substring(0, a.RuneLen()); got != "window A" {
		t.Errorf("index a content = %q; want %q (aliased?)", got, "window A")
	}
	if got := b.Substring(0, b.RuneLen()); got != "hello\nworld wide" {
		t.Errorf("index b content = %q", got)
	}
	if a.RuneLen() != 8 || b.RuneLen() != 16 {
		t.Errorf("lengths a=%d b=%d; want 8, 16", a.RuneLen(), b.RuneLen())
	}
	// Newlines are tracked per index, not shared.
	if s, e, ok := a.LineRuneRange(0); !ok || s != 0 || e != 8 {
		t.Errorf("index a line 0 = [%d, %d) ok=%v; want the whole single line", s, e, ok)
	}
	if s, e, ok := b.LineRuneRange(1); !ok || s != 6 || e != 16 {
		t.Errorf("index b line 1 = [%d, %d) ok=%v; want [6, 16)", s, e, ok)
	}
}

func TestRuneLen_NilSafe(t *testing.T) {
	var ix *accessibility.TextIndex
	if ix.RuneLen() != 0 {
		t.Error("nil index RuneLen should be 0")
	}
	if got := accessibility.IndexText("héllo").RuneLen(); got != 5 {
		t.Errorf("RuneLen(héllo) = %d; want 5", got)
	}
}
