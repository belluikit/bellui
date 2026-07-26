package model

import (
	"sort"
	"testing"

	"github.com/belluikit/bellui/data/binding"

	"github.com/stretchr/testify/assert"
)

// sampleTree builds a small fixed hierarchy used across the tree-proxy tests:
//
//	""            (root)
//	├── a         (branch)
//	│   ├── a1    (leaf)
//	│   └── a2    (branch)
//	│       └── a2x (leaf)
//	├── b         (branch)
//	│   └── b1    (leaf)
//	└── c         (leaf)
func sampleTree() TreeSource {
	children := map[string][]string{
		"":   {"a", "b", "c"},
		"a":  {"a1", "a2"},
		"a2": {"a2x"},
		"b":  {"b1"},
	}
	branches := map[string]bool{"a": true, "a2": true, "b": true}
	return NewTreeSource(
		"",
		func(uid string) []string { return children[uid] },
		func(uid string) bool { return branches[uid] },
	)
}

func TestTreeProxy_PassThrough(t *testing.T) {
	p := NewTreeProxy(sampleTree())

	assert.Equal(t, "", p.Root())
	assert.Equal(t, []string{"a", "b", "c"}, p.Children(""))
	assert.Equal(t, []string{"a1", "a2"}, p.Children("a"))
	assert.True(t, p.IsBranch("a"))
	assert.True(t, p.IsBranch("a2"))
	assert.False(t, p.IsBranch("a1"))
	assert.False(t, p.IsBranch("c"))
}

func TestTreeProxy_PerSiblingGroupSort(t *testing.T) {
	p := NewTreeProxy(sampleTree())
	// Reverse-alphabetical comparator on the node id.
	p.SetLess(func(a, b string) bool { return a > b })

	// Each sibling group is ordered among itself; the hierarchy is preserved.
	assert.Equal(t, []string{"c", "b", "a"}, p.Children(""))
	assert.Equal(t, []string{"a2", "a1"}, p.Children("a"))
	assert.Equal(t, []string{"a2x"}, p.Children("a2"))

	// nil restores source order.
	p.SetLess(nil)
	assert.Equal(t, []string{"a", "b", "c"}, p.Children(""))
}

func TestTreeProxy_AncestorKeepingFilter_DeepLeaf(t *testing.T) {
	p := NewTreeProxy(sampleTree())
	// Only the deep leaf a2x matches.
	p.SetPredicate(func(uid string) bool { return uid == "a2x" })

	// The whole ancestor chain a → a2 → a2x is kept; non-matching siblings drop.
	assert.Equal(t, []string{"a"}, p.Children(""))
	assert.Equal(t, []string{"a2"}, p.Children("a")) // a1 dropped
	assert.Equal(t, []string{"a2x"}, p.Children("a2"))

	// Branches on the surviving path stay expandable; the matched leaf is a leaf.
	assert.True(t, p.IsBranch("a"))
	assert.True(t, p.IsBranch("a2"))
	assert.False(t, p.IsBranch("a2x"))
}

func TestTreeProxy_Filter_BranchByOwnLabel(t *testing.T) {
	p := NewTreeProxy(sampleTree())
	// The branch "b" matches by its own id; its child b1 does not.
	p.SetPredicate(func(uid string) bool { return uid == "b" })

	assert.Equal(t, []string{"b"}, p.Children(""))
	// Default (reachability) semantics: a matching branch with no matching
	// descendant shows no children, so it is not drawn as an expandable branch.
	assert.Empty(t, p.Children("b"))
	assert.False(t, p.IsBranch("b"))
}

func TestTreeProxy_KeepMatchingSubtree(t *testing.T) {
	p := NewTreeProxy(sampleTree())
	p.SetKeepMatchingSubtree(true)
	p.SetPredicate(func(uid string) bool { return uid == "a" })

	// "a" matches, so its entire subtree is revealed even though none of the
	// descendants match individually.
	assert.Equal(t, []string{"a"}, p.Children(""))
	assert.Equal(t, []string{"a1", "a2"}, p.Children("a"))
	assert.Equal(t, []string{"a2x"}, p.Children("a2"))
	assert.True(t, p.IsBranch("a"))
	assert.True(t, p.IsBranch("a2"))
}

func TestTreeProxy_KeepMatchingSubtree_RespectsSort(t *testing.T) {
	p := NewTreeProxy(sampleTree())
	p.SetKeepMatchingSubtree(true)
	p.SetLess(func(a, b string) bool { return a > b })
	p.SetPredicate(func(uid string) bool { return uid == "a" })

	assert.Equal(t, []string{"a2", "a1"}, p.Children("a")) // subtree kept AND sorted
}

