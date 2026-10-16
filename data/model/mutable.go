package model

import "github.com/belluikit/bellui"

// MutableModel is a [Model] whose items can be written back in place — the write
// counterpart to the read-only [Model]. It is the seam the editing delegates
// (widget.BindListEditing / widget.BindTableEditing) commit an edited value
// through.
//
// [SliceModel] is always mutable. [SortProxy] and [FilterProxy] are mutable when
// their ultimate source is: a SetItem on a proxy maps the display index to the
// source row through the proxy's permutation — and through any chain of proxies
// (filter → sort → view) — then writes there, so editing a displayed row updates
// the correct underlying row regardless of the active sort or filter.
//
// When a proxy's source is not itself a MutableModel the write cannot be
// forwarded. Rather than panic, SetItem logs an error via [bellui.LogError] and
// leaves the model unchanged, so a bound editor degrades to a no-op instead of
// crashing. Callers that must know whether a write is possible should assert the
// concrete source type, or wrap a [SliceModel] at the base of the proxy chain.
//
// Since: 1.0
type MutableModel[T any] interface {
	Model[T]

	// SetItem replaces the item at display index i and notifies listeners.
	SetItem(i int, item T)
}

// Compile-time proof that the concrete models satisfy MutableModel. SliceModel
// already provides SetItem; the proxies gain it below.
var (
	_ MutableModel[int] = (*SliceModel[int])(nil)
	_ MutableModel[int] = (*SortProxy[int])(nil)
	_ MutableModel[int] = (*FilterProxy[int])(nil)
)

// SetItem writes item back to the source row that display index i currently maps
// to and notifies listeners. It is a no-op (with a logged error) when i is out of
// range or the source is not a [MutableModel]. See [MutableModel].
//
// Since: 1.0
func (p *SortProxy[T]) SetItem(i int, item T) {
	writeThrough[T]("model.SortProxy.SetItem", p.src, p.SourceIndex(i), item)
}

// SetItem writes item back to the source row that display index i currently maps
// to and notifies listeners. It is a no-op (with a logged error) when i is out of
// range or the source is not a [MutableModel]. See [MutableModel].
//
// Since: 1.0
func (p *FilterProxy[T]) SetItem(i int, item T) {
	writeThrough[T]("model.FilterProxy.SetItem", p.src, p.SourceIndex(i), item)
}

// writeThrough forwards a write to the source row srcIndex of a proxy, or logs
// why it could not. Writing through the source fires the source's change
// notification, which the proxy is already subscribed to, so the proxy re-indexes
// and re-notifies once.
func writeThrough[T any](who string, src Model[T], srcIndex int, item T) {
	if srcIndex < 0 {
		bellui.LogError(who+": display index out of range", nil)
		return
	}
	mm, ok := src.(MutableModel[T])
	if !ok {
		bellui.LogError(who+": source is not a MutableModel, write ignored", nil)
		return
	}
	mm.SetItem(srcIndex, item)
}
