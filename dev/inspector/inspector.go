package inspector

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/driver"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
)

// Inspector is a live, read-only view of a target window's widget tree. It
// opens in its own window with a tree pane, a detail pane and a pick mode that
// resolves the object under the cursor on the target. Create one with [Show].
type Inspector struct {
	target bellui.Window
	win    bellui.Window

	tree        *widget.Tree
	detail      *bellui.Container
	snap        *snapshot
	pickBtn     *widget.Button
	selectedUID string

	highlight     *canvas.Rectangle
	highlightLive bool

	pick    *pickOverlay
	picking bool
	closed  bool
}

// Show opens an inspector window targeting w and returns it. Call it on the
// main goroutine (the same goroutine that runs the Bellui event loop); the
// inspector reads the target's live geometry directly. Phase 1 is desktop-only
// — on single-window platforms (mobile, wasm) a second window is not available.
//
// The inspector adds no overhead to the target until something is selected or
// pick mode is armed, and removes everything it added when its window closes.
func Show(w bellui.Window) *Inspector {
	ins := &Inspector{target: w, highlight: newHighlight()}
	ins.build()
	return ins
}

func (ins *Inspector) build() {
	ins.win = bellui.CurrentApp().NewWindow("Bellui Inspector")

	ins.tree = widget.NewTree(
		func(uid string) []string { return ins.snap.childUIDs(uid) },
		func(uid string) bool { return ins.snap.isBranch(uid) },
		func(bool) core.CanvasObject { return widget.NewLabel("template") },
		func(uid string, _ bool, o core.CanvasObject) {
			o.(*widget.Label).SetText(nodeLabel(ins.snap.byUID[uid]))
		},
	)
	ins.tree.OnSelected = ins.selectNode

	ins.detail = container.NewVBox(widget.NewLabelWithStyle(
		"Select a node", bellui.TextAlignLeading, bellui.TextStyle{Italic: true},
	))

	refreshBtn := widget.NewButtonWithIcon("Refresh", theme.ViewRefreshIcon(), ins.refresh)
	ins.pickBtn = widget.NewButtonWithIcon("Pick", theme.SearchIcon(), ins.togglePick)
	toolbar := container.NewHBox(refreshBtn, ins.pickBtn)

	split := container.NewHSplit(
		container.NewScroll(ins.tree),
		container.NewScroll(ins.detail),
	)
	split.SetOffset(0.55)

	ins.win.SetContent(container.NewBorder(toolbar, nil, nil, nil, split))
	ins.win.Resize(geo.NewSize(720, 560))
	ins.win.SetOnClosed(ins.onClose)

	ins.refresh()
	ins.win.Show()
}

// targetRoots returns the object trees to walk on the target canvas: the
// content plus any overlays, minus the inspector's own highlight and pick
// surfaces so they never appear in the tree or get picked.
func (ins *Inspector) targetRoots() []core.CanvasObject {
	c := ins.target.Canvas()
	roots := []core.CanvasObject{}
	if content := c.Content(); content != nil {
		roots = append(roots, content)
	}
	for _, o := range c.Overlays().List() {
		if o == ins.highlight || o == ins.pick {
			continue
		}
		roots = append(roots, o)
	}
	return roots
}

// refresh re-walks the target and rebuilds the tree. It is on-demand only
// (the Refresh button, and after a pick) — there is no ticker, so a closed or
// idle inspector costs nothing.
func (ins *Inspector) refresh() {
	if ins.closed {
		return
	}
	ins.snap = snapshotRoots(ins.targetRoots())
	ins.tree.Refresh()
}

func (ins *Inspector) selectNode(uid string) {
	n := ins.snap.byUID[uid]
	if n == nil {
		return
	}
	ins.selectedUID = uid
	ins.showDetail(describe(n))
	ins.highlightBounds(n.absPos, n.size)
}

