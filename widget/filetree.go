package widget

// FileTree is adapted from the fyne-x project (fyne.io/x/fyne,
// widget/filetree.go), Copyright (C) 2020 Fyne.io developers and community,
// BSD-3-Clause license. The port lists directories asynchronously (showing a
// placeholder row while a listing is in flight), filters hidden files by
// default following the file-dialog convention (storage.IsHidden), and keeps
// MapURI mappings across Refresh.

import (
	"errors"
	"sort"
	"strings"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/bellui/layout"
	"github.com/belluikit/bellui/storage"
	"github.com/belluikit/bellui/storage/repository"
	"github.com/belluikit/bellui/theme"
)

// fileTreeLoadingIDPrefix marks the synthetic placeholder child shown under a
// branch whose directory listing has not arrived yet.
const fileTreeLoadingIDPrefix = "filetree-loading://"

var errNotListable = errors.New("URI is not listable")

// FileTree extends Tree to display a file system hierarchy rooted at a URI.
// Directory contents are listed asynchronously off the main goroutine, so a
// slow lister (network mount, custom repository) does not stall a frame; a
// "Loading..." placeholder row is shown until the listing lands.
//
// Configuration fields must be set before the tree is shown and, like all
// widget state, be mutated only on the main goroutine (call Refresh after
// changing them). Filter and Sorter are invoked on a background goroutine and
// must not touch widget state.
//
// Since: 1.0
type FileTree struct {
	Tree

	// Filter limits the files shown, in the manner of the file dialog.
	// Directories are always shown.
	Filter storage.FileFilter
	// ShowHidden shows items the platform considers hidden
	// (storage.IsHidden), mirroring the file dialog's hidden-files toggle.
	ShowHidden bool
	// ShowRootPath labels the root node with its full URI string instead of
	// its name.
	ShowRootPath bool
	// Sorter orders the children of each branch; when nil the lister's
	// order is kept. For locale-correct name ordering (rather than raw
	// code-point order, which mis-sorts accented and non-English names), set:
	//
	//	tree.Sorter = func(a, b bellui.URI) bool { return lang.Compare(a.Name(), b.Name()) < 0 }
	//
	// lang.Compare is goroutine-safe, so it is fine to call from here (Sorter
	// runs on a background goroutine).
	Sorter func(bellui.URI, bellui.URI) bool
	// Watch, when set before the tree is shown, watches the file system and
	// automatically re-lists a branch whose directory contents change
	// externally — the root and every open branch are watched while the tree
	// is shown (watches stop when the renderer is destroyed and re-arm on
	// re-show). Requires the URIs' repository to implement
	// [repository.WatchableRepository]; the built-in file repository does on
	// desktop and mobile, while platforms or schemes without change
	// notification degrade silently to manual Refresh.
	//
	// Since: 1.0
	Watch bool

	listCache     map[TreeNodeID][]TreeNodeID
	listableCache map[TreeNodeID]bellui.ListableURI
	notListable   map[TreeNodeID]bool
	uriCache      map[TreeNodeID]bellui.URI
	mapped        map[TreeNodeID]bellui.URI
	pending       map[TreeNodeID]bool
	stale         map[TreeNodeID]bool

	// watchCancels holds the release function of every live branch watch,
	// keyed by node; non-nil exactly while watching is armed. Main goroutine
	// only.
	watchCancels map[TreeNodeID]func()

	// listDone, when set, is called at the end of the main-goroutine
	// completion of an asynchronous listing. Test seam.
	listDone func(TreeNodeID)
	// listSynchronously runs listings inline instead of on a goroutine, so
	// tests are deterministic under the test driver's simple threading.
	// Test seam.
	listSynchronously bool
	// watchNotify, when set, receives change notifications instead of the
	// default bellui.Do(noteChanged) hop. Test seam.
	watchNotify func(TreeNodeID)
}

