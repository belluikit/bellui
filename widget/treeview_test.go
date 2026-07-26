package widget_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
)

func TestTreeView_AddNode(t *testing.T) {
	tv := widget.NewTreeView()

	err := tv.AddNode("", widget.TreeViewNode{ID: "a", Text: "Alpha"})
	assert.NoError(t, err)
	err = tv.AddNode("", widget.TreeViewNode{ID: "b", Text: "Beta"})
	assert.NoError(t, err)
	err = tv.AddNode("a", widget.TreeViewNode{ID: "a1", Text: "Alpha-1"})
	assert.NoError(t, err)

	assert.Equal(t, []string{"a", "b"}, tv.ChildIDs(""))
	assert.Equal(t, []string{"a1"}, tv.ChildIDs("a"))
	assert.True(t, tv.IsBranch("a"))
	assert.False(t, tv.IsBranch("b"))
}

func TestTreeView_AddNode_Duplicate(t *testing.T) {
	tv := widget.NewTreeView()
	_ = tv.AddNode("", widget.TreeViewNode{ID: "a", Text: "Alpha"})
	err := tv.AddNode("", widget.TreeViewNode{ID: "a", Text: "Alpha Again"})
	assert.Error(t, err)
}

func TestTreeView_AddNode_BadParent(t *testing.T) {
	tv := widget.NewTreeView()
	err := tv.AddNode("nonexistent", widget.TreeViewNode{ID: "a", Text: "Alpha"})
	assert.Error(t, err)
}

func TestTreeView_RemoveNode(t *testing.T) {
	tv := widget.NewTreeView()
	_ = tv.AddNode("", widget.TreeViewNode{ID: "a", Text: "Alpha"})
	_ = tv.AddNode("a", widget.TreeViewNode{ID: "a1", Text: "Child"})
	_ = tv.AddNode("", widget.TreeViewNode{ID: "b", Text: "Beta"})

	tv.RemoveNode("a")

	assert.Equal(t, []string{"b"}, tv.ChildIDs(""))
	_, ok := tv.GetNode("a")
	assert.False(t, ok)
	_, ok = tv.GetNode("a1")
	assert.False(t, ok)
}

func TestTreeView_SetChildren(t *testing.T) {
	tv := widget.NewTreeView()
	_ = tv.AddNode("", widget.TreeViewNode{ID: "a", Text: "Alpha"})
	_ = tv.AddNode("a", widget.TreeViewNode{ID: "old", Text: "Old"})

	tv.SetChildren("a", []widget.TreeViewNode{
		{ID: "new1", Text: "New 1"},
		{ID: "new2", Text: "New 2"},
	})

	assert.Equal(t, []string{"new1", "new2"}, tv.ChildIDs("a"))
	_, ok := tv.GetNode("old")
	assert.False(t, ok)
}

func TestTreeView_InsertNode(t *testing.T) {
	tv := widget.NewTreeView()
	_ = tv.AddNode("", widget.TreeViewNode{ID: "a", Text: "Alpha"})
	_ = tv.AddNode("", widget.TreeViewNode{ID: "c", Text: "Charlie"})

	err := tv.InsertNode("", 1, widget.TreeViewNode{ID: "b", Text: "Beta"})
	assert.NoError(t, err)
	assert.Equal(t, []string{"a", "b", "c"}, tv.ChildIDs(""))
}

func TestTreeView_GetNode(t *testing.T) {
	tv := widget.NewTreeView()
	_ = tv.AddNode("", widget.TreeViewNode{ID: "a", Text: "Alpha", Data: 42})

	n, ok := tv.GetNode("a")
	assert.True(t, ok)
	assert.Equal(t, "Alpha", n.Text)
	assert.Equal(t, 42, n.Data)

	_, ok = tv.GetNode("nonexistent")
	assert.False(t, ok)
}

func TestTreeView_UpdateNodeData(t *testing.T) {
	tv := widget.NewTreeView()
	_ = tv.AddNode("", widget.TreeViewNode{ID: "a", Text: "Alpha"})
	tv.UpdateNodeData("a", widget.TreeViewNode{ID: "a", Text: "Updated"})

	n, _ := tv.GetNode("a")
	assert.Equal(t, "Updated", n.Text)
}

