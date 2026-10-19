// Package textbuf provides the editable text storage backing the lazy
// single-segment editing path of widget.Entry / widget.RichText (pre-rebrand tracker #381).
//
// A Buffer is addressed by rune offset and keeps a logical-line index
// (newline-delimited) alongside the content. Its String method is memoized:
// repeated calls without an intervening splice return the same string object,
// which callers use as an O(1) pointer-equality identity anchor
// (seg.Text == buf.String()) to detect external writes.
//
// Buffers are not safe for concurrent use; like all widget state they must be
// accessed from the main goroutine only.
package textbuf

import (
	"bytes"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	// chunkMax is the chunk capacity in bytes (never split mid-rune).
	chunkMax = 4096
	// chunkMin is the re-chunk lower bound: every chunk except possibly the
	// last holds at least chunkMin bytes, so the chunk count stays bounded by
	// ~2n/chunkMax and never degrades with edit count.
	chunkMin = chunkMax / 2
)

// counts is a (bytes, runes, newlines) triple.
type counts struct {
	bytes, runes, lines int
}

// chunk is one fixed-capacity UTF-8 piece of the content. data never splits a
// UTF-8 sequence; runes and lines are its rune and '\n' counts.
type chunk struct {
	data  []byte
	runes int
	lines int
}

// Buffer is an editable UTF-8 text buffer: a flat-spine rope of fixed-capacity
// byte chunks with per-chunk rune and newline counts, addressed by rune
// offset. Edits are O(edit + chunkMax) plus an O(chunks) deferred index
// rebuild; the full-string materialization is one alloc + memcpy (the payload
// is already UTF-8 — nothing is re-encoded).
type Buffer struct {
	chunks []chunk
	cum    []counts // cumulative totals before chunk i; len(chunks)+1 when cumOK
	cumOK  bool
	memo   string // memoized String() materialization
	memoOK bool
	total  counts
	gen    uint64 // bumped on every mutation
}

// New returns an empty Buffer.
func New() *Buffer {
	return &Buffer{memoOK: true}
}

// FromString builds a Buffer holding s in one pass (chunk copy + counts; no
// []rune decode). The memo is seeded with s itself, so the first String call
// returns the same string object that was passed in. Invalid UTF-8 in s is
// normalized to U+FFFD, exactly as []rune(s) does.
func FromString(s string) *Buffer {
	if !utf8.ValidString(s) {
		s = string([]rune(s))
	}
	b := &Buffer{memo: s, memoOK: true}
	b.chunks = appendCut(nil, s)
	for _, c := range b.chunks {
		b.total.bytes += len(c.data)
		b.total.runes += c.runes
		b.total.lines += c.lines
	}
	return b
}

// Len returns the content length in runes. O(1).
func (b *Buffer) Len() int {
	return b.total.runes
}

// Gen returns a generation counter that increases on every mutation.
func (b *Buffer) Gen() uint64 {
	return b.gen
}

// String materializes the whole buffer. Memoized: repeated calls without an
// intervening splice return the same string object — the identity contract
// widget.RichText's fast edit path relies on. A miss costs one allocation and
// a chunk-walk memcpy of the content bytes.
func (b *Buffer) String() string {
	if b.memoOK {
		return b.memo
	}
	var sb strings.Builder
	sb.Grow(b.total.bytes)
	for _, c := range b.chunks {
		sb.Write(c.data)
	}
	b.memo = sb.String()
	b.memoOK = true
	return b.memo
}

// LineCount returns the number of logical (newline-delimited) lines. An empty
// buffer has one empty line. O(1).
func (b *Buffer) LineCount() int {
	return b.total.lines + 1
}

// Splice replaces the rune range [low, high) with ins. Out-of-range bounds are
// clamped. Runes that are not valid Unicode scalar values are stored as U+FFFD,
// matching how string(ins) would encode them.
func (b *Buffer) Splice(low, high int, ins []rune) {
	insRunes := normalizeRunes(ins)
	insStr := ""
	if len(insRunes) > 0 {
		insStr = string(insRunes)
	}
	b.splice(low, high, insStr)
}

