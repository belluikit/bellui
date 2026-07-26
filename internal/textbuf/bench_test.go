package textbuf

import (
	"strings"
	"testing"
)

// benchDoc builds a document of roughly size bytes out of ~62-byte lines,
// matching the widget/entry_edit_bench_test.go corpus shape.
func benchDoc(size int) string {
	const line = "the quick brown fox jumps over the lazy dog line content here\n"
	return strings.Repeat(line, size/len(line)+1)
}

var benchSizes = []struct {
	name string
	size int
}{
	{"3MB", 3 << 20},
	{"10MB", 10 << 20},
	{"50MB", 50 << 20},
}

// benchmarkSplice types one rune at pos, then deletes it again, so the
// document size is stable across iterations.
func benchmarkSplice(b *testing.B, size int, posOf func(n int) int) {
	buf := FromString(benchDoc(size))
	pos := posOf(buf.Len())
	ins := []rune{'x'}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Splice(pos, pos, ins)
		buf.Splice(pos, pos+1, nil)
	}
}

func BenchmarkSpliceHead(b *testing.B) {
	for _, s := range benchSizes {
		b.Run(s.name, func(b *testing.B) { benchmarkSplice(b, s.size, func(int) int { return 0 }) })
	}
}

func BenchmarkSpliceMid(b *testing.B) {
	for _, s := range benchSizes {
		b.Run(s.name, func(b *testing.B) { benchmarkSplice(b, s.size, func(n int) int { return n / 2 }) })
	}
}

func BenchmarkSpliceTail(b *testing.B) {
	for _, s := range benchSizes {
		b.Run(s.name, func(b *testing.B) { benchmarkSplice(b, s.size, func(n int) int { return n }) })
	}
}

// BenchmarkStringAfterSplice measures the per-keystroke materialization cost a
// widget edit pays: one splice followed by String(). With the flat backing the
// splice maintains the memo eagerly; the rope backing (#403) moves the same
// O(n) into the String() miss — either way this is the mandated per-keystroke
// floor to compare against.
func BenchmarkStringAfterSplice(b *testing.B) {
	for _, s := range benchSizes {
		b.Run(s.name, func(b *testing.B) {
			buf := FromString(benchDoc(s.size))
			ins := []rune{'x'}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				buf.Splice(0, 0, ins)
				_ = buf.String()
				buf.Splice(0, 1, nil)
				_ = buf.String()
			}
		})
	}
}

func BenchmarkFromString(b *testing.B) {
	for _, s := range benchSizes {
		doc := benchDoc(s.size)
		b.Run(s.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				buf := FromString(doc)
				if buf.Len() == 0 {
					b.Fatal("empty")
				}
			}
		})
	}
}

func BenchmarkLineOfPos(b *testing.B) {
	buf := FromString(benchDoc(10 << 20))
	n := buf.Len()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = buf.LineOfPos((i * 7919) % n)
	}
}
