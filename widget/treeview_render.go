package widget

import (
	"image/color"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/async"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/bellui/theme"
)

// === Layer 1: treeViewRenderer ===

var _ core.WidgetRenderer = (*treeViewRenderer)(nil)

type treeViewRenderer struct {
	widget.BaseRenderer
	tree     *TreeView
	content  *treeViewContent
	scroller *widget.Scroll
}

func (r *treeViewRenderer) MinSize() (min geo.Size) {
	min = r.scroller.MinSize()
	min = min.Max(r.tree.branchMinSize)
	min = min.Max(r.tree.leafMinSize)
	return min
}

func (r *treeViewRenderer) Layout(size geo.Size) {
	r.content.viewport = size
	r.scroller.Resize(size)
	r.tree.offsetUpdated(r.scroller.Offset)
}

func (r *treeViewRenderer) Refresh() {
	r.updateMinSizes()
	s := r.tree.Size()
	if s.IsZero() {
		r.tree.Resize(r.tree.MinSize())
	} else {
		r.Layout(s)
	}
	r.scroller.Refresh()
	r.content.Refresh()
	canvas.Refresh(r.tree.super())
}

func (r *treeViewRenderer) updateMinSizes() {
	if f := r.tree.CreateNode; f != nil {
		branch := createItemAndApplyThemeScope(func() core.CanvasObject { return f(true) }, r.tree)
		r.tree.branchMinSize = measureTemplate(newTreeViewItem(r.tree, branch, true))

		leaf := createItemAndApplyThemeScope(func() core.CanvasObject { return f(false) }, r.tree)
		r.tree.leafMinSize = measureTemplate(newTreeViewItem(r.tree, leaf, false))
	} else {
		// Default: icon + label
		r.tree.branchMinSize = measureTemplate(newTreeViewItem(r.tree, NewLabel("Template"), true))
		r.tree.leafMinSize = measureTemplate(newTreeViewItem(r.tree, NewLabel("Template"), false))
	}
}

// === Layer 2: treeViewContent ===

var _ core.Widget = (*treeViewContent)(nil)

type treeViewContent struct {
	BaseWidget
	tree     *TreeView
	viewport geo.Size

	nextRefreshID TreeViewNodeID
}

func newTreeViewContent(tree *TreeView) *treeViewContent {
	c := &treeViewContent{tree: tree}
	c.ExtendBaseWidget(c)
	return c
}

func (c *treeViewContent) CreateRenderer() core.WidgetRenderer {
	return &treeViewContentRenderer{
		BaseRenderer: widget.BaseRenderer{},
		treeContent:  c,
		items:        make(map[TreeViewNodeID]*treeViewItem),
	}
}

func (c *treeViewContent) Resize(size geo.Size) {
	if size == c.Size() {
		return
	}
	c.size = size
	c.Refresh()
}

func (c *treeViewContent) refreshForID(id TreeViewNodeID) {
	c.nextRefreshID = id
	c.BaseWidget.Refresh()
}

func (c *treeViewContent) Refresh() {
	c.nextRefreshID = allTreeViewNodesID
	c.BaseWidget.Refresh()
}

// === Layer 2: treeViewContentRenderer ===

var _ core.WidgetRenderer = (*treeViewContentRenderer)(nil)

type treeViewContentRenderer struct {
	widget.BaseRenderer
	treeContent *treeViewContent
	separators  []core.CanvasObject
	guideLines  []core.CanvasObject
	objects     []core.CanvasObject
	items       map[TreeViewNodeID]*treeViewItem
	itemPool    async.Pool[core.CanvasObject]

	wasVisible         []TreeViewNodeID
	visible            []TreeViewNodeID
	minSizeCache       geo.Size
	loadMoreIndicators map[TreeViewNodeID]*Activity

	dropIndicator *canvas.Rectangle
}

// treeViewLoadMoreEntry records a branch that needs a "load more" indicator,
// appended after its children have been laid out.
type treeViewLoadMoreEntry struct {
	parentID TreeViewNodeID
	depth    int
}

// treeViewLayoutState carries the mutable layout cursor and accumulators shared
// between Layout and its helper methods for a single Layout pass.
type treeViewLayoutState struct {
	th             bellui.Theme
	v              bellui.ThemeVariant
	newItems       map[TreeViewNodeID]*treeViewItem
	pad            float32
	offsetY        float32
	viewport       geo.Size
	width          float32
	separatorCount int
	separatorSize  geo.Size
	separatorOff   float32
	hideSeparators bool
	showGuideLines bool
	iconSize       float32
	indentWidth    float32
	y              float32

	guideLineDepths []int
	loadMoreEntries []treeViewLoadMoreEntry
}