// NewFileTree creates a new FileTree from the given root URI.
//
// Since: 1.0
func NewFileTree(root bellui.URI) *FileTree {
	tree := &FileTree{
		Tree: Tree{
			Root: root.String(),
			CreateNode: func(branch bool) core.CanvasObject {
				var icon core.CanvasObject
				if branch {
					icon = NewIcon(nil)
				} else {
					icon = NewFileIcon(nil)
				}
				return &bellui.Container{
					Layout:  layout.NewBorderLayout(nil, nil, icon, nil),
					Objects: []core.CanvasObject{icon, NewLabel("Template Object")},
				}
			},
		},
		listCache:     make(map[TreeNodeID][]TreeNodeID),
		listableCache: make(map[TreeNodeID]bellui.ListableURI),
		notListable:   make(map[TreeNodeID]bool),
		uriCache:      make(map[TreeNodeID]bellui.URI),
		mapped:        make(map[TreeNodeID]bellui.URI),
		pending:       make(map[TreeNodeID]bool),
		stale:         make(map[TreeNodeID]bool),
	}
	tree.IsBranch = func(id TreeNodeID) bool {
		if isFileTreeLoadingID(id) {
			return false
		}
		_, err := tree.toListable(id)
		return err == nil
	}
	tree.ChildUIDs = tree.childUIDs
	tree.UpdateNode = func(id TreeNodeID, branch bool, node core.CanvasObject) {
		c := node.(*bellui.Container)
		icon, label := c.Objects[0], c.Objects[1].(*Label)
		if isFileTreeLoadingID(id) {
			if f, ok := icon.(*FileIcon); ok {
				f.SetURI(nil)
			}
			label.SetText(lang.L("Loading..."))
			return
		}

		uri, err := tree.toURI(id)
		if err != nil {
			bellui.LogError("Unable to parse URI", err)
			return
		}

		if branch {
			var r bellui.Resource
			if tree.IsBranchOpen(id) {
				r = theme.FolderOpenIcon()
			} else {
				r = theme.FolderIcon()
			}
			icon.(*Icon).SetResource(r)
		} else {
			// Set file uri to update icon
			icon.(*FileIcon).SetURI(uri)
		}

		var l string
		if tree.Root == id && tree.ShowRootPath {
			l = id
		} else {
			l = uri.Name()
		}
		label.SetText(l)
	}

	// Watch newly opened branches (when enabled); re-list closed ones on the
	// next open so a stale cache never survives a close/open cycle.
	tree.OnBranchOpened = tree.watchBranch
	tree.OnBranchClosed = func(id TreeNodeID) {
		if _, ok := tree.listCache[id]; ok {
			tree.stale[id] = true
		}
		tree.unwatchBranch(id)
	}

	tree.ExtendBaseWidget(tree)
	return tree
}

// CreateRenderer is an internal requirement for Bellui; it also (re-)arms the
// file-system watches: a renderer Destroy releases them (a discarded tree
// must not keep a watcher goroutine notifying it), and Destroy also fires on
// cache eviction of a live, temporarily-hidden tree, so re-show re-arms here.
func (t *FileTree) CreateRenderer() core.WidgetRenderer {
	t.startWatching()
	return &fileTreeRenderer{WidgetRenderer: t.Tree.CreateRenderer(), tree: t}
}

type fileTreeRenderer struct {
	core.WidgetRenderer
	tree *FileTree
}

func (r *fileTreeRenderer) Destroy() {
	r.tree.stopWatching()
	r.WidgetRenderer.Destroy()
}

// startWatching arms the file-system watches for the root and every open
// branch; a no-op when Watch is unset or watching is already armed.
func (t *FileTree) startWatching() {
	if !t.Watch || t.watchCancels != nil {
		return
	}
	t.watchCancels = make(map[TreeNodeID]func())
	t.watchBranch(t.Root)
	for id, open := range t.open {
		if open {
			t.watchBranch(id)
		}
	}
}

// stopWatching releases every live watch.
func (t *FileTree) stopWatching() {
	for _, cancel := range t.watchCancels {
		cancel()
	}
	t.watchCancels = nil
}

// watchBranch subscribes to change notification for the given branch when
// watching is armed and the URI's repository supports it. Errors (or an
// unsupported repository/platform) degrade silently to manual Refresh.
func (t *FileTree) watchBranch(id TreeNodeID) {
	if t.watchCancels == nil {
		return
	}
	if _, ok := t.watchCancels[id]; ok {
		return
	}
	uri, err := t.toURI(id)
	if err != nil {
		return
	}
	repo, err := repository.ForURI(uri)
	if err != nil {
		return
	}
	watchable, ok := repo.(repository.WatchableRepository)
	if !ok {
		return
	}
	cancel, err := watchable.WatchURI(uri, func() { t.notifyWatched(id) })
	if err != nil {
		return
	}
	t.watchCancels[id] = cancel
}

func (t *FileTree) unwatchBranch(id TreeNodeID) {
	if cancel, ok := t.watchCancels[id]; ok {
		cancel()
		delete(t.watchCancels, id)
	}
}

// notifyWatched relays a change notification from the repository's watcher
// goroutine onto the main goroutine.
func (t *FileTree) notifyWatched(id TreeNodeID) {
	if f := t.watchNotify; f != nil {
		f(id)
		return
	}
	bellui.Do(func() { t.noteChanged(id) })
}

// noteChanged marks a changed branch's listing stale and redraws, so the next
// query re-lists it (asynchronously, keeping the current children visible
// until the fresh results land). Main goroutine only.
func (t *FileTree) noteChanged(id TreeNodeID) {
	if _, ok := t.listCache[id]; ok {
		t.stale[id] = true
	}
	t.Tree.Refresh()
}

