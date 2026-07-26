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
	"github.com/belluikit/bellui/internal/cache"
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

func TestNewFileTree(t *testing.T) {
	test.NewTempApp(t)
	tree, root := newTestFileTree(t)

	ids := tree.ChildUIDs(root.String())
	branchA, _ := storage.Child(root, "A")
	branchB, _ := storage.Child(root, "B")
	assert.ElementsMatch(t, []TreeNodeID{branchA.String(), branchB.String()}, ids,
		"hidden file excluded by default")

	assert.True(t, tree.IsBranch(root.String()))
	assert.True(t, tree.IsBranch(branchB.String()))

	leaves := tree.ChildUIDs(branchB.String())
	leafC, _ := storage.Child(branchB, "C.txt")
	leafD, _ := storage.Child(branchB, "D.txt")
	assert.ElementsMatch(t, []TreeNodeID{leafC.String(), leafD.String()}, leaves)
	assert.False(t, tree.IsBranch(leafC.String()))
	assert.Empty(t, tree.ChildUIDs(leafC.String()), "leaves have no children")
}

// TestFileTree_AsyncPlaceholder exercises the production (asynchronous)
// listing path: the first query answers with a loading placeholder row and
// the children land off the main goroutine. The test only touches the tree
// strictly between the trigger and the completion signal.
func TestFileTree_AsyncPlaceholder(t *testing.T) {
	test.NewTempApp(t)
	tree, root := newTestFileTree(t)
	tree.listSynchronously = false
	done := make(chan TreeNodeID, 16)
	tree.listDone = func(id TreeNodeID) { done <- id }

	ids := tree.ChildUIDs(root.String())
	require.Len(t, ids, 1)
	assert.True(t, isFileTreeLoadingID(ids[0]), "placeholder shown while listing is in flight")
	assert.False(t, tree.IsBranch(ids[0]), "placeholder is a leaf")

	select {
	case id := <-done:
		assert.Equal(t, root.String(), id)
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for async listing")
	}

	ids = tree.ChildUIDs(root.String())
	assert.Len(t, ids, 2, "children served from cache once the listing landed")
}

func TestFileTree_ShowHidden(t *testing.T) {
	test.NewTempApp(t)
	tree, root := newTestFileTree(t)

	ids := tree.ChildUIDs(root.String())
	assert.Len(t, ids, 2, "hidden file excluded by default")

	tree.ShowHidden = true
	tree.Refresh() // marks known listings stale; next query re-lists
	ids = tree.ChildUIDs(root.String())
	hidden, _ := storage.Child(root, ".hidden.txt")
	assert.Contains(t, ids, hidden.String())
	assert.Len(t, ids, 3)
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

func TestFileTree_Sorter(t *testing.T) {
	test.NewTempApp(t)
	tree, root := newTestFileTree(t)
	tree.Sorter = func(u1, u2 bellui.URI) bool {
		return u2.String() < u1.String() // Reverse alphabetical
	}

	ids := tree.ChildUIDs(root.String())
	branchA, _ := storage.Child(root, "A")
	branchB, _ := storage.Child(root, "B")
	assert.Equal(t, []TreeNodeID{branchB.String(), branchA.String()}, ids)
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

// --- #454: file-system watching ---

func TestFileTree_WatchLifecycle(t *testing.T) {
	test.NewTempApp(t)
	tree, root := newTestFileTree(t)
	tree.Watch = true
	// The test asserts the watch bookkeeping only: swallow notifications (the
	// TempDir removal at cleanup fires events) and release the watches before
	// the test ends so nothing outlives it.
	tree.watchNotify = func(TreeNodeID) {}
	t.Cleanup(tree.stopWatching)

	require.Nil(t, tree.watchCancels, "not armed before the renderer exists")

	tree.CreateRenderer()
	require.NotNil(t, tree.watchCancels, "armed on show")
	assert.Contains(t, tree.watchCancels, root.String(), "root is watched")

	// Opening a branch adds a watch; closing removes it.
	branchB, _ := storage.Child(root, "B")
	tree.ChildUIDs(root.String()) // ensure children known
	tree.OpenBranch(branchB.String())
	assert.Contains(t, tree.watchCancels, branchB.String())
	tree.CloseBranch(branchB.String())
	assert.NotContains(t, tree.watchCancels, branchB.String())

	// Destroy releases everything; re-show re-arms root and open branches.
	tree.OpenBranch(branchB.String())
	cache.Renderer(tree).Destroy()
	assert.Nil(t, tree.watchCancels, "Destroy stops watching")
	tree.CreateRenderer()
	require.NotNil(t, tree.watchCancels, "re-show re-arms")
	assert.Contains(t, tree.watchCancels, root.String())
	assert.Contains(t, tree.watchCancels, branchB.String(), "open branch re-watched")
}

func TestFileTree_WatchDisabledByDefault(t *testing.T) {
	test.NewTempApp(t)
	tree, _ := newTestFileTree(t)
	tree.CreateRenderer()
	assert.Nil(t, tree.watchCancels, "no watches unless Watch is set")
}

func TestFileTree_NoteChangedReListsBranch(t *testing.T) {
	test.NewTempApp(t)
	tree, root := newTestFileTree(t)

	id := root.String()
	assert.Len(t, tree.ChildUIDs(id), 2)

	// An external change arrives (relayed to the main goroutine): the cached
	// listing is marked stale and the next query re-lists.
	require.NoError(t, os.WriteFile(path.Join(root.Path(), "E.txt"), []byte("e"), os.ModePerm))
	tree.noteChanged(id)
	assert.Len(t, tree.ChildUIDs(id), 3, "changed branch re-lists")

	// A change for a never-listed branch is harmless.
	assert.NotPanics(t, func() { tree.noteChanged("file:///nonexistent") })
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