func (r *treeViewContentRenderer) Layout(size geo.Size) {
	tree := r.treeContent.tree
	th := r.treeContent.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()
	r.objects = nil

	pad := th.Size(theme.SizeNamePadding)
	viewport := r.treeContent.viewport
	width := geo.Max(size.Width, viewport.Width)
	separatorThickness := th.Size(theme.SizeNameSeparatorThickness)
	iconSize := th.Size(theme.SizeNameInlineIcon)

	st := &treeViewLayoutState{
		th:             th,
		v:              v,
		newItems:       make(map[TreeViewNodeID]*treeViewItem),
		pad:            pad,
		offsetY:        tree.offset.Y,
		viewport:       viewport,
		width:          width,
		separatorSize:  geo.NewSize(width, separatorThickness),
		separatorOff:   (pad + separatorThickness) / 2,
		hideSeparators: tree.HideSeparators,
		showGuideLines: tree.ShowGuideLines,
		iconSize:       iconSize,
		indentWidth:    iconSize + pad,
	}

	r.wasVisible, r.visible = r.visible, r.wasVisible
	r.visible = r.visible[:0]

	// Walk all open nodes
	tree.walkAll(func(uid, parentID TreeViewNodeID, isBranch bool, depth int) {
		r.layoutRow(st, uid, isBranch, depth)
	})

	r.layoutLoadMoreIndicators(st)

	// Hide unused separators
	if st.hideSeparators {
		st.separatorCount = 0
	}
	for ; st.separatorCount < len(r.separators); st.separatorCount++ {
		r.separators[st.separatorCount].Hide()
	}

	// Return unused items to pool
	for uid, item := range r.items {
		if _, ok := st.newItems[uid]; !ok {
			// A pooled row must not carry an in-progress rename: reused later
			// for the same id — the node deleted and re-added, or its row
			// scrolled back into view — it would reopen the editor unasked.
			item.renaming = false
			r.itemPool.Put(item)
		}
	}
	r.items = st.newItems

	r.layoutGuideLines(st, size)

	// Drop indicator
	if r.dropIndicator != nil {
		r.objects = append(r.objects, r.dropIndicator)
	}

	// A rename asked for before the row existed — a node created and renamed in
	// one go, or one scrolled to from far down the tree — starts once the row
	// does. It keeps waiting across further layouts while the node is still
	// there, since bringing a row into view can take more than one pass.
	if uid := tree.takePendingRename(); uid != "" {
		if item, ok := r.items[uid]; ok {
			item.startRename()
		} else if _, stillThere := tree.GetNode(uid); stillThere {
			tree.armPendingRename(uid)
		}
	}
}

// layoutRow positions a single tree row (separator + item) during the walkAll
// pass and records guide-line depths and load-more entries as needed.
func (r *treeViewContentRenderer) layoutRow(st *treeViewLayoutState, uid TreeViewNodeID, isBranch bool, depth int) {
	tree := r.treeContent.tree

	// Add separator space
	addSeparator := st.y > 0
	if addSeparator {
		st.y += st.pad
	}

	m := tree.leafMinSize
	if isBranch {
		m = tree.branchMinSize
	}

	// Check viewport bounds
	if st.y+m.Height < st.offsetY {
		// Above viewport
		st.y += m.Height
		return
	}
	if st.y > st.offsetY+st.viewport.Height {
		// Below viewport — but we can't early-return from walkAll callback
		st.y += m.Height
		return
	}

	// Node is in viewport
	r.visible = append(r.visible, uid)

	st.recordGuideLineDepth(depth)
	r.layoutSeparator(st, addSeparator)

	// Get or create item
	item, exists := r.items[uid]
	if !exists {
		item = r.getItem(isBranch)
	}
	item.update(uid, depth, isBranch)
	if f := tree.UpdateNode; f != nil {
		f(uid, isBranch, item.content)
	} else {
		// Default: update label text and icon
		r.updateDefaultContent(item, uid)
	}
	st.newItems[uid] = item
	r.objects = append(r.objects, item)
	item.Move(geo.NewPos(0, st.y))
	item.Resize(geo.NewSize(st.width, m.Height))

	st.y += m.Height

	r.recordLoadMore(st, uid, isBranch, depth)
}

// recordGuideLineDepth tracks the first occurrence of each depth so a guide line
// can later be drawn per depth level.
func (st *treeViewLayoutState) recordGuideLineDepth(depth int) {
	if !st.showGuideLines || depth <= 0 {
		return
	}
	for _, d := range st.guideLineDepths {
		if d == depth {
			return
		}
	}
	st.guideLineDepths = append(st.guideLineDepths, depth)
}

