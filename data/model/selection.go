package model

import (
	"sync"

	"github.com/belluikit/bellui/data/binding"
)

// SelectionMode controls how many items a [SelectionModel] may hold selected.
//
// Since: 1.0
type SelectionMode int

const (
	// SelectSingle keeps at most one item selected; selecting another replaces it.
	SelectSingle SelectionMode = iota
	// SelectMulti allows any set of items to be selected independently.
	SelectMulti
)

// SelectionModel tracks the selected items of a [Model] by a stable identity key
// K rather than by row index, so a selection survives a re-sort, a re-filter or
// an insert in the underlying model — the index-remap bug that positional
// selection (widget.List/Table store an int index) suffers.
//
// K is derived from each item by the id function passed to [NewSelectionModel];
// choose a genuinely stable key (a database id, a file path, a struct field),
// not the row index — using the index reintroduces exactly the bug this type
// exists to avoid.
//
// It is observable: register with AddListener to be notified (immediately, then
// on every change) when the selected set changes — for example to refresh a
// bound widget's highlight.
//
// Since: 1.0
type SelectionModel[T any, K comparable] struct {
	listeners

	mu   sync.RWMutex
	m    Model[T]
	id   func(T) K
	mode SelectionMode
	sel  map[K]struct{}
	// order preserves selection order for Selected(); anchor is the range base.
	order  []K
	anchor *K
}

// NewSelectionModel returns a selection tracker over m, keying items by id.
// The default mode is [SelectSingle]; change it with [SelectionModel.SetMode].
//
// Since: 1.0
func NewSelectionModel[T any, K comparable](m Model[T], id func(T) K) *SelectionModel[T, K] {
	return &SelectionModel[T, K]{
		m:    m,
		id:   id,
		mode: SelectSingle,
		sel:  map[K]struct{}{},
	}
}

// SetMode sets single- or multi-selection. Switching to [SelectSingle] while
// several items are selected keeps only the most recently selected one.
func (s *SelectionModel[T, K]) SetMode(mode SelectionMode) {
	s.mu.Lock()
	s.mode = mode
	changed := false
	if mode == SelectSingle && len(s.order) > 1 {
		keep := s.order[len(s.order)-1]
		s.sel = map[K]struct{}{keep: {}}
		s.order = []K{keep}
		s.anchor = &keep
		changed = true
	}
	s.mu.Unlock()
	if changed {
		s.notify()
	}
}

// Select adds the item with key k to the selection. In [SelectSingle] mode it
// replaces any current selection.
func (s *SelectionModel[T, K]) Select(k K) {
	s.mu.Lock()
	if s.mode == SelectSingle {
		s.sel = map[K]struct{}{}
		s.order = s.order[:0]
	}
	if _, ok := s.sel[k]; !ok {
		s.sel[k] = struct{}{}
		s.order = append(s.order, k)
	}
	kk := k
	s.anchor = &kk
	s.mu.Unlock()
	s.notify()
}

// Unselect removes k from the selection (no-op if it was not selected).
func (s *SelectionModel[T, K]) Unselect(k K) {
	s.mu.Lock()
	if _, ok := s.sel[k]; !ok {
		s.mu.Unlock()
		return
	}
	delete(s.sel, k)
	s.order = removeKey(s.order, k)
	if s.anchor != nil && *s.anchor == k {
		s.anchor = nil
	}
	s.mu.Unlock()
	s.notify()
}

// Toggle selects k if unselected, or unselects it if selected.
func (s *SelectionModel[T, K]) Toggle(k K) {
	if s.IsSelected(k) {
		s.Unselect(k)
		return
	}
	s.Select(k)
}

// Clear removes all selection.
func (s *SelectionModel[T, K]) Clear() {
	s.mu.Lock()
	if len(s.sel) == 0 {
		s.mu.Unlock()
		return
	}
	s.sel = map[K]struct{}{}
	s.order = s.order[:0]
	s.anchor = nil
	s.mu.Unlock()
	s.notify()
}

