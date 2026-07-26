package model

import (
	"sort"
	"strings"
	"testing"

	"github.com/belluikit/bellui/lang"

	"github.com/stretchr/testify/assert"
)

func TestSortProxy_OrdersAndTracksSource(t *testing.T) {
	src := NewSliceModel("banana", "apple", "cherry")
	p := NewSortProxy[string](src)

	// No comparator: source order preserved.
	assert.Equal(t, []string{"banana", "apple", "cherry"}, collect[string](p))

	p.SetLess(func(a, b string) bool { return a < b })
	assert.Equal(t, []string{"apple", "banana", "cherry"}, collect[string](p))

	// A source change re-sorts automatically.
	src.Append("aardvark")
	assert.Equal(t, []string{"aardvark", "apple", "banana", "cherry"}, collect[string](p))

	// SourceIndex maps display order back to the source slice.
	assert.Equal(t, "aardvark", src.Item(p.SourceIndex(0)))
	assert.Equal(t, -1, p.SourceIndex(99))
}

func TestSortProxy_StableForEqualKeys(t *testing.T) {
	// Equal sort keys keep source order (stable sort).
	type row struct{ key, tag string }
	src := NewSliceModel(
		row{"a", "first"}, row{"a", "second"}, row{"a", "third"},
	)
	p := NewSortProxy[row](src)
	p.SetLess(func(a, b row) bool { return a.key < b.key })
	assert.Equal(t, []string{"first", "second", "third"},
		[]string{p.Item(0).tag, p.Item(1).tag, p.Item(2).tag})
}

func TestSortProxy_CollatedDefault(t *testing.T) {
	lang.SetLocaleForTests(t, "de")
	src := NewSliceModel("zebra", "äpfel", "apfel")
	p := NewSortProxy[string](src)
	p.SetLess(StringLess) // lang.Compare: German 'ä' collates with 'a'

	got := collect[string](p)
	assert.Equal(t, "zebra", got[2], "ä/a-words sort before z under German collation")
	assert.Contains(t, got[:2], "äpfel")
}

func TestFilterProxy_KeepsMatchesAndTracksSource(t *testing.T) {
	src := NewSliceModel("apple", "banana", "apricot", "cherry")
	p := NewFilterProxy[string](src)

	assert.Equal(t, 4, p.Len(), "no predicate keeps everything")

	p.SetPredicate(func(s string) bool { return strings.HasPrefix(s, "a") })
	assert.Equal(t, []string{"apple", "apricot"}, collect[string](p))

	src.Append("avocado")
	assert.Equal(t, []string{"apple", "apricot", "avocado"}, collect[string](p))

	assert.Equal(t, "apricot", src.Item(p.SourceIndex(1)))
}

// TestCompose_FilterThenSort exercises the composed pipeline (source -> filter ->
// sort -> view) and checks that a single source change flows through both layers.
func TestCompose_FilterThenSort(t *testing.T) {
	src := NewSliceModel("delta", "alpha", "avocado", "beta", "apricot")
	filtered := NewFilterProxy[string](src)
	filtered.SetPredicate(func(s string) bool { return strings.HasPrefix(s, "a") })
	sorted := NewSortProxy[string](filtered)
	sorted.SetLess(func(a, b string) bool { return a < b })

	assert.Equal(t, []string{"alpha", "apricot", "avocado"}, collect[string](sorted))

	// One append to the source must propagate through filter and sort.
	src.Append("aardvark")
	assert.Equal(t, []string{"aardvark", "alpha", "apricot", "avocado"}, collect[string](sorted))

	// A non-matching append changes nothing downstream.
	src.Append("zulu")
	assert.Equal(t, []string{"aardvark", "alpha", "apricot", "avocado"}, collect[string](sorted))
}

func TestSortProxy_CloseDetaches(t *testing.T) {
	src := NewSliceModel(3, 1, 2)
	p := NewSortProxy[int](src)
	p.SetLess(func(a, b int) bool { return a < b })
	assert.Equal(t, []int{1, 2, 3}, collect[int](p))

	p.Close()
	src.Append(0) // must not reach the detached proxy
	assert.Equal(t, []int{1, 2, 3}, collect[int](p), "closed proxy ignores source changes")
}

// FuzzSortProxy checks the proxy permutation is always a valid sorted ordering of
// the source: same multiset of items, and non-decreasing under the comparator.
func FuzzSortProxy(f *testing.F) {
	f.Add("banana,apple,cherry,apple")
	f.Add("")
	f.Add("z,y,x,w,v,a,a,a")
	f.Fuzz(func(t *testing.T, csv string) {
		items := strings.Split(csv, ",")
		src := NewSliceModel(items...)
		p := NewSortProxy[string](src)
		p.SetLess(func(a, b string) bool { return a < b })

		got := collect[string](p)

		// Same length and multiset as the source.
		assert.Equal(t, len(items), len(got))
		want := append([]string(nil), items...)
		sort.Strings(want)
		gotSorted := append([]string(nil), got...)
		sort.Strings(gotSorted)
		assert.Equal(t, want, gotSorted, "proxy must be a permutation of the source")

		// Actually ordered.
		assert.True(t, sort.SliceIsSorted(got, func(i, j int) bool { return got[i] < got[j] }))
	})
}