// layoutSeparator places (or reuses) the separator drawn above the current row.
func (r *treeViewContentRenderer) layoutSeparator(st *treeViewLayoutState, addSeparator bool) {
	if !addSeparator || st.hideSeparators {
		return
	}
	var separator core.CanvasObject
	if st.separatorCount < len(r.separators) {
		// a parked separator missed any override applied since
		separator = matchThemeScope(r.separators[st.separatorCount], r.treeContent.tree)
		separator.Show()
	} else {
		separator = matchThemeScope(NewSeparator(), r.treeContent.tree)
		r.separators = append(r.separators, separator)
	}
	separator.Move(geo.NewPos(0, st.y-st.separatorOff))
	separator.Resize(st.separatorSize)
	r.objects = append(r.objects, separator)
	st.separatorCount++
}

// recordLoadMore queues a load-more indicator for an open branch that has more
// children available.
func (r *treeViewContentRenderer) recordLoadMore(st *treeViewLayoutState, uid TreeViewNodeID, isBranch bool, depth int) {
	if !isBranch || !r.treeContent.tree.IsBranchOpen(uid) {
		return
	}
	tree := r.treeContent.tree
	tree.mu.RLock()
	n, ok := tree.nodeMap[uid]
	hasMore := ok && n.hasMore
	tree.mu.RUnlock()
	if hasMore {
		// Record position for load more indicator (after all children).
		st.loadMoreEntries = append(st.loadMoreEntries, treeViewLoadMoreEntry{
			parentID: uid,
			depth:    depth + 1,
		})
	}
}

// layoutLoadMoreIndicators positions the queued load-more indicators and kicks
// off any not-yet-running loads.
func (r *treeViewContentRenderer) layoutLoadMoreIndicators(st *treeViewLayoutState) {
	tree := r.treeContent.tree
	for _, entry := range st.loadMoreEntries {
		st.y += st.pad
		activity := r.getLoadMoreIndicator(entry.parentID)
		activityHeight := tree.leafMinSize.Height
		indentX := st.pad + float32(entry.depth)*st.indentWidth
		activity.Move(geo.NewPos(indentX, st.y))
		activity.Resize(geo.NewSize(st.width-indentX-st.pad, activityHeight))
		activity.Show()
		r.objects = append(r.objects, activity)
		st.y += activityHeight

		// Trigger load if not already loading
		tree.mu.Lock()
		if !tree.loadingInProgress[entry.parentID] {
			tree.loadingInProgress[entry.parentID] = true
			tree.mu.Unlock()
			if f := tree.OnLoadMore; f != nil {
				f(entry.parentID)
			}
		} else {
			tree.mu.Unlock()
		}
	}

	r.pruneLoadMoreIndicators(st.loadMoreEntries)
}

// pruneLoadMoreIndicators stops and drops cached load-more Activities for
// branches that no longer show an indicator this pass (collapsed, fully loaded,
// or scrolled out). Without it the map retains one spinner per branch that ever
// showed "load more" for the whole lifetime of the tree renderer.
func (r *treeViewContentRenderer) pruneLoadMoreIndicators(active []treeViewLoadMoreEntry) {
	if len(r.loadMoreIndicators) == 0 {
		return
	}
	keep := make(map[TreeViewNodeID]struct{}, len(active))
	for _, entry := range active {
		keep[entry.parentID] = struct{}{}
	}
	for id, activity := range r.loadMoreIndicators {
		if _, ok := keep[id]; !ok {
			activity.Stop()
			delete(r.loadMoreIndicators, id)
		}
	}
}

// layoutGuideLines draws (or hides) the vertical guide lines for the depths
// recorded during the walk.
func (r *treeViewContentRenderer) layoutGuideLines(st *treeViewLayoutState, size geo.Size) {
	if !st.showGuideLines {
		for _, line := range r.guideLines {
			line.Hide()
		}
		return
	}

	guideIdx := 0
	guideColor := st.th.Color(theme.ColorNameOutlineVariant, st.v)
	guideThickness := st.th.Size(theme.SizeNameSeparatorThickness)
	for _, depth := range st.guideLineDepths {
		x := st.pad + float32(depth)*st.indentWidth + st.iconSize/2
		var line *canvas.Rectangle
		if guideIdx < len(r.guideLines) {
			line = r.guideLines[guideIdx].(*canvas.Rectangle)
			line.Show()
		} else {
			line = canvas.NewRectangle(guideColor)
			r.guideLines = append(r.guideLines, line)
		}
		line.FillColor = guideColor
		line.Move(geo.NewPos(x, 0))
		line.Resize(geo.NewSize(guideThickness, size.Height))
		r.objects = append(r.objects, line)
		guideIdx++
	}
	for ; guideIdx < len(r.guideLines); guideIdx++ {
		r.guideLines[guideIdx].Hide()
	}
}

