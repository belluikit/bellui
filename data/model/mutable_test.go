package model

import (
	"strings"
	"testing"

	"github.com/belluikit/bellui/data/binding"

	"github.com/stretchr/testify/assert"
)

// roModel is a minimal read-only Model used to exercise the non-mutable source
// path: it deliberately does not implement MutableModel.
type roModel struct {
	listeners
	items []string
}

func (m *roModel) Len() int          { return len(m.items) }
func (m *roModel) Item(i int) string { return m.items[i] }

var _ Model[string] = (*roModel)(nil)

func TestSortProxy_SetItemWritesThroughToSource(t *testing.T) {
	src := NewSliceModel("banana", "apple", "cherry")
	p := NewSortProxy[string](src)
	p.SetLess(func(a, b string) bool { return a < b })

	// Display order is apple, banana, cherry. Edit the displayed row 0 (apple).
	assert.Equal(t, "apple", p.Item(0))
	p.SetItem(0, "apricot")

	// The source's "apple" (index 1) is the one that changed.
	assert.Equal(t, []string{"banana", "apricot", "cherry"}, collect[string](src))
	// The proxy re-sorted after the write.
	assert.Equal(t, []string{"apricot", "banana", "cherry"}, collect[string](p))
}

func TestFilterProxy_SetItemWritesThroughToSource(t *testing.T) {
	src := NewSliceModel("apple", "banana", "apricot", "cherry")
	p := NewFilterProxy[string](src)
	p.SetPredicate(func(s string) bool { return strings.HasPrefix(s, "a") })

	// Display order is apple, apricot. Edit displayed row 1 (apricot -> avocado).
	assert.Equal(t, "apricot", p.Item(1))
	p.SetItem(1, "avocado")

	// Source index 2 (apricot) changed, banana/cherry untouched.
	assert.Equal(t, []string{"apple", "banana", "avocado", "cherry"}, collect[string](src))
	assert.Equal(t, []string{"apple", "avocado"}, collect[string](p))
}

func TestChain_SetItemWritesThroughFilterThenSort(t *testing.T) {
	src := NewSliceModel("delta", "alpha", "avocado", "beta", "apricot")
	filtered := NewFilterProxy[string](src)
	filtered.SetPredicate(func(s string) bool { return strings.HasPrefix(s, "a") })
	sorted := NewSortProxy[string](filtered)
	sorted.SetLess(func(a, b string) bool { return a < b })

	// View is alpha, apricot, avocado. Edit displayed row 0 (alpha -> aardvark).
	assert.Equal(t, "alpha", sorted.Item(0))
	sorted.SetItem(0, "aardvark")

	// Only the source "alpha" (index 1) changed; the write mapped view->filter->source.
	assert.Equal(t, []string{"delta", "aardvark", "avocado", "beta", "apricot"}, collect[string](src))
	// The view re-sorted through both proxies.
	assert.Equal(t, []string{"aardvark", "apricot", "avocado"}, collect[string](sorted))
}

func TestSortProxy_SetItemNonMutableSourceIsNoOp(t *testing.T) {
	src := &roModel{items: []string{"c", "a", "b"}}
	p := NewSortProxy[string](src)
	p.SetLess(func(a, b string) bool { return a < b })
	assert.Equal(t, []string{"a", "b", "c"}, collect[string](p))

	// Source is not mutable: the write must be a logged no-op, never a panic.
	assert.NotPanics(t, func() { p.SetItem(0, "z") })
	assert.Equal(t, []string{"a", "b", "c"}, collect[string](p), "non-mutable source unchanged")
	assert.Equal(t, []string{"c", "a", "b"}, src.items)
}

func TestSortProxy_SetItemOutOfRangeIsNoOp(t *testing.T) {
	src := NewSliceModel("a", "b")
	p := NewSortProxy[string](src)
	assert.NotPanics(t, func() { p.SetItem(99, "x") })
	assert.Equal(t, []string{"a", "b"}, collect[string](src))
}

func TestSetItem_NotifiesOnce(t *testing.T) {
	src := NewSliceModel("banana", "apple", "cherry")
	p := NewSortProxy[string](src)
	p.SetLess(func(a, b string) bool { return a < b })

	count := 0
	// AddListener fires immediately once; count subsequent notifications only.
	fired := false
	p.AddListener(binding.NewDataListener(func() {
		if fired {
			count++
		}
		fired = true
	}))

	p.SetItem(0, "apricot")
	assert.Equal(t, 1, count, "one write must notify the bound proxy exactly once")
}
