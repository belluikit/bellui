package widget

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// benchLine is the ~62-byte line the large-document benchmarks are built from.
const benchLine = "the quick brown fox jumps over the lazy dog line content here\n"

// benchDocSizes are the document sizes the large-document benchmarks run at:
// the historical 50k-line (~3 MB) corpus and a 500k-line (~30 MB) multi-MB
// document (#382 Tier 2 acceptance).
var benchDocSizes = []struct {
	name  string
	lines int
}{
	{"50k", 50_000},
	{"500k", 500_000},
}

func benchEntryDoc(lines int) string {
	var sb strings.Builder
	sb.Grow(lines * len(benchLine))
	for i := 0; i < lines; i++ {
		sb.WriteString(benchLine)
	}
	return sb.String()
}

// newBenchEntry builds a shown multi-line entry over a large document with the
// row model built, ready for per-operation measurement.
func newBenchEntry(b *testing.B, lines int) (*Entry, bellui.Window) {
	b.Helper()
	e := NewMultiLineEntry()
	e.SetText(benchEntryDoc(lines))
	w := test.NewWindow(e)
	w.Resize(geo.NewSize(800, 600))
	e.textProvider().updateRowBounds() // force the initial row-bounds build
	return e, w
}

// BenchmarkEntryTypeBigDoc measures single-keystroke latency on a large
// single-segment document, at the document head and at the tail (the tail
// variant pins position independence — the pre-rope backing paid O(cursor
// position) per keystroke). The per-keystroke floor is the single mandated
// document materialization for the exported Entry.Text field (#382).
func BenchmarkEntryTypeBigDoc(b *testing.B) {
	for _, size := range benchDocSizes {
		for _, pos := range []string{"head", "tail"} {
			b.Run(size.name+"/"+pos, func(b *testing.B) {
				e, w := newBenchEntry(b, size.lines)
				defer w.Close()
				if pos == "head" {
					e.CursorRow, e.CursorColumn = 0, 0
				} else {
					e.CursorRow, e.CursorColumn = size.lines-1, 10
				}

				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					e.TypedRune('x')
				}
			})
		}
	}
}

// BenchmarkEntryTypeBigDocDeferred is BenchmarkEntryTypeBigDoc with the
// DeferTextSync mode (#410) on: the per-keystroke document materialization is
// deferred to sync points, so typing latency should be O(edit + window) —
// flat across document sizes — instead of memcpy-bound on the document.
func BenchmarkEntryTypeBigDocDeferred(b *testing.B) {
	for _, size := range benchDocSizes {
		for _, pos := range []string{"head", "tail"} {
			b.Run(size.name+"/"+pos, func(b *testing.B) {
				e, w := newBenchEntry(b, size.lines)
				defer w.Close()
				e.DeferTextSync = true
				e.Refresh() // push the mode to the provider (syncSegments)
				if pos == "head" {
					e.CursorRow, e.CursorColumn = 0, 0
				} else {
					e.CursorRow, e.CursorColumn = size.lines-1, 10
				}

				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					e.TypedRune('x')
				}
			})
		}
	}
}

// BenchmarkEntrySetTextBigDoc measures the initial-load path (fycoder#106):
// SetText plus the first row-bounds build, with a synthetic per-line
// highlighter attached — the windowed highlight build (#401) must keep the
// highlighter out of the load path entirely.
func BenchmarkEntrySetTextBigDoc(b *testing.B) {
	for _, size := range benchDocSizes {
		b.Run(size.name, func(b *testing.B) {
			doc := benchEntryDoc(size.lines)
			e := NewMultiLineEntry()
			e.LineHighlighter = func(line string, state int) ([]StyleRun, int) {
				return []StyleRun{{Text: line}}, state
			}
			w := test.NewWindow(e)
			defer w.Close()
			w.Resize(geo.NewSize(800, 600))

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				e.SetText(doc)
				e.textProvider().updateRowBounds()
			}
		})
	}
}

// BenchmarkEntryUndoRedoBigDoc measures one undo+redo pair. Delta-native
// replay (#401) applies the recorded edit through the incremental splice path;
// before it, each application decoded and re-concatenated the whole document
// and forced a full row-model rebuild.
func BenchmarkEntryUndoRedoBigDoc(b *testing.B) {
	for _, size := range benchDocSizes {
		b.Run(size.name, func(b *testing.B) {
			e, w := newBenchEntry(b, size.lines)
			defer w.Close()
			e.CursorRow, e.CursorColumn = 5, 3
			e.TypedRune('x')

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				e.Undo()
				e.Redo()
			}
		})
	}
}

// BenchmarkEntryComposeBigDoc measures one IME preedit update. Splicing the
// preedit through the buffer (#402) keeps composition on the incremental path;
// before it, every update rebuilt the whole row model.
func BenchmarkEntryComposeBigDoc(b *testing.B) {
	for _, size := range benchDocSizes {
		b.Run(size.name, func(b *testing.B) {
			e, w := newBenchEntry(b, size.lines)
			defer w.Close()
			e.CursorRow, e.CursorColumn = 5, 3

			preedits := [...]string{"に", "にほ", "にほん"}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				e.Compose(&event.PreeditEvent{Text: preedits[i%len(preedits)], CursorPos: 1})
			}
			b.StopTimer()
			e.ComposeCancel()
		})
	}
}