func (r *treeViewContentRenderer) MinSize() geo.Size {
	if !r.minSizeCache.IsZero() {
		return r.minSizeCache
	}

	tree := r.treeContent.tree
	th := r.treeContent.Theme()
	pad := th.Size(theme.SizeNamePadding)
	width := float32(0)
	height := float32(0)
	iconSize := th.Size(theme.SizeNameInlineIcon)

	tree.walkAll(func(uid, _ TreeViewNodeID, isBranch bool, depth int) {
		if height > 0 {
			height += pad
		}
		m := tree.leafMinSize
		if isBranch {
			m = tree.branchMinSize
		}
		indent := float32(depth) * (iconSize + pad)
		w := m.Width + indent
		if w > width {
			width = w
		}
		height += m.Height
	})

	r.minSizeCache = geo.NewSize(width, height)
	return r.minSizeCache
}

func (r *treeViewContentRenderer) Refresh() {
	r.minSizeCache = geo.Size{}
	id := r.treeContent.nextRefreshID
	r.treeContent.nextRefreshID = ""

	if id == allTreeViewNodesID || id == "" {
		r.Layout(r.treeContent.Size())
	} else if id == onlyNewTreeViewNodesID {
		r.Layout(r.treeContent.Size())
	} else {
		// Refresh single item
		if item, ok := r.items[id]; ok {
			item.Refresh()
		}
	}
}

func (r *treeViewContentRenderer) Objects() []core.CanvasObject {
	return r.objects
}

// Destroy releases the renderers of the separators the content keeps for
// reuse: those it is not showing are not among its objects, so the framework's
// destroy walk would not reach them (#722). Every shown child, including the
// load-more Activity widgets, whose own renderer.Destroy stops their
// animation, is reachable via Objects and torn down by that walk. The
// indicator maps are plain memory. Items in the itemPool keep their cached
// renderers, which nothing here can reach (#740).
func (r *treeViewContentRenderer) Destroy() {
	for _, s := range r.separators {
		releaseRenderer(s)
	}
}

func (r *treeViewContentRenderer) showDropIndicator(drop dropPosition, tree *TreeView) {
	if drop.targetID == "" {
		r.hideDropIndicator()
		return
	}
	th := r.treeContent.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()
	pad := th.Size(theme.SizeNamePadding)

	if drop.location == dropInside {
		// Highlight the target item's background
		if item, ok := r.items[drop.targetID]; ok {
			if ir := cache.Renderer(item); ir != nil {
				tir := ir.(*treeViewItemRenderer)
				tir.bg.FillColor = th.Color(theme.ColorNameSecondaryContainer, v)
				tir.bg.Refresh()
			}
		}
		if r.dropIndicator != nil {
			r.dropIndicator.Hide()
		}
		return
	}

	// Before/After: show insertion line
	if r.dropIndicator == nil {
		r.dropIndicator = canvas.NewRectangle(th.Color(theme.ColorNamePrimary, v))
		r.dropIndicator.CornerRadius = 1
	}
	r.dropIndicator.FillColor = th.Color(theme.ColorNamePrimary, v)

	// Find Y position of the target node
	y := float32(0)
	targetY := float32(-1)
	targetH := float32(0)
	tree.walkAll(func(uid, _ TreeViewNodeID, isBranch bool, depth int) {
		if y > 0 {
			y += pad
		}
		m := tree.leafMinSize
		if isBranch {
			m = tree.branchMinSize
		}
		if uid == drop.targetID {
			targetY = y
			targetH = m.Height
		}
		y += m.Height
	})

	if targetY < 0 {
		r.dropIndicator.Hide()
		return
	}

	lineY := targetY
	if drop.location == dropAfter {
		lineY = targetY + targetH + pad/2
	} else {
		lineY -= pad / 2
	}

	viewport := r.treeContent.viewport
	r.dropIndicator.Move(geo.NewPos(pad, lineY))
	r.dropIndicator.Resize(geo.NewSize(viewport.Width-pad*2, 2))
	r.dropIndicator.Show()
	r.dropIndicator.Refresh()
}

func (r *treeViewContentRenderer) hideDropIndicator() {
	if r.dropIndicator != nil {
		r.dropIndicator.Hide()
	}
}

func (r *treeViewContentRenderer) getLoadMoreIndicator(parentID TreeViewNodeID) *Activity {
	if r.loadMoreIndicators == nil {
		r.loadMoreIndicators = make(map[TreeViewNodeID]*Activity)
	}
	if a, ok := r.loadMoreIndicators[parentID]; ok {
		return matchThemeScope(a, r.treeContent.tree)
	}
	a := matchThemeScope(NewActivity(), r.treeContent.tree)
	a.Start()
	r.loadMoreIndicators[parentID] = a
	return a
}