// SpliceString is Splice with the insertion provided as a string, avoiding a
// redundant re-encode of ins when the caller already has one.
func (b *Buffer) SpliceString(low, high int, ins string) {
	if !utf8.ValidString(ins) {
		ins = string([]rune(ins))
	}
	b.splice(low, high, ins)
}

// splice replaces the (clamped) rune range [low, high) with the valid-UTF-8
// insertion ins. One rule covers every mutation: the touched chunk range is
// rebuilt as head ++ ins ++ tail and re-cut; interior chunks of a large
// deletion are dropped whole, never copied.
func (b *Buffer) splice(low, high int, ins string) {
	n := b.total.runes
	if low < 0 {
		low = 0
	}
	if high > n {
		high = n
	}
	if low > high {
		low = high
	}

	b.ensureCum()
	ci, byteLow := b.locate(low)
	cj, byteHigh := b.locate(high)

	// Rebuild the touched range: head of chunk ci, the insertion, tail of
	// chunk cj. If the result would leave an undersized non-final chunk,
	// absorb the right neighbor into the rebuild (the entire merge policy).
	end := cj + 1
	if end > len(b.chunks) {
		end = len(b.chunks)
	}
	var head, tail []byte
	if ci < len(b.chunks) {
		head = b.chunks[ci].data[:byteLow]
	}
	if cj < len(b.chunks) {
		tail = b.chunks[cj].data[byteHigh:]
	}
	tmpLen := len(head) + len(ins) + len(tail)
	if tmpLen > 0 && tmpLen < chunkMin && end < len(b.chunks) {
		tail = append(append([]byte{}, tail...), b.chunks[end].data...)
		end++
		tmpLen = len(head) + len(ins) + len(tail)
	}
	tmp := make([]byte, 0, tmpLen)
	tmp = append(tmp, head...)
	tmp = append(tmp, ins...)
	tmp = append(tmp, tail...)

	start := ci
	if start > len(b.chunks) {
		start = len(b.chunks)
	}
	b.chunks = spliceSlice(b.chunks, start, end, appendCut(nil, string(tmp)))

	b.total = counts{}
	for _, c := range b.chunks {
		b.total.bytes += len(c.data)
		b.total.runes += c.runes
		b.total.lines += c.lines
	}
	b.cumOK = false
	b.memoOK = false
	b.gen++
}

// RuneAt returns the rune at position pos. Panics if pos is out of range.
func (b *Buffer) RuneAt(pos int) rune {
	b.ensureCum()
	ci, bo := b.locate(pos)
	r, _ := utf8.DecodeRune(b.chunks[ci].data[bo:])
	return r
}

// AppendRunes appends the rune range [begin, end) to dst and returns the
// extended slice, letting callers reuse a scratch buffer. Panics if the range
// is invalid.
func (b *Buffer) AppendRunes(dst []rune, begin, end int) []rune {
	if begin < 0 || end < begin || end > b.total.runes {
		panic("textbuf: rune range out of bounds")
	}
	if begin == end {
		return dst
	}
	b.ensureCum()
	ci, bo := b.locate(begin)
	remaining := end - begin
	for remaining > 0 {
		data := b.chunks[ci].data[bo:]
		for len(data) > 0 && remaining > 0 {
			r, sz := utf8.DecodeRune(data)
			dst = append(dst, r)
			data = data[sz:]
			remaining--
		}
		ci++
		bo = 0
	}
	return dst
}

// Slice returns a copy of the rune range [begin, end). The result does not
// alias the buffer and stays valid across later splices.
func (b *Buffer) Slice(begin, end int) []rune {
	return b.AppendRunes(make([]rune, 0, end-begin), begin, end)
}

