package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// Cached load-more Activity spinners must not accumulate for the lifetime of the
// tree renderer: once a branch stops showing a "load more" indicator (e.g. it is
// collapsed), its entry should be pruned from loadMoreIndicators.
func TestTreeView_LoadMoreIndicatorsPruned(t *testing.T) {
	test.NewTempApp(t)

	tv := NewTreeView()
	require.NoError(t, tv.AddNode("", TreeViewNode{ID: "a", Text: "Branch A"}))
	require.NoError(t, tv.AddNode("a", TreeViewNode{ID: "a1", Text: "Child A1"}))
	require.NoError(t, tv.AddNode("", TreeViewNode{ID: "b", Text: "Branch B"}))
	require.NoError(t, tv.AddNode("b", TreeViewNode{ID: "b1", Text: "Child B1"}))
	tv.SetHasMore("a", true)
	tv.SetHasMore("b", true)

	w := test.NewTempWindow(t, tv)
	w.Resize(geo.NewSize(300, 600))

	tv.OpenBranch("a")
	tv.OpenBranch("b")

	tvr := test.TempWidgetRenderer(t, tv).(*treeViewRenderer)
	content := tvr.content
	cr := test.TempWidgetRenderer(t, content).(*treeViewContentRenderer)

	// Both open branches advertise more children, so both cache an indicator.
	cr.Layout(content.Size())
	require.Contains(t, cr.loadMoreIndicators, TreeViewNodeID("a"))
	require.Contains(t, cr.loadMoreIndicators, TreeViewNodeID("b"))

	// Collapsing B drops its indicator on the next layout; A's stays.
	tv.CloseBranch("b")
	cr.Layout(content.Size())
	assert.Contains(t, cr.loadMoreIndicators, TreeViewNodeID("a"))
	assert.NotContains(t, cr.loadMoreIndicators, TreeViewNodeID("b"),
		"collapsed branch indicator should be pruned")
}
