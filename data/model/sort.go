package model

import (
	"sort"
	"sync"

	"github.com/belluikit/bellui/data/binding"
)

// SortProxy wraps a source [Model] and presents its items in a sorted order,
// while remaining a [Model] itself so it can be composed (for example wrapped in
// a [FilterProxy], or bound directly to a widget). It maintains a permutation of
// source indices and rebuilds it once whenever the source changes or the
// comparator is replaced — never per displayed item — so a bound widget keeps
// its O(visible) per-frame cost.
//
// Since: 2.8
type SortProxy[T any] struct {
	listeners

	mu   sync.RWMutex
	src  Model[T]
	less func(a, b T) bool
	perm []int // display index -> source index

	remove func()
}

// NewSortProxy returns a proxy over src. Until a comparator is set with
// [SortProxy.SetLess] the proxy preserves the source order; for user-visible
// text pass [StringLess].
//
// Since: 2.8
func NewSortProxy[T any](src Model[T]) *SortProxy[T] {
	p := &SortProxy[T]{src: src}
	l := binding.NewDataListener(p.onSourceChanged)
	// AddListener fires immediately, building the initial permutation.
	src.AddListener(l)
	p.remove = func() { src.RemoveListener(l) }
	return p
}

// SetLess replaces the comparator (nil restores source order) and re-sorts.
func (p *SortProxy[T]) SetLess(less func(a, b T) bool) {
	p.mu.Lock()
	p.less = less
	p.mu.Unlock()
	p.rebuild()
	p.notify()
}

// Close detaches the proxy from its source. After Close the proxy is inert.
func (p *SortProxy[T]) Close() {
	if p.remove != nil {
		p.remove()
	}
}

func (p *SortProxy[T]) onSourceChanged() {
	p.rebuild()
	p.notify()
}

func (p *SortProxy[T]) rebuild() {
	n := p.src.Len()
	perm := make([]int, n)
	for i := range perm {
		perm[i] = i
	}

	p.mu.RLock()
	less := p.less
	p.mu.RUnlock()

	if less != nil {
		// Stable so equal items keep source order — a predictable, Qt-like sort.
		sort.SliceStable(perm, func(a, b int) bool {
			return less(p.src.Item(perm[a]), p.src.Item(perm[b]))
		})
	}

	p.mu.Lock()
	p.perm = perm
	p.mu.Unlock()
}

// Len reports the number of items (equal to the source length).
func (p *SortProxy[T]) Len() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.perm)
}

// Item returns the item shown at display index i.
func (p *SortProxy[T]) Item(i int) T {
	p.mu.RLock()
	src := p.perm[i]
	p.mu.RUnlock()
	return p.src.Item(src)
}

// SourceIndex maps a display index to the corresponding index in the source
// model, or -1 if i is out of range.
//
// Since: 2.8
func (p *SortProxy[T]) SourceIndex(i int) int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if i < 0 || i >= len(p.perm) {
		return -1
	}
	return p.perm[i]
}