func TestTreeView_OpenCloseBranch(t *testing.T) {
	tv := widget.NewTreeView()
	_ = tv.AddNode("", widget.TreeViewNode{ID: "a", Text: "Branch"})
	_ = tv.AddNode("a", widget.TreeViewNode{ID: "a1", Text: "Child"})

	assert.False(t, tv.IsBranchOpen("a"))

	var opened, closed string
	tv.OnBranchOpened = func(id string) { opened = id }
	tv.OnBranchClosed = func(id string) { closed = id }

	tv.OpenBranch("a")
	assert.True(t, tv.IsBranchOpen("a"))
	assert.Equal(t, "a", opened)

	tv.CloseBranch("a")
	assert.False(t, tv.IsBranchOpen("a"))
	assert.Equal(t, "a", closed)
}

func TestTreeView_ToggleBranch(t *testing.T) {
	tv := widget.NewTreeView()
	_ = tv.AddNode("", widget.TreeViewNode{ID: "a", Text: "Branch"})
	_ = tv.AddNode("a", widget.TreeViewNode{ID: "a1", Text: "Child"})

	tv.ToggleBranch("a")
	assert.True(t, tv.IsBranchOpen("a"))
	tv.ToggleBranch("a")
	assert.False(t, tv.IsBranchOpen("a"))
}

func TestTreeView_OpenAllBranches(t *testing.T) {
	tv := widget.NewTreeView()
	_ = tv.AddNode("", widget.TreeViewNode{ID: "a", Text: "A"})
	_ = tv.AddNode("a", widget.TreeViewNode{ID: "b", Text: "B"})
	_ = tv.AddNode("b", widget.TreeViewNode{ID: "c", Text: "C"})

	tv.OpenAllBranches()
	assert.True(t, tv.IsBranchOpen("a"))
	assert.True(t, tv.IsBranchOpen("b"))

	tv.CloseAllBranches()
	assert.False(t, tv.IsBranchOpen("a"))
	assert.False(t, tv.IsBranchOpen("b"))
}

func TestTreeView_Select(t *testing.T) {
	tv := widget.NewTreeView()
	_ = tv.AddNode("", widget.TreeViewNode{ID: "a", Text: "Alpha"})
	_ = tv.AddNode("", widget.TreeViewNode{ID: "b", Text: "Beta"})

	var selected, unselected string
	tv.OnSelected = func(id string) { selected = id }
	tv.OnUnselected = func(id string) { unselected = id }

	tv.Select("a")
	assert.Equal(t, "a", selected)
	assert.Equal(t, "a", tv.Selected())

	tv.Select("b")
	assert.Equal(t, "b", selected)
	assert.Equal(t, "a", unselected)

	tv.UnselectAll()
	assert.Equal(t, "", tv.Selected())
	assert.Equal(t, "b", unselected)
}

func TestTreeView_MoveNode(t *testing.T) {
	tv := widget.NewTreeView()
	_ = tv.AddNode("", widget.TreeViewNode{ID: "a", Text: "Alpha"})
	_ = tv.AddNode("", widget.TreeViewNode{ID: "b", Text: "Beta"})
	_ = tv.AddNode("", widget.TreeViewNode{ID: "c", Text: "Charlie"})

	var movedID, movedParent string
	var movedIdx int
	tv.OnMoved = func(id, newParent string, idx int) {
		movedID = id
		movedParent = newParent
		movedIdx = idx
	}

	tv.MoveNode("c", "", 0) // Move c to beginning
	assert.Equal(t, []string{"c", "a", "b"}, tv.ChildIDs(""))
	assert.Equal(t, "c", movedID)
	assert.Equal(t, "", movedParent)
	assert.Equal(t, 0, movedIdx)
}

func TestTreeView_MoveNode_Reparent(t *testing.T) {
	tv := widget.NewTreeView()
	_ = tv.AddNode("", widget.TreeViewNode{ID: "a", Text: "Alpha"})
	_ = tv.AddNode("", widget.TreeViewNode{ID: "b", Text: "Beta"})

	tv.MoveNode("b", "a", 0)
	assert.Equal(t, []string{"a"}, tv.ChildIDs(""))
	assert.Equal(t, []string{"b"}, tv.ChildIDs("a"))
	assert.True(t, tv.IsBranch("a"))
}