// Substring returns the text of the rune range [begin, end) as a string: from
// the memo when it is clean, otherwise gathered from the chunks. The whole
// range is the memo itself (the string-identity contract), but any part of it
// is a copy with memory of its own. A zero-copy slice shares the memo's
// memory, and callers keep short ones (row text, line caches) long after the
// next edit replaces the memo: each kept a whole document alive, a full copy
// per keystroke (#508).
func (b *Buffer) Substring(begin, end int) string {
	if begin < 0 {
		begin = 0
	}
	if end > b.total.runes {
		end = b.total.runes
	}
	if begin >= end {
		return ""
	}
	b.ensureCum()
	ci, bo := b.locate(begin)
	cj, bhi := b.locate(end)
	byteBegin := b.cum[ci].bytes + bo
	byteEnd := b.total.bytes
	if cj < len(b.chunks) {
		byteEnd = b.cum[cj].bytes + bhi
	}
	if b.memoOK {
		if byteBegin == 0 && byteEnd == len(b.memo) {
			return b.memo
		}
		return strings.Clone(b.memo[byteBegin:byteEnd])
	}
	var sb strings.Builder
	sb.Grow(byteEnd - byteBegin)
	for ; ci < len(b.chunks) && b.cum[ci].bytes < byteEnd; ci++ {
		data := b.chunks[ci].data
		lo := 0
		if b.cum[ci].bytes < byteBegin {
			lo = byteBegin - b.cum[ci].bytes
		}
		hi := len(data)
		if b.cum[ci].bytes+hi > byteEnd {
			hi = byteEnd - b.cum[ci].bytes
		}
		sb.Write(data[lo:hi])
	}
	return sb.String()
}

// LineStart returns the rune offset of line i's first rune. Panics if i is
// out of range. O(log chunks + one chunk scan), newline search vectorized.
func (b *Buffer) LineStart(i int) int {
	if i < 0 || i > b.total.lines {
		panic("textbuf: line index out of range")
	}
	if i == 0 {
		return 0
	}
	b.ensureCum()
	// Find the chunk containing the i-th newline (1-based).
	ci := sort.Search(len(b.chunks), func(k int) bool { return b.cum[k+1].lines >= i }) //nolint:gosec
	c := &b.chunks[ci]
	need := i - b.cum[ci].lines
	off := 0
	for ; need > 0; need-- {
		off += bytes.IndexByte(c.data[off:], '\n') + 1
	}
	if c.runes == len(c.data) { // ASCII fast path
		return b.cum[ci].runes + off
	}
	return b.cum[ci].runes + utf8.RuneCount(c.data[:off])
}

// LineOfPos returns the logical line containing rune position pos: the largest
// line whose start is <= pos. Positions past the end map to the last line.
func (b *Buffer) LineOfPos(pos int) int {
	if pos <= 0 {
		return 0
	}
	if pos >= b.total.runes {
		return b.total.lines
	}
	b.ensureCum()
	ci, bo := b.locate(pos)
	return b.cum[ci].lines + bytes.Count(b.chunks[ci].data[:bo], []byte{'\n'})
}

// ForEachLine calls fn for every logical line with its rune range [start, end)
// excluding the trailing newline, stopping early if fn returns false. One
// O(bytes) chunk walk using vectorized newline search and rune counting — no
// per-line binary searches or per-rune decode loops.
func (b *Buffer) ForEachLine(fn func(line, start, end int) bool) {
	line, start, off := 0, 0, 0
	for _, c := range b.chunks {
		if c.lines == 0 { // no newline: advance the rune offset wholesale
			off += c.runes
			continue
		}
		ascii := c.runes == len(c.data)
		data := c.data
		for {
			nl := bytes.IndexByte(data, '\n')
			if nl < 0 {
				if ascii {
					off += len(data)
				} else {
					off += utf8.RuneCount(data)
				}
				break
			}
			if ascii {
				off += nl
			} else {
				off += utf8.RuneCount(data[:nl])
			}
			if !fn(line, start, off) {
				return
			}
			line++
			off++ // the newline itself
			start = off
			data = data[nl+1:]
		}
	}
	fn(line, start, off)
}

