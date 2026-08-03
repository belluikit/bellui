package widget

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/driver"
	"github.com/belluikit/bellui/test"
)

// firstTreeViewItem returns the first rendered row of a tree, which is the row
// for its first node.
func firstTreeViewItem(t *testing.T, c core.Canvas) *treeViewItem {
	t.Helper()

	var item *treeViewItem
	driver.WalkVisibleObjectTree(c.Content(), func(o core.CanvasObject, _, _ geo.Position, _ geo.Size) bool {
		if ti, ok := o.(*treeViewItem); ok && item == nil {
			item = ti
		}
		return false
	}, nil)
	return item
}

func menuLabels(items []*bellui.MenuItem) []string {
	labels := make([]string, len(items))
	for i, item := range items {
		if item.IsSeparator {
			labels[i] = "---"
			continue
		}
		labels[i] = item.Label
	}
	return labels
}

func newRenderedTree(t *testing.T) (*TreeView, bellui.Window) {
	t.Helper()

	test.NewTempApp(t)
	tv := NewTreeViewWithNodes([]TreeViewNode{
		{ID: "1", Text: "One"},
		{ID: "2", Text: "Two"},
	})
	w := test.NewTempWindow(t, tv)
	w.Resize(geo.NewSize(300, 200))
	return tv, w
}

func TestTreeViewItem_ContextMenu_BuiltInsOnly(t *testing.T) {
	tv, _ := newRenderedTree(t)
	tv.AllowRename = true
	tv.AllowDelete = true

	item := &treeViewItem{tree: tv, uid: "1"}
	assert.Equal(t, []string{"Rename", "Delete"}, menuLabels(item.contextMenuItems()))
}

func TestTreeViewItem_ContextMenu_NoActionsMeansNoMenu(t *testing.T) {
	tv, _ := newRenderedTree(t)

	item := &treeViewItem{tree: tv, uid: "1"}
	assert.Empty(t, item.contextMenuItems(), "a tree with no allowed actions and no host actions has no menu")
}

func TestTreeViewItem_ContextMenu_HostActionsAppended(t *testing.T) {
	tv, _ := newRenderedTree(t)
	tv.AllowRename = true
	tv.AllowDelete = true

	var askedFor TreeViewNodeID
	tv.ContextActions = func(id TreeViewNodeID) []*bellui.MenuItem {
		askedFor = id
		return []*bellui.MenuItem{
			bellui.NewMenuItem("Duplicate", func() {}),
			bellui.NewMenuItem("Copy path", func() {}),
		}
	}

	item := &treeViewItem{tree: tv, uid: "2"}
	assert.Equal(t, []string{"Rename", "Delete", "---", "Duplicate", "Copy path"}, menuLabels(item.contextMenuItems()))
	assert.Equal(t, TreeViewNodeID("2"), askedFor, "the provider must be asked about the node that was tapped")
}

func TestTreeViewItem_ContextMenu_HostActionsWithoutBuiltIns(t *testing.T) {
	tv, _ := newRenderedTree(t)
	tv.ContextActions = func(TreeViewNodeID) []*bellui.MenuItem {
		return []*bellui.MenuItem{bellui.NewMenuItem("Reveal in Finder", func() {})}
	}

	item := &treeViewItem{tree: tv, uid: "1"}
	assert.Equal(t, []string{"Reveal in Finder"}, menuLabels(item.contextMenuItems()),
		"no built-ins means no leading separator")
}

func TestTreeViewItem_ContextMenu_EmptyHostActionsLeaveMenuAlone(t *testing.T) {
	tv, _ := newRenderedTree(t)
	tv.AllowRename = true
	tv.ContextActions = func(TreeViewNodeID) []*bellui.MenuItem { return nil }

	item := &treeViewItem{tree: tv, uid: "1"}
	assert.Equal(t, []string{"Rename"}, menuLabels(item.contextMenuItems()))
}

func TestTreeView_BeginRename_StartsInlineEditor(t *testing.T) {
	tv, w := newRenderedTree(t)

	tv.BeginRename("1")

	item := firstTreeViewItem(t, w.Canvas())
	require.NotNil(t, item, "no treeViewItem rendered")
	assert.True(t, item.renaming, "the row should be in its inline editor")
}

func TestTreeView_BeginRename_IgnoresAllowRename(t *testing.T) {
	// AllowRename gates the user's own gestures. An application asking for the
	// editor outright gets it.
	tv, w := newRenderedTree(t)
	tv.AllowRename = false

	tv.BeginRename("1")

	item := firstTreeViewItem(t, w.Canvas())
	require.NotNil(t, item)
	assert.True(t, item.renaming)
}

func TestTreeView_BeginRename_UnknownNode(t *testing.T) {
	tv, w := newRenderedTree(t)

	tv.BeginRename("nope")

	item := firstTreeViewItem(t, w.Canvas())
	require.NotNil(t, item)
	assert.False(t, item.renaming)
	assert.Empty(t, tv.takePendingRename(), "an unknown node must not leave a rename armed")
}

func TestTreeView_BeginRename_ReachesAnOffScreenRow(t *testing.T) {
	test.NewTempApp(t)
	tv := NewTreeView()
	for i := 0; i < 60; i++ {
		require.NoError(t, tv.AddNode("", TreeViewNode{ID: TreeViewNodeID(strconv.Itoa(i)), Text: "node"}))
	}
	w := test.NewTempWindow(t, tv)
	w.Resize(geo.NewSize(300, 100))
	require.Nil(t, tv.itemForID("59"), "the last row of a 60-node tree should be off-screen in a 100pt window")

	tv.BeginRename("59")

	// Scrolling a row into view takes a further layout pass — the tree's own
	// offset is only picked up when the tree lays out, not when the scroller
	// moves — so the request waits rather than being dropped.
	w.Resize(geo.NewSize(320, 110))

	item := tv.itemForID("59")
	require.NotNil(t, item, "the row should have been scrolled into view")
	assert.True(t, item.renaming, "the held rename should start once the row is laid out")
	assert.Empty(t, tv.pendingRename, "a started rename must not stay armed")
}

func TestTreeView_BeginRename_DroppedWhenTheNodeGoes(t *testing.T) {
	// A held rename must not outlive its node, or it would fire on whatever row
	// later takes that place in the tree.
	tv, w := newRenderedTree(t)
	tv.armPendingRename("gone")

	w.Resize(geo.NewSize(320, 220)) // force a layout pass

	assert.Empty(t, tv.pendingRename, "a rename for a node that is not in the tree must be dropped")
}

func TestTreeViewItem_RenameSurvivesRelayout(t *testing.T) {
	tv, w := newRenderedTree(t)
	tv.AllowRename = true

	item := firstTreeViewItem(t, w.Canvas())
	require.NotNil(t, item)
	item.startRename()
	require.True(t, item.renaming)

	// Anything that re-lays the content — a resize, a scroll, a refresh — must
	// not silently drop the user out of the editor they are typing in.
	w.Resize(geo.NewSize(320, 220))
	assert.True(t, item.renaming, "a re-layout cancelled an inline rename in progress")
}