func (r *treeViewContentRenderer) getItem(isBranch bool) *treeViewItem {
	if obj := r.itemPool.Get(); obj != nil {
		return matchThemeScope(obj.(*treeViewItem), r.treeContent.tree)
	}

	tree := r.treeContent.tree
	var content core.CanvasObject
	if f := tree.CreateNode; f != nil {
		content = createItemAndApplyThemeScope(func() core.CanvasObject { return f(isBranch) }, tree)
	} else {
		content = NewLabel("") // default
	}
	return matchThemeScope(newTreeViewItem(tree, content, isBranch), tree)
}

func (r *treeViewContentRenderer) updateDefaultContent(item *treeViewItem, uid TreeViewNodeID) {
	tree := r.treeContent.tree
	tree.mu.RLock()
	n, ok := tree.nodeMap[uid]
	tree.mu.RUnlock()
	if !ok {
		return
	}

	if lbl, ok := item.content.(*Label); ok {
		lbl.SetText(n.Text)
	}

	item.nodeIcon = n.Icon
}

// === Layer 3: treeViewItem ===

var (
	_ event.Tappable          = (*treeViewItem)(nil)
	_ event.DoubleTappable    = (*treeViewItem)(nil)
	_ event.SecondaryTappable = (*treeViewItem)(nil)
	_ event.Draggable         = (*treeViewItem)(nil)
	_ event.Hoverable         = (*treeViewItem)(nil)
)

type treeViewItem struct {
	BaseWidget
	tree     *TreeView
	uid      TreeViewNodeID
	depth    int
	isBranch bool
	content  core.CanvasObject
	nodeIcon bellui.Resource

	hovered  bool
	renaming bool
}

func newTreeViewItem(tree *TreeView, content core.CanvasObject, isBranch bool) *treeViewItem {
	item := &treeViewItem{
		tree:     tree,
		content:  content,
		isBranch: isBranch,
	}
	item.ExtendBaseWidget(item)
	return item
}

func (ti *treeViewItem) update(uid TreeViewNodeID, depth int, isBranch bool) {
	if ti.uid != uid {
		// This row is being recycled for a different node, so any inline editor
		// on it belonged to the node it used to show. A row that keeps its node
		// keeps its editor: layout runs on every resize, scroll and refresh, and
		// clearing here would throw away whatever the user was typing.
		ti.renaming = false
	}
	ti.uid = uid
	ti.depth = depth
	ti.isBranch = isBranch
	ti.Refresh()
}

func (ti *treeViewItem) Tapped(*event.PointEvent) {
	if ti.renaming {
		return
	}
	ti.tree.Select(ti.uid)
}

func (ti *treeViewItem) DoubleTapped(*event.PointEvent) {
	if ti.tree.AllowRename {
		ti.startRename()
		return
	}
	if ti.isBranch {
		ti.tree.ToggleBranch(ti.uid)
	}
}

func (ti *treeViewItem) TappedSecondary(e *event.PointEvent) {
	items := ti.contextMenuItems()
	if len(items) == 0 {
		return
	}
	menu := bellui.NewMenu("", items...)
	c := bellui.CurrentApp().Driver().CanvasForObject(ti)
	pos := bellui.CurrentApp().Driver().AbsolutePositionForObject(ti)
	popup := NewPopUpMenu(menu, c)
	inheritDirection(popup, ti) // #749
	popup.ShowAtPosition(pos.Add(e.Position))
}

// contextMenuItems builds the row's right-click menu: the built-in entries the
// tree's toggles allow, then whatever the host's ContextActions contributes for
// this node. An empty result means no menu is worth showing.
func (ti *treeViewItem) contextMenuItems() []*bellui.MenuItem {
	var items []*bellui.MenuItem
	if ti.tree.AllowRename {
		items = append(items, bellui.NewMenuItem(lang.L("Rename"), func() {
			ti.startRename()
		}))
	}
	if ti.tree.AllowDelete {
		items = append(items, bellui.NewMenuItem(lang.L("Delete"), func() {
			ti.tree.DeleteNode(ti.uid)
		}))
	}

	f := ti.tree.ContextActions
	if f == nil {
		return items
	}
	extra := f(ti.uid)
	if len(extra) == 0 {
		return items
	}
	if len(items) > 0 {
		items = append(items, bellui.NewMenuItemSeparator())
	}
	return append(items, extra...)
}

func (ti *treeViewItem) startRename() {
	ti.renaming = true

	tir, _ := cache.Renderer(ti).(*treeViewItemRenderer)
	if tir != nil && tir.renameEntry != nil {
		// Populate the editor here, once — not in Refresh, which runs on every
		// layout pass and would rewrite the entry with the model's text while
		// the user is typing in it.
		ti.tree.mu.RLock()
		text := ""
		if n, ok := ti.tree.nodeMap[ti.uid]; ok {
			text = n.Text
		}
		ti.tree.mu.RUnlock()
		tir.renameEntry.SetText(text)
	}

	ti.Refresh()

	if tir != nil && tir.renameEntry != nil {
		c := bellui.CurrentApp().Driver().CanvasForObject(ti)
		if c != nil {
			c.Focus(tir.renameEntry)
		}
	}
}