// SelectRange selects every item whose current display position lies between the
// anchor (the last singly-selected item, or the first item if none) and the item
// at display index to, inclusive. This backs shift-click range selection; it is a
// no-op in [SelectSingle] mode. Keys are resolved against the model's current
// order, so the range reflects the active sort/filter.
func (s *SelectionModel[T, K]) SelectRange(to int) {
	s.mu.Lock()
	if s.mode == SelectSingle {
		s.mu.Unlock()
		return
	}

	n := s.m.Len()
	if to < 0 || to >= n {
		s.mu.Unlock()
		return
	}

	from := 0
	if s.anchor != nil {
		if idx := s.indexOfLocked(*s.anchor); idx >= 0 {
			from = idx
		}
	}
	if from > to {
		from, to = to, from
	}
	for i := from; i <= to; i++ {
		k := s.id(s.m.Item(i))
		if _, ok := s.sel[k]; !ok {
			s.sel[k] = struct{}{}
			s.order = append(s.order, k)
		}
	}
	s.mu.Unlock()
	s.notify()
}

// keyAt returns the identity key of the item at display index i in the bound
// model, or (zero, false) if i is out of range.
func (s *SelectionModel[T, K]) keyAt(i int) (K, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if i < 0 || i >= s.m.Len() {
		var zero K
		return zero, false
	}
	return s.id(s.m.Item(i)), true
}

// SelectIndex selects the item at display index i in the bound model. It is the
// bridge a widget adapter uses to turn a positional tap into an identity-keyed
// selection.
func (s *SelectionModel[T, K]) SelectIndex(i int) {
	if k, ok := s.keyAt(i); ok {
		s.Select(k)
	}
}

// ToggleIndex toggles selection of the item at display index i.
func (s *SelectionModel[T, K]) ToggleIndex(i int) {
	if k, ok := s.keyAt(i); ok {
		s.Toggle(k)
	}
}

// IsIndexSelected reports whether the item at display index i is selected. A
// widget's UpdateItem can call this to style a row for the current selection —
// including multi-selection, which the built-in single-row highlight cannot show.
func (s *SelectionModel[T, K]) IsIndexSelected(i int) bool {
	if k, ok := s.keyAt(i); ok {
		return s.IsSelected(k)
	}
	return false
}

// IsSelected reports whether the item with key k is selected.
func (s *SelectionModel[T, K]) IsSelected(k K) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.sel[k]
	return ok
}

// Selected returns the selected keys in selection order.
func (s *SelectionModel[T, K]) Selected() []K {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]K, len(s.order))
	copy(out, s.order)
	return out
}

// Len reports the number of selected items.
func (s *SelectionModel[T, K]) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.sel)
}

// IndexOf returns the current display index of the item with key k in the bound
// model, or -1 if k is not present. This is how a widget adapter translates a
// stable selection back into the row to highlight after a re-sort or re-filter.
//
// It is an O(n) scan of the model; call it when the selection or model changes,
// not per frame.
func (s *SelectionModel[T, K]) IndexOf(k K) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.indexOfLocked(k)
}

func (s *SelectionModel[T, K]) indexOfLocked(k K) int {
	n := s.m.Len()
	for i := 0; i < n; i++ {
		if s.id(s.m.Item(i)) == k {
			return i
		}
	}
	return -1
}

// SelectedIndices returns the current display indices of the selected items that
// are still present in the model, ascending. Items no longer in the model (for
// example filtered out) are omitted here but remain selected — they reappear
// when the filter is relaxed, which is the whole point of identity-based
// selection.
func (s *SelectionModel[T, K]) SelectedIndices() []int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n := s.m.Len()
	out := make([]int, 0, len(s.sel))
	for i := 0; i < n; i++ {
		if _, ok := s.sel[s.id(s.m.Item(i))]; ok {
			out = append(out, i)
		}
	}
	return out
}

// AddListener/RemoveListener/notify are provided by the embedded listeners.
var _ binding.DataItem = (*SelectionModel[int, int])(nil)

func removeKey[K comparable](s []K, k K) []K {
	for i, v := range s {
		if v == k {
			return append(s[:i], s[i+1:]...)
		}
	}
	return s
}