func TestTreeView_RenameNode(t *testing.T) {
	tv := widget.NewTreeView()
	_ = tv.AddNode("", widget.TreeViewNode{ID: "a", Text: "Alpha"})

	var renamedID, newName string
	tv.OnRenamed = func(id, name string) { renamedID = id; newName = name }

	tv.RenameNode("a", "Alpha Renamed")
	n, _ := tv.GetNode("a")
	assert.Equal(t, "Alpha Renamed", n.Text)
	assert.Equal(t, "a", renamedID)
	assert.Equal(t, "Alpha Renamed", newName)
}

func TestTreeView_DeleteNode(t *testing.T) {
	tv := widget.NewTreeView()
	_ = tv.AddNode("", widget.TreeViewNode{ID: "a", Text: "Alpha"})
	_ = tv.AddNode("", widget.TreeViewNode{ID: "b", Text: "Beta"})

	var deletedID string
	tv.OnDeleted = func(id string) { deletedID = id }

	tv.DeleteNode("a")
	assert.Equal(t, []string{"b"}, tv.ChildIDs(""))
	assert.Equal(t, "a", deletedID)
}

func TestTreeView_CreateRenderer(t *testing.T) {
	tv := widget.NewTreeView()
	_ = tv.AddNode("", widget.TreeViewNode{ID: "a", Text: "Alpha"})
	_ = tv.AddNode("", widget.TreeViewNode{ID: "b", Text: "Beta"})
	_ = tv.AddNode("a", widget.TreeViewNode{ID: "a1", Text: "Child"})

	w := test.NewTempWindow(t, tv)
	w.Resize(geo.NewSize(300, 400))

	// Should not panic
	assert.NotNil(t, tv.MinSize())
	assert.Greater(t, tv.MinSize().Width, float32(0))
	assert.Greater(t, tv.MinSize().Height, float32(0))
}

func TestTreeView_WithIcons(t *testing.T) {
	tv := widget.NewTreeView()
	_ = tv.AddNode("", widget.TreeViewNode{
		ID:   "a",
		Text: "With Icon",
		Icon: theme.FolderIcon(),
	})
	_ = tv.AddNode("", widget.TreeViewNode{
		ID:   "b",
		Text: "No Icon",
	})

	w := test.NewTempWindow(t, tv)
	w.Resize(geo.NewSize(300, 400))

	// Should render without panic
	n, _ := tv.GetNode("a")
	assert.NotNil(t, n.Icon)
	n2, _ := tv.GetNode("b")
	assert.Nil(t, n2.Icon)
}

func TestTreeView_HasMore(t *testing.T) {
	tv := widget.NewTreeView()
	_ = tv.AddNode("", widget.TreeViewNode{ID: "a", Text: "Branch"})

	tv.SetHasMore("a", true)
	assert.True(t, tv.IsBranch("a")) // hasMore makes it a branch

	var loadedParent string
	tv.OnLoadMore = func(parentID string) { loadedParent = parentID }

	tv.OpenBranch("a")
	assert.Equal(t, "a", loadedParent)
}

func TestTreeView_KeyboardNav(t *testing.T) {
	tv := widget.NewTreeView()
	_ = tv.AddNode("", widget.TreeViewNode{ID: "a", Text: "Alpha"})
	_ = tv.AddNode("", widget.TreeViewNode{ID: "b", Text: "Beta"})
	_ = tv.AddNode("", widget.TreeViewNode{ID: "c", Text: "Charlie"})

	w := test.NewTempWindow(t, tv)
	w.Resize(geo.NewSize(300, 400))
	w.Canvas().Focus(tv)

	// Focus should be on first item
	tv.TypedKey(&event.KeyEvent{Name: event.KeySpace})
	assert.Equal(t, "a", tv.Selected())

	// Move down and select
	tv.TypedKey(&event.KeyEvent{Name: event.KeyDown})
	tv.TypedKey(&event.KeyEvent{Name: event.KeySpace})
	assert.Equal(t, "b", tv.Selected())
}

func TestTreeView_NewTreeViewWithNodes(t *testing.T) {
	tv := widget.NewTreeViewWithNodes([]widget.TreeViewNode{
		{ID: "1", Text: "One"},
		{ID: "2", Text: "Two"},
		{ID: "3", Text: "Three"},
	})

	assert.Equal(t, 3, len(tv.ChildIDs("")))
	n, ok := tv.GetNode("2")
	assert.True(t, ok)
	assert.Equal(t, "Two", n.Text)
}