// TestEntryTypeFlatLatency is the #382 Tier 2 acceptance gate: per-keystroke
// cost must scale with the single mandated document materialization (one
// alloc + memcpy for the exported Entry.Text field) and nothing else. It
// asserts allocated bytes/op at 500k lines stay within ~12× of 50k lines
// (10× the document size plus slack) and the alloc COUNT stays flat — the
// stable, hardware-independent formulation of "flat latency". Wall-clock
// ratios are reported but only asserted loosely; run with BELLUI_PERF=1.
func TestEntryTypeFlatLatency(t *testing.T) {
	if os.Getenv("BELLUI_PERF") == "" {
		t.Skip("set BELLUI_PERF=1 to run the latency-scaling gate")
	}

	measure := func(lines int) testing.BenchmarkResult {
		return testing.Benchmark(func(b *testing.B) {
			e, w := newBenchEntry(b, lines)
			defer w.Close()
			e.CursorRow, e.CursorColumn = lines-1, 10 // tail: the position-dependence worst case

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				e.TypedRune('x')
			}
		})
	}

	small := measure(50_000)
	big := measure(500_000)
	t.Logf("50k: %v/op, %d B/op, %d allocs/op", small.NsPerOp(), small.AllocedBytesPerOp(), small.AllocsPerOp())
	t.Logf("500k: %v/op, %d B/op, %d allocs/op", big.NsPerOp(), big.AllocedBytesPerOp(), big.AllocsPerOp())

	byteRatio := float64(big.AllocedBytesPerOp()) / float64(small.AllocedBytesPerOp())
	if byteRatio > 12 {
		t.Errorf("bytes/op grew %.1f× for a 10× document — more than the mandated materialization", byteRatio)
	}
	allocRatio := float64(big.AllocsPerOp()) / float64(small.AllocsPerOp())
	if allocRatio > 1.5 {
		t.Errorf("allocs/op grew %.1f× for a 10× document — per-keystroke allocations must be size-independent", allocRatio)
	}
	nsRatio := float64(big.NsPerOp()) / float64(small.NsPerOp())
	if nsRatio > 15 {
		t.Errorf("ns/op grew %.1f× for a 10× document — expected memcpy-bound scaling (≤~10×)", nsRatio)
	}
	fmt.Printf("flat-latency gate: ns ratio %.2f, bytes ratio %.2f, allocs ratio %.2f\n", nsRatio, byteRatio, allocRatio)
}

// TestEntryTypeFlatLatencyDeferred is the deferred-mode (#410) counterpart of
// TestEntryTypeFlatLatency: with DeferTextSync on there is no mandated
// per-keystroke materialization at all, so a 10× document must cost the same
// per keystroke — truly flat, not merely memcpy-bound. Run with BELLUI_PERF=1.
func TestEntryTypeFlatLatencyDeferred(t *testing.T) {
	if os.Getenv("BELLUI_PERF") == "" {
		t.Skip("set BELLUI_PERF=1 to run the latency-scaling gate")
	}

	// Each iteration types a rune and backspaces it, keeping the edited line's
	// length constant: testing.Benchmark ramps b.N into the thousands, and a
	// line growing by one rune per iteration would make the edited row's
	// shaping cost scale with b.N — an artifact that swamps the document-size
	// signal this gate measures.
	measure := func(lines int) testing.BenchmarkResult {
		return testing.Benchmark(func(b *testing.B) {
			e, w := newBenchEntry(b, lines)
			defer w.Close()
			e.DeferTextSync = true
			e.Refresh()                               // push the mode to the provider (syncSegments)
			e.CursorRow, e.CursorColumn = lines-1, 10 // tail: the position-dependence worst case

			backspace := &event.KeyEvent{Name: event.KeyBackspace}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				e.TypedRune('x')
				e.TypedKey(backspace)
			}
		})
	}

	small := measure(50_000)
	big := measure(500_000)
	t.Logf("50k deferred: %v/op, %d B/op, %d allocs/op", small.NsPerOp(), small.AllocedBytesPerOp(), small.AllocsPerOp())
	t.Logf("500k deferred: %v/op, %d B/op, %d allocs/op", big.NsPerOp(), big.AllocedBytesPerOp(), big.AllocsPerOp())

	// No document-sized term remains, so all three metrics must stay flat for
	// a 10× document. Bounds leave headroom for machine noise, not for any
	// O(document) regression (which shows up as ~10×).
	byteRatio := float64(big.AllocedBytesPerOp()) / float64(small.AllocedBytesPerOp())
	if byteRatio > 3 {
		t.Errorf("deferred bytes/op grew %.1f× for a 10× document — an O(document) cost crept back in", byteRatio)
	}
	allocRatio := float64(big.AllocsPerOp()) / float64(small.AllocsPerOp())
	if allocRatio > 1.5 {
		t.Errorf("deferred allocs/op grew %.1f× for a 10× document — per-keystroke allocations must be size-independent", allocRatio)
	}
	nsRatio := float64(big.NsPerOp()) / float64(small.NsPerOp())
	if nsRatio > 3 {
		t.Errorf("deferred ns/op grew %.1f× for a 10× document — deferred typing must be flat", nsRatio)
	}
	fmt.Printf("deferred flat-latency gate: ns ratio %.2f, bytes ratio %.2f, allocs ratio %.2f\n", nsRatio, byteRatio, allocRatio)
}