// cancelRename leaves the inline editor without writing anything back.
func (ti *treeViewItem) cancelRename() {
	if !ti.renaming {
		return
	}
	ti.renaming = false
	ti.Refresh()
}

func (ti *treeViewItem) commitRename(newText string) {
	if !ti.renaming {
		return
	}
	ti.renaming = false
	if newText != "" {
		ti.tree.RenameNode(ti.uid, newText)
	}
	ti.Refresh()
}

func (ti *treeViewItem) HoverIn(*event.HoverEvent)    { ti.setHovered(true) }
func (ti *treeViewItem) HoverMoved(*event.HoverEvent) {}
func (ti *treeViewItem) HoverOut()                    { ti.setHovered(false) }

// AccessibleRole returns [core.RoleTreeItem].
func (ti *treeViewItem) AccessibleRole() core.AccessibleRole { return core.RoleTreeItem }

// AccessibleLabel returns the node's text; custom CreateNode content
// provides its own semantics as child nodes instead.
func (ti *treeViewItem) AccessibleLabel() string {
	if ti.tree.CreateNode != nil {
		return ""
	}
	ti.tree.mu.RLock()
	defer ti.tree.mu.RUnlock()
	if n, ok := ti.tree.nodeMap[ti.uid]; ok {
		return n.Text
	}
	return ""
}

// AccessibleState reports the selected, expanded and focused states.
func (ti *treeViewItem) AccessibleState() core.AccessibleState {
	var state core.AccessibleState
	ti.tree.mu.RLock()
	if ti.tree.selected == ti.uid {
		state |= core.StateSelected
	}
	ti.tree.mu.RUnlock()
	if ti.isBranch && ti.tree.IsBranchOpen(ti.uid) {
		state |= core.StateExpanded
	}
	if ti.tree.focused && ti.tree.currentFocus == ti.uid {
		state |= core.StateFocused
	}
	return state
}

// AccessibleActions returns "select", plus "expand"/"collapse" for branches
// and "rename"/"delete" when the tree allows them — the on-row buttons for
// those are hover-only, so without these actions they would be unreachable
// for assistive technology and keyboard users (pre-rebrand tracker #544).
func (ti *treeViewItem) AccessibleActions() []core.AccessibleAction {
	actions := []core.AccessibleAction{{Name: core.ActionSelect, Description: "Select this item"}}
	if ti.isBranch {
		if ti.tree.IsBranchOpen(ti.uid) {
			actions = append(actions, core.AccessibleAction{Name: core.ActionCollapse, Description: "Collapse this branch"})
		} else {
			actions = append(actions, core.AccessibleAction{Name: core.ActionExpand, Description: "Expand this branch"})
		}
	}
	if ti.tree.AllowRename {
		actions = append(actions, core.AccessibleAction{Name: core.ActionRename, Description: "Rename this item"})
	}
	if ti.tree.AllowDelete {
		actions = append(actions, core.AccessibleAction{Name: core.ActionDelete, Description: "Delete this item"})
	}
	return actions
}

// PerformAccessibleAction selects the row, toggles branch expansion, or
// starts rename / deletes the node when the tree allows it.
func (ti *treeViewItem) PerformAccessibleAction(name string) {
	switch name {
	case core.ActionSelect:
		if !ti.renaming {
			ti.tree.Select(ti.uid)
		}
	case core.ActionExpand:
		if ti.isBranch {
			ti.tree.OpenBranch(ti.uid)
		}
	case core.ActionCollapse:
		if ti.isBranch {
			ti.tree.CloseBranch(ti.uid)
		}
	case core.ActionRename:
		if ti.tree.AllowRename && !ti.renaming {
			ti.startRename()
		}
	case core.ActionDelete:
		if ti.tree.AllowDelete {
			ti.tree.DeleteNode(ti.uid)
		}
	}
}

// AccessibleLevel returns the row's 1-based nesting level; rendered trees
// lay rows out flat, so this carries the outline structure.
func (ti *treeViewItem) AccessibleLevel() int { return ti.depth + 1 }

func (ti *treeViewItem) setHovered(hovered bool) {
	if ti.hovered == hovered {
		return
	}
	ti.hovered = hovered
	ti.Refresh()
}

// rowHoverButton is an IconButton inside a tree row that keeps the row in its
// hovered state while the pointer is over the button. Hover is dispatched to
// the deepest Hoverable only, so a plain IconButton would steal hover from the
// row — hiding the very action buttons the pointer is on and dropping the row
// highlight while over the chevron.
type rowHoverButton struct {
	IconButton
	row *treeViewItem
}

