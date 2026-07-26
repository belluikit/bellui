package model

import (
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSortProxy_NoWorkPerFrame is the O(visible)-stays-O(visible) gate: once the
// permutation is built, reading every item across many "frames" must not invoke
// the comparator at all. Sorting happens only on a data change or SetLess.
func TestSortProxy_NoWorkPerFrame(t *testing.T) {
	src := NewSliceModel[int]()
	items := make([]int, 5000)
	for i := range items {
		items[i] = len(items) - i
	}
	src.Set(items)

	var cmpCalls int64
	p := NewSortProxy[int](src)
	p.SetLess(func(a, b int) bool {
		atomic.AddInt64(&cmpCalls, 1)
		return a < b
	})

	built := atomic.LoadInt64(&cmpCalls)
	assert.Positive(t, built, "initial sort invokes the comparator")

	// Simulate many frames of a widget reading its visible (here: all) rows.
	for frame := 0; frame < 1000; frame++ {
		n := p.Len()
		for i := 0; i < n; i++ {
			_ = p.Item(i)
		}
	}
	assert.Equal(t, built, atomic.LoadInt64(&cmpCalls),
		"reading items must not re-sort — no per-frame comparator calls")
}

// TestFilterProxy_NoWorkPerFrame is the same gate for the filter predicate.
func TestFilterProxy_NoWorkPerFrame(t *testing.T) {
	src := NewSliceModel[int]()
	items := make([]int, 5000)
	for i := range items {
		items[i] = i
	}
	src.Set(items)

	var predCalls int64
	p := NewFilterProxy[int](src)
	p.SetPredicate(func(v int) bool {
		atomic.AddInt64(&predCalls, 1)
		return v%2 == 0
	})

	built := atomic.LoadInt64(&predCalls)
	assert.Positive(t, built)

	for frame := 0; frame < 1000; frame++ {
		n := p.Len()
		for i := 0; i < n; i++ {
			_ = p.Item(i)
		}
	}
	assert.Equal(t, built, atomic.LoadInt64(&predCalls),
		"reading items must not re-filter — no per-frame predicate calls")
}

func makeBigModel(n int) *SliceModel[string] {
	items := make([]string, n)
	for i := range items {
		// Reverse-ish keys so the sort has real work to do.
		items[i] = strconv.Itoa(n - i)
	}
	return NewSliceModel(items...)
}

// BenchmarkSortProxyRebuild measures the once-per-change O(n log n) cost.
func BenchmarkSortProxyRebuild(b *testing.B) {
	src := makeBigModel(100_000)
	p := NewSortProxy[string](src)
	less := func(a, b string) bool { return a < b }
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.SetLess(less) // forces a full rebuild
	}
}

// BenchmarkSortProxyFrameWalk measures the per-frame cost of reading all rows —
// this is what must stay flat regardless of model size.
func BenchmarkSortProxyFrameWalk(b *testing.B) {
	src := makeBigModel(100_000)
	p := NewSortProxy[string](src)
	p.SetLess(func(a, b string) bool { return a < b })
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		n := p.Len()
		for j := 0; j < n; j++ {
			_ = p.Item(j)
		}
	}
}