// ensureCum rebuilds the cumulative index after an edit. O(chunks).
func (b *Buffer) ensureCum() {
	if b.cumOK {
		return
	}
	if cap(b.cum) >= len(b.chunks)+1 {
		b.cum = b.cum[:len(b.chunks)+1]
	} else {
		b.cum = make([]counts, len(b.chunks)+1)
	}
	acc := counts{}
	for i, c := range b.chunks {
		b.cum[i] = acc
		acc.bytes += len(c.data)
		acc.runes += c.runes
		acc.lines += c.lines
	}
	b.cum[len(b.chunks)] = acc
	b.cumOK = true
}

// locate returns the chunk index containing rune position pos and pos's byte
// offset within it. pos == Len() locates the end of the last chunk, so an
// append-at-end splice extends that chunk instead of growing a parade of tiny
// trailing chunks. Requires cumOK.
func (b *Buffer) locate(pos int) (ci, byteOff int) {
	if pos >= b.total.runes {
		if len(b.chunks) == 0 {
			return 0, 0
		}
		ci = len(b.chunks) - 1
		return ci, len(b.chunks[ci].data)
	}
	ci = sort.Search(len(b.chunks), func(k int) bool { return b.cum[k+1].runes > pos }) //nolint:gosec
	c := &b.chunks[ci]
	rel := pos - b.cum[ci].runes
	if c.runes == len(c.data) { // ASCII fast path
		return ci, rel
	}
	off := 0
	for ; rel > 0; rel-- {
		_, sz := utf8.DecodeRune(c.data[off:])
		off += sz
	}
	return ci, off
}

// appendCut cuts s (valid UTF-8) into chunks of at most chunkMax bytes at rune
// boundaries, evening the final two pieces so no piece except a sole survivor
// is smaller than chunkMin, and appends them to dst.
func appendCut(dst []chunk, s string) []chunk {
	for len(s) > 0 {
		cut := len(s)
		if cut > chunkMax {
			if len(s) <= chunkMax+chunkMin {
				cut = len(s) / 2 // even the last two pieces
			} else {
				cut = chunkMax
			}
			for cut > 0 && !utf8.RuneStart(s[cut]) {
				cut--
			}
			if cut == 0 { // degenerate; cannot happen for valid UTF-8 with chunkMax >= 4
				cut = len(s)
			}
		}
		piece := s[:cut]
		dst = append(dst, chunk{
			data:  []byte(piece),
			runes: utf8.RuneCountInString(piece),
			lines: strings.Count(piece, "\n"),
		})
		s = s[cut:]
	}
	return dst
}

// normalizeRunes returns ins with any rune that is not a valid Unicode scalar
// value replaced by U+FFFD, so the stored bytes always round-trip through the
// rune forms unchanged. The common all-valid case returns ins as-is.
func normalizeRunes(ins []rune) []rune {
	for i, r := range ins {
		if utf8.RuneLen(r) < 0 {
			out := make([]rune, len(ins))
			copy(out, ins[:i])
			for j := i; j < len(ins); j++ {
				if utf8.RuneLen(ins[j]) < 0 {
					out[j] = utf8.RuneError
				} else {
					out[j] = ins[j]
				}
			}
			return out
		}
	}
	return ins
}

// spliceSlice replaces s[low:high] with ins, reusing s's backing array when it
// has the capacity. Go's copy is memmove-safe, so the overlapping tail shift is
// correct.
func spliceSlice[T any](s []T, low, high int, ins []T) []T {
	newLen := low + len(ins) + (len(s) - high)
	if newLen <= cap(s) {
		tail := s[high:]
		s = s[:newLen]
		copy(s[low+len(ins):], tail) // shift tail (overlap-safe)
		copy(s[low:], ins)
		return s
	}
	ns := make([]T, newLen)
	copy(ns, s[:low])
	copy(ns[low:], ins)
	copy(ns[low+len(ins):], s[high:])
	return ns
}
