package model

import (
	"sync"

	"github.com/belluikit/bellui/data/binding"
)

// FilterProxy wraps a source [Model] and presents only the items that satisfy a
// predicate, while remaining a [Model] itself so it can be composed (for example
// wrapped in a [SortProxy], or bound directly to a widget). It maintains the list
// of surviving source indices and rebuilds it once whenever the source changes or
// the predicate is replaced — an O(n) pass, never per displayed item.
//
// Since: 1.0
type FilterProxy[T any] struct {
	listeners

	mu   sync.RWMutex
	src  Model[T]
	keep func(T) bool
	perm []int // display index -> source index (kept items, in source order)

	remove func()
}

// NewFilterProxy returns a proxy over src. Until a predicate is set with
// [FilterProxy.SetPredicate] every item passes.
//
// Since: 1.0
func NewFilterProxy[T any](src Model[T]) *FilterProxy[T] {
	p := &FilterProxy[T]{src: src}
	l := binding.NewDataListener(p.onSourceChanged)
	// AddListener fires immediately, building the initial index.
	src.AddListener(l)
	p.remove = func() { src.RemoveListener(l) }
	return p
}

// SetPredicate replaces the keep predicate (nil keeps every item) and re-filters.
// For search-as-you-type, debounce calls to this on the input side; each call is
// a single O(n) re-index.
func (p *FilterProxy[T]) SetPredicate(keep func(T) bool) {
	p.mu.Lock()
	p.keep = keep
	p.mu.Unlock()
	p.rebuild()
	p.notify()
}

// Close detaches the proxy from its source. After Close the proxy is inert.
func (p *FilterProxy[T]) Close() {
	if p.remove != nil {
		p.remove()
	}
}

func (p *FilterProxy[T]) onSourceChanged() {
	p.rebuild()
	p.notify()
}

func (p *FilterProxy[T]) rebuild() {
	p.mu.RLock()
	keep := p.keep
	p.mu.RUnlock()

	n := p.src.Len()
	perm := make([]int, 0, n)
	for i := 0; i < n; i++ {
		if keep == nil || keep(p.src.Item(i)) {
			perm = append(perm, i)
		}
	}

	p.mu.Lock()
	p.perm = perm
	p.mu.Unlock()
}

// Len reports the number of items that currently pass the predicate.
func (p *FilterProxy[T]) Len() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.perm)
}

// Item returns the item shown at display index i.
func (p *FilterProxy[T]) Item(i int) T {
	p.mu.RLock()
	src := p.perm[i]
	p.mu.RUnlock()
	return p.src.Item(src)
}

// SourceIndex maps a display index to the corresponding index in the source
// model, or -1 if i is out of range.
//
// Since: 1.0
func (p *FilterProxy[T]) SourceIndex(i int) int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if i < 0 || i >= len(p.perm) {
		return -1
	}
	return p.perm[i]
}