func newRowHoverButton(icon bellui.Resource, row *treeViewItem, tapped func()) *rowHoverButton {
	b := &rowHoverButton{row: row}
	b.Icon = icon
	b.OnTapped = tapped
	b.ExtendBaseWidget(b)
	return b
}

func (b *rowHoverButton) HoverIn(e *event.HoverEvent) {
	b.IconButton.HoverIn(e)
	b.row.setHovered(true)
}

func (b *rowHoverButton) HoverOut() {
	b.IconButton.HoverOut()
	b.row.setHovered(false)
}

func (ti *treeViewItem) Dragged(e *event.DragEvent) {
	if !ti.tree.AllowReorder && !ti.tree.AllowReparent {
		return
	}
	ti.tree.handleDrag(ti.uid, e)
}

func (ti *treeViewItem) DragEnd() {
	if !ti.tree.AllowReorder && !ti.tree.AllowReparent {
		return
	}
	ti.tree.finalizeDrag()
}

func (ti *treeViewItem) CreateRenderer() core.WidgetRenderer {
	ti.ExtendBaseWidget(ti)
	th := ti.Theme()

	bg := canvas.NewRectangle(color.Transparent)
	bg.CornerRadius = th.Size(theme.SizeNameShapeSmall)

	chevron := newRowHoverButton(theme.NavigateNextIcon(), ti, func() {
		ti.tree.ToggleBranch(ti.uid)
	})

	var icon *canvas.Image
	if ti.nodeIcon != nil {
		icon = canvas.NewImageFromResource(ti.nodeIcon)
		icon.FillMode = canvas.ImageFillContain
	}

	// Rename entry (created once, shown/hidden as needed)
	renameEntry := NewEntry()
	renameEntry.OnSubmitted = func(text string) {
		ti.commitRename(text)
	}
	// Clicking or tabbing away ends the rename. Layout used to end it as a side
	// effect of rebuilding every row, which also ended renames nobody meant to
	// cancel; this is the deliberate exit that replaces it. onFocusChanged is a
	// single-slot hook (Form claims it on entries it owns); taking it is safe
	// only because renameEntry never leaves this renderer.
	renameEntry.onFocusChanged = func(focused bool) {
		if !focused {
			ti.cancelRename()
		}
	}
	renameEntry.Hide()

	// Action buttons (shown on hover)
	var renameBtn, deleteBtn *rowHoverButton
	renameBtn = newRowHoverButton(theme.DocumentCreateIcon(), ti, func() {
		ti.startRename()
	})
	renameBtn.SetAccessibilityLabel(lang.L("Rename"))
	renameBtn.Hide()
	deleteBtn = newRowHoverButton(theme.DeleteIcon(), ti, func() {
		ti.tree.DeleteNode(ti.uid)
	})
	deleteBtn.SetAccessibilityLabel(lang.L("Delete"))
	deleteBtn.Hide()

	return &treeViewItemRenderer{
		item:        ti,
		bg:          bg,
		chevron:     chevron,
		icon:        icon,
		content:     ti.content,
		renameEntry: renameEntry,
		renameBtn:   renameBtn,
		deleteBtn:   deleteBtn,
	}
}

// === Layer 3: treeViewItemRenderer ===

type treeViewItemRenderer struct {
	item        *treeViewItem
	bg          *canvas.Rectangle
	chevron     *rowHoverButton
	icon        *canvas.Image
	content     core.CanvasObject
	renameEntry *Entry
	renameBtn   *rowHoverButton
	deleteBtn   *rowHoverButton
}

func (r *treeViewItemRenderer) MinSize() geo.Size {
	th := r.item.Theme()
	pad := th.Size(theme.SizeNamePadding)
	iconSize := th.Size(theme.SizeNameInlineIcon)

	contentMin := r.content.MinSize()
	w := pad + iconSize + pad + contentMin.Width + pad // chevron + content + padding
	if r.item.nodeIcon != nil {
		w += iconSize + pad // icon
	}
	h := geo.Max(iconSize, contentMin.Height) + pad*2
	return geo.NewSize(w, h)
}

