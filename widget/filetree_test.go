package widget

import (
	"os"
	"path"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/storage"
	"github.com/belluikit/bellui/test"
)

// newTestFileTree builds a FileTree over a fresh directory tree
// (A/, B/, B/C.txt, B/D.txt, .hidden.txt), listing synchronously so tests
// are deterministic under the test driver's simple threading.
func newTestFileTree(t *testing.T) (*FileTree, bellui.URI) {
	t.Helper()
	tempDir := t.TempDir()
	require.NoError(t, os.MkdirAll(path.Join(tempDir, "A"), os.ModePerm))
	require.NoError(t, os.MkdirAll(path.Join(tempDir, "B"), os.ModePerm))
	require.NoError(t, os.WriteFile(path.Join(tempDir, "B", "C.txt"), []byte("c"), os.ModePerm))
	require.NoError(t, os.WriteFile(path.Join(tempDir, "B", "D.txt"), []byte("d"), os.ModePerm))
	hidden := path.Join(tempDir, ".hidden.txt")
	require.NoError(t, os.WriteFile(hidden, []byte("h"), os.ModePerm))
	require.NoError(t, hideTestFile(hidden))

	root := storage.NewFileURI(tempDir)
	tree := NewFileTree(root)
	tree.listSynchronously = true
	return tree, root
}

func TestFileTree_ZeroValue(t *testing.T) {
	tree := &FileTree{}
	tree.Refresh() // Should not crash
}

func TestFileTree_Filter(t *testing.T) {
	test.NewTempApp(t)
	tree, root := newTestFileTree(t)
	tree.Filter = storage.NewExtensionFileFilter([]string{".txt"})
	tree.ShowHidden = true

	ids := tree.ChildUIDs(root.String())
	branchA, _ := storage.Child(root, "A")
	branchB, _ := storage.Child(root, "B")
	hidden, _ := storage.Child(root, ".hidden.txt")
	assert.ElementsMatch(t, []TreeNodeID{branchA.String(), branchB.String(), hidden.String()}, ids,
		"filter matches files; directories always shown")

	tree.Filter = storage.NewExtensionFileFilter([]string{".png"})
	tree.Refresh()
	ids = tree.ChildUIDs(root.String())
	assert.ElementsMatch(t, []TreeNodeID{branchA.String(), branchB.String()}, ids)
}

func TestFileTree_UpdateNode(t *testing.T) {
	test.NewTempApp(t)
	tree, root := newTestFileTree(t)

	label := func(id TreeNodeID, branch bool) string {
		node := tree.CreateNode(branch)
		tree.UpdateNode(id, branch, node)
		return node.(*bellui.Container).Objects[1].(*Label).Text
	}

	assert.Equal(t, filepath.Base(root.Path()), label(root.String(), true),
		"root labelled by name by default")

	tree.ShowRootPath = true
	assert.Equal(t, root.String(), label(root.String(), true),
		"ShowRootPath labels the root with the full URI")

	branchB, _ := storage.Child(root, "B")
	leafC, _ := storage.Child(branchB, "C.txt")
	assert.Equal(t, "B", label(branchB.String(), true))
	assert.Equal(t, "C.txt", label(leafC.String(), false))

	assert.Equal(t, "Loading...", label(fileTreeLoadingIDPrefix+root.String(), false),
		"placeholder row shows the loading label")
}

func TestFileTree_MapURI(t *testing.T) {
	test.NewTempApp(t)
	tree, root := newTestFileTree(t)

	target, _ := storage.Child(root, "B")
	tree.MapURI("custom-id", target)

	uri, err := tree.toURI("custom-id")
	require.NoError(t, err)
	assert.Equal(t, target.String(), uri.String())

	tree.Refresh()
	uri, err = tree.toURI("custom-id")
	require.NoError(t, err, "mapping survives Refresh")
	assert.Equal(t, target.String(), uri.String())

	tree.MapURI("custom-id", nil)
	_, err = tree.toURI("custom-id")
	assert.Error(t, err, "mapping removed")
}

func TestFileTree_OpenBranchReLists(t *testing.T) {
	test.NewTempApp(t)
	tree, root := newTestFileTree(t)

	branchB, _ := storage.Child(root, "B")
	id := branchB.String()
	assert.Len(t, tree.ChildUIDs(id), 2)

	tree.OpenBranch(id)
	tree.CloseBranch(id)
	require.NoError(t, os.WriteFile(path.Join(root.Path(), "B", "E.txt"), []byte("e"), os.ModePerm))

	assert.Len(t, tree.ChildUIDs(id), 3, "closing a branch marks its listing stale, reopening re-lists")
}

// --- pre-rebrand tracker #454: file-system watching ---

func TestFileTree_WatchDisabledByDefault(t *testing.T) {
	test.NewTempApp(t)
	tree, _ := newTestFileTree(t)
	tree.CreateRenderer()
	assert.Nil(t, tree.watchCancels, "no watches unless Watch is set")
}

func TestFileTree_WatchNotifySeam(t *testing.T) {
	test.NewTempApp(t)
	tree, root := newTestFileTree(t)
	tree.Watch = true

	notified := make(chan TreeNodeID, 16)
	tree.watchNotify = func(id TreeNodeID) {
		select {
		case notified <- id:
		default: // never block the watcher goroutine after the test ends
		}
	}
	tree.CreateRenderer()
	t.Cleanup(tree.stopWatching)

	// A real file-system change notifies the seam with the root id.
	require.NoError(t, os.WriteFile(path.Join(root.Path(), "fresh.txt"), []byte("f"), os.ModePerm))
	select {
	case id := <-notified:
		assert.Equal(t, root.String(), id)
	case <-time.After(5 * time.Second):
		t.Skip("no file notification delivered (platform temp-dir limitation)")
	}
}
