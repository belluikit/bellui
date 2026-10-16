// Package model provides an observable, typed data layer that sits between an
// application's data and the List/Table widgets — a composable sort/filter/
// selection layer in the spirit of Qt's model/view classes.
//
// The core type is [Model], a read-only observable sequence of T. [SortProxy]
// and [FilterProxy] wrap a Model and present a re-ordered or narrowed view that
// is itself a Model, so they compose (filter then sort then view). They do their
// O(n log n) / O(n) work once per source change and never per displayed row, so
// a widget bound to a proxy keeps its O(visible) per-frame cost. [SelectionModel]
// tracks selection by stable item identity rather than row index, so a selection
// survives a re-sort, a re-filter or an insert.
//
// The widget adapters that drive a List or Table from a Model live in the widget
// package (widget.BindListModel / widget.BindTableModel), keeping this package
// free of any dependency on widget so the model layer is reusable on its own.
//
// Model mutation and the proxy/selection updates it triggers follow the widgets'
// threading rules: mutate on the main goroutine (via [bellui.Do] when a change
// originates on a background goroutine), matching how widget.List and FileTree
// already expect their callbacks to be driven.
//
// Since: 1.0
package model

import (
	"sync"

	"github.com/belluikit/bellui/data/binding"
	"github.com/belluikit/bellui/lang"
)

// Model is a read-only, observable sequence of T. It reuses the binding
// package's listener vocabulary ([binding.DataItem]): a listener registered with
// AddListener is fired once immediately on registration and again whenever the
// contents change, so a widget adapter can subscribe and render the current
// state in one step.
//
// Since: 1.0
type Model[T any] interface {
	binding.DataItem

	// Len reports the number of items currently exposed by the model.
	Len() int
	// Item returns the item at display index i, where 0 <= i < Len().
	Item(i int) T
}

// StringLess orders strings by the active locale's collation order (see
// [lang.Compare]), the recommended default comparator for sorting user-visible
// text: it puts (for example) German "äpfel" with "a" and Swedish "ö" after "z",
// where a raw code-point comparison would not. Use it with [SortProxy.SetLess]:
//
//	proxy.SetLess(model.StringLess)
//
// Since: 1.0
func StringLess(a, b string) bool {
	return lang.Compare(a, b) < 0
}

// listeners is the shared change-notification machinery embedded by every Model
// implementation in this package. It mirrors the binding package's convention of
// firing a newly-added listener immediately so callers observe current state.
type listeners struct {
	mu   sync.RWMutex
	fns  map[int]binding.DataListener
	next int
}

// AddListener registers l and fires it once immediately.
func (s *listeners) AddListener(l binding.DataListener) {
	s.mu.Lock()
	if s.fns == nil {
		s.fns = map[int]binding.DataListener{}
	}
	// Keyed by id rather than the listener value so an uncomparable or
	// duplicate listener can still be removed, matching binding's base.
	id := s.next
	s.next++
	s.fns[id] = l
	s.mu.Unlock()

	l.DataChanged()
}

// RemoveListener detaches the first registration matching l.
func (s *listeners) RemoveListener(l binding.DataListener) {
	s.mu.Lock()
	for id, fn := range s.fns {
		if fn == l {
			delete(s.fns, id)
			break
		}
	}
	s.mu.Unlock()
}

// notify fires every registered listener on the calling goroutine.
func (s *listeners) notify() {
	s.mu.RLock()
	fns := make([]binding.DataListener, 0, len(s.fns))
	for _, fn := range s.fns {
		fns = append(fns, fn)
	}
	s.mu.RUnlock()

	for _, fn := range fns {
		fn.DataChanged()
	}
}

// SliceModel is a [Model] backed by an in-memory slice of T. Its mutators
// (Set, Append, SetItem) notify listeners; call them on the main goroutine.
//
// Since: 1.0
type SliceModel[T any] struct {
	listeners

	mu    sync.RWMutex
	items []T
}

// NewSliceModel returns a [SliceModel] seeded with the given items.
//
// Since: 1.0
func NewSliceModel[T any](items ...T) *SliceModel[T] {
	m := &SliceModel[T]{}
	m.items = append(m.items, items...)
	return m
}

// Len reports the number of items.
func (m *SliceModel[T]) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.items)
}

// Item returns the item at index i.
func (m *SliceModel[T]) Item(i int) T {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.items[i]
}

// Set replaces the whole contents and notifies listeners.
func (m *SliceModel[T]) Set(items []T) {
	m.mu.Lock()
	m.items = append(m.items[:0:0], items...)
	m.mu.Unlock()
	m.notify()
}

// Append adds items to the end and notifies listeners.
func (m *SliceModel[T]) Append(items ...T) {
	m.mu.Lock()
	m.items = append(m.items, items...)
	m.mu.Unlock()
	m.notify()
}

// SetItem replaces the item at index i and notifies listeners.
func (m *SliceModel[T]) SetItem(i int, item T) {
	m.mu.Lock()
	m.items[i] = item
	m.mu.Unlock()
	m.notify()
}

// dataListModel adapts a [binding.DataList] to a [Model] of [binding.DataItem],
// so existing data-binding users get the proxy/selection layer for free.
type dataListModel struct {
	listeners
	src binding.DataList
}

// FromDataList exposes an existing [binding.DataList] as a [Model], forwarding
// its change notifications. The returned model reads through to the list, so
// mutate the underlying binding as usual; the model reflects it and re-notifies.
//
// The model holds a listener on the source for its lifetime; it is meant to live
// as long as the binding it wraps.
//
// Since: 1.0
func FromDataList(l binding.DataList) Model[binding.DataItem] {
	m := &dataListModel{src: l}
	l.AddListener(binding.NewDataListener(m.notify))
	return m
}

func (m *dataListModel) Len() int {
	return m.src.Length()
}

func (m *dataListModel) Item(i int) binding.DataItem {
	item, err := m.src.GetItem(i)
	if err != nil {
		return nil
	}
	return item
}
