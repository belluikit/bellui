package textbuf

import (
	"strings"
	"testing"
	"unicode/utf8"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// model is the naive oracle: a plain string spliced by full re-encode. Every
// Buffer query is checked against it after each operation.
type model struct {
	s string
}

func (m *model) splice(low, high int, ins []rune) {
	r := []rune(m.s)
	n := len(r)
	if low < 0 {
		low = 0
	}
	if high > n {
		high = n
	}
	if low > high {
		low = high
	}
	m.s = string(r[:low]) + string(ins) + string(r[high:])
}

func (m *model) runes() []rune { return []rune(m.s) }

func (m *model) lineStarts() []int {
	starts := []int{0}
	for i, r := range m.runes() {
		if r == '\n' {
			starts = append(starts, i+1)
		}
	}
	return starts
}

// checkAgainstModel asserts every Buffer accessor agrees with the oracle.
func checkAgainstModel(t *testing.T, b *Buffer, m *model) {
	t.Helper()
	require.Equal(t, m.s, b.String(), "String")
	runes := m.runes()
	require.Equal(t, len(runes), b.Len(), "Len")

	starts := m.lineStarts()
	require.Equal(t, len(starts), b.LineCount(), "LineCount")
	for i, want := range starts {
		require.Equal(t, want, b.LineStart(i), "LineStart(%d)", i)
	}

	// LineOfPos at every line boundary, one inside each line, and past-end.
	for i, start := range starts {
		require.Equal(t, i, b.LineOfPos(start), "LineOfPos(start of %d)", i)
		end := len(runes)
		if i+1 < len(starts) {
			end = starts[i+1] - 1
		}
		if end > start {
			require.Equal(t, i, b.LineOfPos(start+(end-start)/2), "LineOfPos(mid of %d)", i)
		}
	}
	require.Equal(t, len(starts)-1, b.LineOfPos(len(runes)+5), "LineOfPos(past end)")

	// Slice / Substring / AppendRunes / RuneAt over a few ranges.
	for _, r := range [][2]int{{0, len(runes)}, {0, len(runes) / 2}, {len(runes) / 3, 2 * len(runes) / 3}} {
		begin, end := r[0], r[1]
		if begin > end {
			continue
		}
		want := string(runes[begin:end])
		require.Equal(t, want, string(b.Slice(begin, end)), "Slice(%d,%d)", begin, end)
		require.Equal(t, want, b.Substring(begin, end), "Substring(%d,%d)", begin, end)
		require.Equal(t, "!"+want, string(b.AppendRunes([]rune{'!'}, begin, end)), "AppendRunes(%d,%d)", begin, end)
	}
	for _, pos := range []int{0, len(runes) / 2, len(runes) - 1} {
		if pos >= 0 && pos < len(runes) {
			require.Equal(t, runes[pos], b.RuneAt(pos), "RuneAt(%d)", pos)
		}
	}

	// ForEachLine agrees with the oracle's line ranges (newline excluded).
	line := 0
	b.ForEachLine(func(i, start, end int) bool {
		require.Equal(t, line, i, "ForEachLine index")
		wantEnd := len(runes)
		if i+1 < len(starts) {
			wantEnd = starts[i+1] - 1
		}
		require.Equal(t, starts[i], start, "ForEachLine start of %d", i)
		require.Equal(t, wantEnd, end, "ForEachLine end of %d", i)
		line++
		return true
	})
	require.Equal(t, len(starts), line, "ForEachLine count")
}

type spliceOp struct {
	low, high int
	ins       string
}

// scriptedOps exercises head/mid/tail edits, newline insertion and removal,
// multi-byte runes, whole-buffer deletion and empty-buffer edits. Positions are
// in runes against the evolving content of TestOracle_Scripted's initial text.
var scriptedOps = []spliceOp{
	{0, 0, "start "},           // head insert
	{6, 6, "middle"},           // mid insert
	{0, 0, "é界"},               // multi-byte head insert
	{2, 2, "\n"},               // newline insert (splits line 0)
	{0, 1, ""},                 // head delete (multi-byte é)
	{5, 9, "xyz"},              // replace across a boundary region
	{3, 3, "a\nb\nc"},          // multi-newline insert
	{1, 8, ""},                 // delete spanning newlines (merges lines)
	{0, 0, "🙂🙃"},               // emoji (4-byte runes)
	{4, 4, "\n\n"},             // consecutive newlines (empty line)
	{2, 3, "д"},                // replace with cyrillic
	{-3, 2, "clamped"},         // low clamp
	{5, 99999, "tail-clamp"},   // high clamp
	{9, 4, "low>high"},         // inverted range clamp (insert at 4)
	{0, 99999, ""},             // whole-buffer delete
	{0, 0, ""},                 // no-op on empty
	{0, 0, "rebuilt\ncontent"}, // refill
}

func TestOracle_Scripted(t *testing.T) {
	for name, initial := range map[string]string{
		"empty":     "",
		"plain":     "the quick brown fox",
		"multiline": "line one\nline two\nline three\n",
		"multibyte": "aé界b\ncд\nz",
	} {
		t.Run(name, func(t *testing.T) {
			b := FromString(initial)
			m := &model{s: initial}
			checkAgainstModel(t, b, m)
			gen := b.Gen()
			for i, op := range scriptedOps {
				b.Splice(op.low, op.high, []rune(op.ins))
				m.splice(op.low, op.high, []rune(op.ins))
				checkAgainstModel(t, b, m)
				assert.Greater(t, b.Gen(), gen, "op %d must bump Gen", i)
				gen = b.Gen()
			}
		})
	}
}

func TestOracle_SpliceString(t *testing.T) {
	b := FromString("hello\nworld")
	m := &model{s: "hello\nworld"}
	for _, op := range scriptedOps {
		b.SpliceString(op.low, op.high, op.ins)
		m.splice(op.low, op.high, []rune(op.ins))
		checkAgainstModel(t, b, m)
	}
}

// TestOracle_InvalidRunes pins the U+FFFD parity contract: invalid runes in
// ins are stored and encoded exactly as string(ins) would encode them.
func TestOracle_InvalidRunes(t *testing.T) {
	invalid := []rune{'a', 0xD800, 'b', -1, utf8.MaxRune + 1, '\n', 'c'}
	b := FromString("xy")
	m := &model{s: "xy"}

	b.Splice(1, 1, invalid)
	// The oracle stores what string(ins) produces; []rune of that string is
	// the normalized form, which the Buffer must match on every accessor.
	m.splice(1, 1, []rune(string(invalid)))
	checkAgainstModel(t, b, m)
	assert.Equal(t, "a�b��\nc", string(b.Slice(1, 8)))
}

// TestOracle_InvalidUTF8Input pins FromString on non-UTF-8 input: content is
// normalized like []rune(s), and the memo matches it (no aliasing of the
// invalid original).
func TestOracle_InvalidUTF8Input(t *testing.T) {
	s := "a\xffb\nc"
	b := FromString(s)
	m := &model{s: string([]rune(s))}
	checkAgainstModel(t, b, m)
}

// strData returns the address of a string's backing bytes for identity
// comparison (as uintptr — testify would dereference a *byte).
func strData(s string) uintptr {
	return uintptr(unsafe.Pointer(unsafe.StringData(s)))
}

// TestString_MemoIdentity pins the identity contract: repeated String calls
// without an intervening splice return the same string object; FromString
// seeds the memo with the input itself.
func TestString_MemoIdentity(t *testing.T) {
	s := strings.Repeat("some text\n", 100)
	b := FromString(s)
	require.Equal(t, strData(s), strData(b.String()), "FromString must seed the memo with the input")

	first := b.String()
	second := b.String()
	require.Equal(t, strData(first), strData(second), "String must be memoized")

	b.Splice(5, 5, []rune("x"))
	third := b.String()
	assert.NotEqual(t, strData(first), strData(third), "a splice must produce a new string")
	fourth := b.String()
	require.Equal(t, strData(third), strData(fourth), "String must be memoized after edits")

	// Substring of the full range returns the memo itself.
	require.Equal(t, strData(third), strData(b.Substring(0, b.Len())))
}

func TestNew_Empty(t *testing.T) {
	b := New()
	assert.Equal(t, 0, b.Len())
	assert.Equal(t, "", b.String())
	assert.Equal(t, 1, b.LineCount())
	assert.Equal(t, 0, b.LineStart(0))
	assert.Equal(t, 0, b.LineOfPos(0))
	assert.Equal(t, uint64(0), b.Gen())
}

// checkInvariants validates the structural invariants of the chunked-rope
// backing: chunk fill bounds, UTF-8 alignment, per-chunk counts, cumulative
// index consistency and memo/content agreement.
func checkInvariants(t *testing.T, b *Buffer) {
	t.Helper()
	content := b.String()
	require.True(t, utf8.ValidString(content), "content must be valid UTF-8")
	require.Equal(t, utf8.RuneCountInString(content), b.Len(), "Len must match a recount")
	require.Equal(t, strings.Count(content, "\n")+1, b.LineCount(), "LineCount must match a recount")

	var joined strings.Builder
	for i, c := range b.chunks {
		require.NotEmptyf(t, c.data, "chunk %d must not be empty", i)
		require.LessOrEqualf(t, len(c.data), chunkMax, "chunk %d exceeds chunkMax", i)
		if i < len(b.chunks)-1 {
			// The rune-boundary backoff can undershoot chunkMin by at most one
			// partial rune.
			require.GreaterOrEqualf(t, len(c.data), chunkMin-utf8.UTFMax,
				"non-final chunk %d below chunkMin", i)
		}
		require.Truef(t, utf8.Valid(c.data), "chunk %d must hold whole UTF-8 sequences", i)
		require.Equalf(t, utf8.RuneCount(c.data), c.runes, "chunk %d rune count", i)
		require.Equalf(t, strings.Count(string(c.data), "\n"), c.lines, "chunk %d line count", i)
		joined.Write(c.data)
	}
	require.Equal(t, content, joined.String(), "chunks must concatenate to the content")

	b.ensureCum()
	require.Len(t, b.cum, len(b.chunks)+1)
	acc := counts{}
	for i, c := range b.chunks {
		require.Equalf(t, acc, b.cum[i], "cum[%d]", i)
		acc.bytes += len(c.data)
		acc.runes += c.runes
		acc.lines += c.lines
	}
	require.Equal(t, acc, b.cum[len(b.chunks)], "cum total")
	require.Equal(t, acc, b.total, "total counts")
}

// bufLineStarts collects the buffer's line starts via the public API.
func bufLineStarts(b *Buffer) []int {
	starts := make([]int, b.LineCount())
	for i := range starts {
		starts[i] = b.LineStart(i)
	}
	return starts
}

func TestInvariants_AfterScriptedOps(t *testing.T) {
	b := FromString("aé界b\ncд\nz")
	checkInvariants(t, b)
	for _, op := range scriptedOps {
		b.Splice(op.low, op.high, []rune(op.ins))
		checkInvariants(t, b)
	}
}

// TestSubstringOwnsItsMemory pins #508: a substring taken while the memo is
// clean must not point into the memo, or keeping it keeps the whole document.
func TestSubstringOwnsItsMemory(t *testing.T) {
	doc := strings.Repeat("0123456789\n", 10_000)
	b := FromString(doc)
	memo := b.String()
	sub := b.Substring(100, 120)
	require.Equal(t, memo[100:120], sub)

	start := uintptr(unsafe.Pointer(unsafe.StringData(memo)))
	p := uintptr(unsafe.Pointer(unsafe.StringData(sub)))
	assert.False(t, p >= start && p < start+uintptr(len(memo)), "the substring shares the memo's memory")
}