// Refresh re-lists every known branch and redraws the tree. Current children
// stay visible until each fresh listing lands, so an open tree does not
// flash back to placeholders.
func (t *FileTree) Refresh() {
	for id := range t.listCache {
		t.stale[id] = true
	}
	t.listableCache = make(map[TreeNodeID]bellui.ListableURI)
	t.notListable = make(map[TreeNodeID]bool)
	t.uriCache = make(map[TreeNodeID]bellui.URI)

	t.Tree.Refresh()
}

// MapURI allows an app to return a specific URI for the given uid.
// This can be helpful to make more custom trees based on file structure.
// The mapping survives Refresh; map to nil to remove it.
func (t *FileTree) MapURI(uid TreeNodeID, target bellui.URI) {
	if target == nil {
		delete(t.mapped, uid)
	} else {
		t.mapped[uid] = target
	}
	t.Refresh()
}

func (t *FileTree) childUIDs(id TreeNodeID) []TreeNodeID {
	if isFileTreeLoadingID(id) {
		return nil
	}
	ids, ok := t.listCache[id]
	if ok && !t.stale[id] {
		return ids
	}
	fresh, inFlight := t.startList(id)
	if fresh != nil {
		return fresh // synchronous listing (test seam) already landed
	}
	if ok {
		return ids // keep stale children visible until the re-list lands
	}
	if inFlight {
		return []TreeNodeID{fileTreeLoadingIDPrefix + id}
	}
	return nil
}

// startList kicks off an asynchronous listing of the given branch, reporting
// whether one is now in flight; a no-op while one already is. Filter, hidden
// state and Sorter are captured on the main goroutine and applied off it.
// The spawned goroutine is this method's final action, so no widget state is
// touched around a concurrently-completing listing. In the synchronous test
// mode the completed listing is returned instead.
func (t *FileTree) startList(id TreeNodeID) ([]TreeNodeID, bool) {
	if t.pending[id] {
		return nil, true
	}
	listable, err := t.toListable(id)
	if err != nil {
		bellui.LogError("Unable to get lister for "+id, err)
		delete(t.stale, id)
		t.listCache[id] = nil
		return nil, false
	}
	t.pending[id] = true

	filter := t.Filter
	showHidden := t.ShowHidden
	sorter := t.Sorter
	list := func() []TreeNodeID {
		uris, err := listable.List()
		if err != nil {
			bellui.LogError("Unable to list "+listable.String(), err)
		}

		kept := uris[:0]
		for _, u := range uris {
			if !showHidden && storage.IsHidden(u) {
				continue
			}
			if filter != nil && !filter.Matches(u) {
				// Never filter out branches, matching the file dialog.
				if can, err := storage.CanList(u); err != nil || !can {
					continue
				}
			}
			kept = append(kept, u)
		}
		if sorter != nil {
			sort.Slice(kept, func(i, j int) bool {
				return sorter(kept[i], kept[j])
			})
		}
		c := make([]TreeNodeID, 0, len(kept))
		for _, u := range kept {
			c = append(c, u.String())
		}
		return c
	}
	finish := func(c []TreeNodeID, refresh bool) {
		delete(t.pending, id)
		delete(t.stale, id)
		t.listCache[id] = c
		if refresh {
			t.Tree.Refresh()
		}
		if f := t.listDone; f != nil {
			f(id)
		}
	}

	if t.listSynchronously {
		// No Tree.Refresh: the caller is usually inside one already.
		c := list()
		finish(c, false)
		return c, false
	}
	go func() {
		c := list()
		bellui.Do(func() {
			finish(c, true)
		})
	}()
	return nil, true
}

func (t *FileTree) toListable(id TreeNodeID) (bellui.ListableURI, error) {
	if listable, ok := t.listableCache[id]; ok {
		return listable, nil
	}
	uri, err := t.toURI(id)
	if err != nil {
		return nil, err
	}
	if t.notListable[id] {
		return nil, errNotListable
	}

	listable, err := storage.ListerForURI(uri)
	if err != nil {
		t.notListable[id] = true
		return nil, err
	}
	t.listableCache[id] = listable
	return listable, nil
}

func (t *FileTree) toURI(id TreeNodeID) (bellui.URI, error) {
	if uri, ok := t.mapped[id]; ok {
		return uri, nil
	}
	if uri, ok := t.uriCache[id]; ok {
		return uri, nil
	}

	uri, err := storage.ParseURI(id)
	if err != nil {
		return nil, err
	}
	t.uriCache[id] = uri
	return uri, nil
}

func isFileTreeLoadingID(id TreeNodeID) bool {
	return strings.HasPrefix(id, fileTreeLoadingIDPrefix)
}