func TestTreeProxy_Filter_EmptyResult(t *testing.T) {
	p := NewTreeProxy(sampleTree())
	p.SetPredicate(func(uid string) bool { return uid == "does-not-exist" })

	assert.Empty(t, p.Children(""))
	assert.Empty(t, p.Children("a"))
	assert.False(t, p.IsBranch("a"))
}

func TestTreeProxy_PredicateSwapRebuilds(t *testing.T) {
	p := NewTreeProxy(sampleTree())

	p.SetPredicate(func(uid string) bool { return uid == "b1" })
	assert.Equal(t, []string{"b"}, p.Children(""))

	// Swap to a different match — the view rebuilds fully.
	p.SetPredicate(func(uid string) bool { return uid == "a1" })
	assert.Equal(t, []string{"a"}, p.Children(""))
	assert.Equal(t, []string{"a1"}, p.Children("a"))

	// Clearing the predicate restores the full tree.
	p.SetPredicate(nil)
	assert.Equal(t, []string{"a", "b", "c"}, p.Children(""))
	assert.True(t, p.IsBranch("b"))
}

func TestTreeProxy_FilterAndSortCompose(t *testing.T) {
	p := NewTreeProxy(sampleTree())
	p.SetLess(func(a, b string) bool { return a > b })
	// Match both a1 and b1 leaves.
	p.SetPredicate(func(uid string) bool { return uid == "a1" || uid == "b1" })

	// Top level keeps a and b (paths to the matches), sorted reverse.
	assert.Equal(t, []string{"b", "a"}, p.Children(""))
	assert.Equal(t, []string{"a1"}, p.Children("a"))
	assert.Equal(t, []string{"b1"}, p.Children("b"))
}

func TestTreeProxy_Observable(t *testing.T) {
	p := NewTreeProxy(sampleTree())

	calls := 0
	l := binding.NewDataListener(func() { calls++ })
	p.AddListener(l) // fires once immediately
	assert.Equal(t, 1, calls)

	p.SetLess(func(a, b string) bool { return a < b })
	assert.Equal(t, 2, calls)

	p.SetPredicate(func(uid string) bool { return true })
	assert.Equal(t, 3, calls)

	p.Refresh()
	assert.Equal(t, 4, calls)

	p.RemoveListener(l)
	p.Refresh()
	assert.Equal(t, 4, calls) // detached
}

func TestTreeProxy_Refresh_PicksUpSourceChange(t *testing.T) {
	children := map[string][]string{"": {"a"}}
	branches := map[string]bool{}
	src := NewTreeSource(
		"",
		func(uid string) []string { return children[uid] },
		func(uid string) bool { return branches[uid] },
	)
	p := NewTreeProxy(src)
	assert.Equal(t, []string{"a"}, p.Children(""))

	// Mutate the backing data, then signal the proxy.
	children[""] = []string{"a", "z"}
	p.Refresh()
	assert.Equal(t, []string{"a", "z"}, p.Children(""))
}

func TestTreeProxy_CycleGuard(t *testing.T) {
	// A malformed source with a cycle a → b → a must not loop forever.
	children := map[string][]string{"": {"a"}, "a": {"b"}, "b": {"a"}}
	branches := map[string]bool{"a": true, "b": true}
	src := NewTreeSource(
		"",
		func(uid string) []string { return children[uid] },
		func(uid string) bool { return branches[uid] },
	)
	p := NewTreeProxy(src)
	p.SetPredicate(func(uid string) bool { return uid == "b" })

	// Terminates; b is reachable under a.
	assert.Equal(t, []string{"a"}, p.Children(""))
	assert.Equal(t, []string{"b"}, p.Children("a"))
}

// TestTreeProxy_IsTreeSource confirms a proxy can itself back another proxy.
func TestTreeProxy_IsTreeSource(t *testing.T) {
	inner := NewTreeProxy(sampleTree())
	inner.SetPredicate(func(uid string) bool { return uid != "c" })

	outer := NewTreeProxy(inner)
	outer.SetLess(func(a, b string) bool { return a > b })

	got := outer.Children("")
	// c was filtered by the inner proxy; the outer proxy sorts what remains.
	sorted := append([]string(nil), got...)
	sort.Sort(sort.Reverse(sort.StringSlice(sorted)))
	assert.Equal(t, sorted, got)
	assert.NotContains(t, got, "c")
	assert.Contains(t, got, "a")
	assert.Contains(t, got, "b")
}