func (ins *Inspector) showDetail(rows []detailRow) {
	objs := make([]core.CanvasObject, 0, len(rows))
	for _, r := range rows {
		objs = append(objs, container.NewHBox(
			widget.NewLabelWithStyle(r.Label, bellui.TextAlignLeading, bellui.TextStyle{Bold: true}),
			widget.NewLabel(r.Value),
		))
	}
	ins.detail.Objects = objs
	ins.detail.Refresh()
}

// highlightBounds moves the bounds marker over the given canvas rectangle on
// the target and makes it visible, adding it to the target overlays on first
// use.
func (ins *Inspector) highlightBounds(pos geo.Position, size geo.Size) {
	ins.highlight.Move(pos)
	ins.highlight.Resize(size)
	ins.highlight.Show()
	if !ins.highlightLive {
		ins.target.Canvas().Overlays().Add(ins.highlight)
		ins.highlightLive = true
	}
	ins.target.Canvas().Refresh(ins.highlight)
}

// --- pick mode ---

func (ins *Inspector) togglePick() {
	if ins.picking {
		ins.stopPick()
		return
	}
	ins.startPick()
}

func (ins *Inspector) startPick() {
	c := ins.target.Canvas()
	ins.pick = newPickOverlay(ins.pickHover, ins.pickCommit, ins.stopPick)
	ins.pick.Resize(c.Size())
	c.Overlays().Add(ins.pick)
	ins.picking = true
	ins.pickBtn.SetText("Picking… (Esc)")
	ins.target.RequestFocus()
	c.Focus(ins.pick) // grab keys so Escape cancels
}

func (ins *Inspector) stopPick() {
	if ins.pick != nil {
		ins.target.Canvas().Unfocus()
		ins.target.Canvas().Overlays().Remove(ins.pick)
		ins.pick = nil
	}
	ins.picking = false
	if ins.pickBtn != nil {
		ins.pickBtn.SetText("Pick")
	}
}

// pickHover previews the object under the cursor: it highlights the object's
// bounds and shows its details without disturbing the tree selection.
func (ins *Inspector) pickHover(pos geo.Position) {
	obj, ok := resolveObjectAt(ins.target.Canvas(), pos)
	if !ok {
		return
	}
	n := ins.liveNode(obj)
	ins.showDetail(describe(n))
	ins.highlightBounds(n.absPos, n.size)
}

// pickCommit selects the object under the cursor: it refreshes the tree so the
// object is present, selects its node (which highlights it), and leaves pick
// mode. If the object cannot be located in the refreshed tree, the preview
// highlight and details are kept.
func (ins *Inspector) pickCommit(pos geo.Position) {
	obj, ok := resolveObjectAt(ins.target.Canvas(), pos)
	ins.stopPick()
	if !ok {
		return
	}
	ins.refresh()
	if n := ins.snap.byObj[obj]; n != nil {
		ins.openAncestors(n.uid)
		ins.tree.Select(n.uid)
		return
	}
	// Not in the walked tree (e.g. an inner renderer object filtered out) —
	// fall back to a transient preview so the pick is never a no-op.
	n := ins.liveNode(obj)
	ins.showDetail(describe(n))
	ins.highlightBounds(n.absPos, n.size)
}

// liveNode builds a transient node for an object not necessarily in the
// current snapshot, computing its absolute position via the same walk the
// paint pass uses.
func (ins *Inspector) liveNode(obj core.CanvasObject) *node {
	pos := driver.AbsolutePositionForObject(obj, ins.targetRoots())
	return &node{
		obj:      obj,
		typeName: typeName(obj),
		kind:     kindOf(obj),
		absPos:   pos,
		size:     obj.Size(),
		visible:  obj.Visible(),
	}
}

// openAncestors expands every branch above uid so the node is visible in the
// tree before it is selected.
func (ins *Inspector) openAncestors(uid string) {
	for p := parentUID(uid); p != ""; p = parentUID(p) {
		ins.tree.OpenBranch(p)
	}
}

func (ins *Inspector) onClose() {
	ins.closed = true
	ins.stopPick()
	if ins.highlightLive {
		ins.target.Canvas().Overlays().Remove(ins.highlight)
		ins.highlightLive = false
	}
}