func (r *treeViewItemRenderer) Layout(size geo.Size) {
	th := r.item.Theme()
	pad := th.Size(theme.SizeNamePadding)
	iconSize := th.Size(theme.SizeNameInlineIcon)
	indentWidth := iconSize + pad

	r.bg.Resize(size)

	x := pad + float32(r.item.depth)*indentWidth

	// Chevron
	if r.item.isBranch {
		r.chevron.Show()
		r.chevron.Move(geo.NewPos(x, (size.Height-iconSize)/2))
		r.chevron.Resize(geo.NewSize(iconSize, iconSize))
	} else {
		r.chevron.Hide()
	}
	x += iconSize + pad

	// Node icon
	if r.icon != nil && r.item.nodeIcon != nil {
		r.icon.Show()
		r.icon.Resource = r.item.nodeIcon
		r.icon.Move(geo.NewPos(x, (size.Height-iconSize)/2))
		r.icon.Resize(geo.NewSize(iconSize, iconSize))
		x += iconSize + pad
	} else if r.icon != nil {
		r.icon.Hide()
	}

	// Action buttons on right (when hovered)
	rightX := size.Width - pad
	btnSize := iconSize
	if r.item.hovered && !r.item.renaming {
		if r.item.tree.AllowDelete {
			rightX -= btnSize
			r.deleteBtn.Show()
			r.deleteBtn.Move(geo.NewPos(rightX, (size.Height-btnSize)/2))
			r.deleteBtn.Resize(geo.NewSize(btnSize, btnSize))
			rightX -= pad / 2
		} else {
			r.deleteBtn.Hide()
		}
		if r.item.tree.AllowRename {
			rightX -= btnSize
			r.renameBtn.Show()
			r.renameBtn.Move(geo.NewPos(rightX, (size.Height-btnSize)/2))
			r.renameBtn.Resize(geo.NewSize(btnSize, btnSize))
			rightX -= pad / 2
		} else {
			r.renameBtn.Hide()
		}
	} else {
		r.renameBtn.Hide()
		r.deleteBtn.Hide()
	}

	// Content or rename entry
	contentWidth := rightX - x - pad
	if r.item.renaming {
		r.content.Hide()
		r.renameEntry.Show()
		entryHeight := r.renameEntry.MinSize().Height
		r.renameEntry.Move(geo.NewPos(x, (size.Height-entryHeight)/2))
		r.renameEntry.Resize(geo.NewSize(contentWidth, entryHeight))
	} else {
		r.renameEntry.Hide()
		r.content.Show()
		contentHeight := r.content.MinSize().Height
		r.content.Move(geo.NewPos(x, (size.Height-contentHeight)/2))
		r.content.Resize(geo.NewSize(contentWidth, contentHeight))
	}
}

func (r *treeViewItemRenderer) Refresh() {
	th := r.item.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()

	// Background color
	r.item.tree.mu.RLock()
	selected := r.item.uid == r.item.tree.selected
	r.item.tree.mu.RUnlock()
	focused := r.item.uid == r.item.tree.currentFocus && r.item.tree.focused
	hovered := r.item.hovered

	if selected && hovered {
		base := th.Color(theme.ColorNameSecondaryContainer, v)
		overlay := theme.StateLayerHover(th.Color(theme.ColorNameOnSecondaryContainer, v))
		r.bg.FillColor = blendColor(base, overlay)
	} else if selected {
		r.bg.FillColor = th.Color(theme.ColorNameSecondaryContainer, v)
	} else if hovered {
		r.bg.FillColor = theme.StateLayerHover(th.Color(theme.ColorNameOnSurface, v))
	} else if focused {
		r.bg.FillColor = theme.StateLayerFocus(th.Color(theme.ColorNameOnSurface, v))
	} else {
		r.bg.FillColor = color.Transparent
	}
	r.bg.CornerRadius = th.Size(theme.SizeNameShapeSmall)
	r.bg.Refresh()

	// Chevron icon
	if r.item.isBranch {
		if r.item.tree.IsBranchOpen(r.item.uid) {
			r.chevron.Icon = theme.MoveDownIcon()
			r.chevron.SetAccessibilityLabel(lang.L("Collapse"))
		} else {
			r.chevron.Icon = theme.NavigateNextIcon()
			r.chevron.SetAccessibilityLabel(lang.L("Expand"))
		}
		r.chevron.Refresh()
	}

	// Node icon
	if r.item.nodeIcon != nil {
		if r.icon == nil {
			r.icon = canvas.NewImageFromResource(r.item.nodeIcon)
			r.icon.FillMode = canvas.ImageFillContain
		} else {
			r.icon.Resource = r.item.nodeIcon
			r.icon.Refresh()
		}
	}

	// Content
	if r.content != r.item.content {
		r.content = r.item.content
	}
	r.content.Refresh()

	r.Layout(r.item.Size())
}

func (r *treeViewItemRenderer) Objects() []core.CanvasObject {
	objs := []core.CanvasObject{r.bg, r.chevron, r.content, r.renameEntry, r.renameBtn, r.deleteBtn}
	if r.icon != nil {
		objs = append(objs, r.icon)
	}
	return objs
}

// Destroy has nothing to release: the child widgets (chevron, icon, content,
// inline rename Entry and action buttons) are returned by Objects and torn down
// by the framework's destroy walk, which stops the Entry's caret animation.
func (r *treeViewItemRenderer) Destroy() {}
