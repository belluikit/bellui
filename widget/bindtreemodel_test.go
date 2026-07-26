package widget_test

import (
	"testing"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/data/model"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"

	"github.com/stretchr/testify/assert"
)

func sampleTreeSource() model.TreeSource {
	children := map[string][]string{
		"":   {"a", "b", "c"},
		"a":  {"a1", "a2"},
		"a2": {"a2x"},
		"b":  {"b1"},
	}
	branches := map[string]bool{"a": true, "a2": true, "b": true}
	return model.NewTreeSource(
		"",
		func(uid string) []string { return children[uid] },
		func(uid string) bool { return branches[uid] },
	)
}

func TestBindTreeModel_DrivesCallbacks(t *testing.T) {
	test.NewTempApp(t)

	p := model.NewTreeProxy(sampleTreeSource())
	tree := widget.NewTree(
		nil, nil,
		func(bool) core.CanvasObject { return widget.NewLabel("") },
		func(uid widget.TreeNodeID, branch bool, o core.CanvasObject) {
			o.(*widget.Label).SetText(uid)
		},
	)

	unbind := widget.BindTreeModel(tree, p)
	defer unbind()

	assert.Equal(t, "", tree.Root)
	assert.Equal(t, []string{"a", "b", "c"}, tree.ChildUIDs(""))
	assert.True(t, tree.IsBranch("a"))
	assert.False(t, tree.IsBranch("c"))
}

func TestBindTreeModel_ReflectsSortAndFilter(t *testing.T) {
	test.NewTempApp(t)

	p := model.NewTreeProxy(sampleTreeSource())
	tree := widget.NewTree(
		nil, nil,
		func(bool) core.CanvasObject { return widget.NewLabel("") },
		func(widget.TreeNodeID, bool, core.CanvasObject) {},
	)
	unbind := widget.BindTreeModel(tree, p)
	defer unbind()

	// Sort reflected through the tree's ChildUIDs.
	p.SetLess(func(a, b string) bool { return a > b })
	assert.Equal(t, []string{"c", "b", "a"}, tree.ChildUIDs(""))

	// Ancestor-keeping filter reflected too.
	p.SetPredicate(func(uid string) bool { return uid == "a2x" })
	assert.Equal(t, []string{"a"}, tree.ChildUIDs(""))
	assert.Equal(t, []string{"a2"}, tree.ChildUIDs("a"))
	assert.True(t, tree.IsBranch("a2"))
	assert.False(t, tree.IsBranch("a2x"))
}
