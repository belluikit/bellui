//go:build accessibility

package mobile

import (
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/mobile"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/accessibility"
	intdriver "github.com/belluikit/bellui/internal/driver"
	"github.com/belluikit/bellui/internal/scale"
	"github.com/belluikit/bellui/internal/textinput"
)

// Platform-neutral half of the mobile screen-reader backends (pre-rebrand tracker #488): flattens
// the semantic tree (pre-rebrand tracker #484) into the flat element lists VoiceOver and TalkBack
// consume, and owns the handle registry the platform bridges dispatch actions
// through. Nothing here touches a platform API, so it compiles (and is
// tested) in the host slice of the mobile driver too.
//
// Mobile assistive technologies navigate a flat sequence of elements (swipe
// right = next), unlike the nested trees the desktop backends publish:
// Android's overlay views are direct children of one container, and iOS
// accessibilityElements arrays are read in order. The flatten preserves the
// walker's tree order, which is reading order by contract.

// Wire roles shared by the iOS and Android bridges. The ObjC side maps them
// to UIAccessibilityTraits, the Java side to widget class names; both switch
// on these numbers, so keep them stable across the three languages.
const (
	mobileRoleButton    = 1
	mobileRoleText      = 2
	mobileRoleLink      = 3
	mobileRoleImage     = 4
	mobileRoleCheckbox  = 5
	mobileRoleSwitch    = 6
	mobileRoleRadio     = 7
	mobileRoleTab       = 8
	mobileRoleTextField = 9
	mobileRoleTextArea  = 10
	mobileRolePassword  = 11
	mobileRoleSearch    = 12
	mobileRoleSlider    = 13
	mobileRoleProgress  = 14
	mobileRoleSpin      = 15
	mobileRoleHeading   = 16
	mobileRoleListItem  = 17
	mobileRoleCombo     = 18
	mobileRoleMenuItem  = 19
	mobileRoleTreeItem  = 20
	mobileRoleCell      = 21
	// mobileRoleGrid is a virtualized grid/table container
	// (core.AccessibleVirtualCellRealizer): the one container role the flat
	// sequence keeps, so the platform side can hang the native
	// virtualized-collection pattern off it — iOS
	// UIAccessibilityContainerDataTable, Android CollectionInfo +
	// ACTION_SCROLL_TO_POSITION (pre-rebrand tracker #817). Plain containers still contribute no
	// element.
	mobileRoleGrid = 22
)

// Wire state/capability flags, a bitmask alongside the role.
const (
	mobileA11yFlagDisabled      = 1 << iota // announce as dimmed/not enabled
	mobileA11yFlagSelected                  // tab/list selection
	mobileA11yFlagChecked                   // checkbox/switch/radio on
	mobileA11yFlagCheckable                 // announces checked/unchecked vocabulary
	mobileA11yFlagPressable                 // double-tap activates via explicit dispatch
	mobileA11yFlagAdjustable                // swipe up/down increments/decrements
	mobileA11yFlagDismissable               // scrub/back gesture dismisses
	mobileA11yFlagFocused                   // holds the canvas keyboard focus
	mobileA11yFlagScrollable                // inside a scroll area: AT scroll gesture pages it
	mobileA11yFlagLongPressable             // AT long-press maps to LongPressed/TappedSecondary
	mobileA11yFlagTextEditable              // editable-text node protocol enabled (pre-rebrand tracker #974 option D)
)

// Action kinds the bridges send back through the dispatch exports. 5 is the
// Android-only state-changed pseudo action (see accessibility_android.go).
const (
	mobileA11yActionActivate       = 1
	mobileA11yActionIncrement      = 2
	mobileA11yActionDecrement      = 3
	mobileA11yActionDismiss        = 4
	mobileA11yActionScrollForward  = 6
	mobileA11yActionScrollBackward = 7
	mobileA11yActionLongPress      = 8

	// mobileA11yActionCustom is the base of the custom-action kind block
	// (pre-rebrand tracker #1025): a declared widget action with no native semantic (undo, redo,
	// expand, rename, ...) is dispatched as kind = base + its registry id
	// (a11yCustomActionIDs). Keep in sync with GoNativeActivity.java's
	// A11Y_ACTION_CUSTOM.
	mobileA11yActionCustom = 0x1000
)

// Text-edit action kinds an editable-text node's bridge sends back through the
// text-action dispatch (pre-rebrand tracker #974 option D). Offsets a/b are rune indices into the
// node's document — the platform side converts from its own code-unit space —
// and every edit lands on the editor's gated, undo-recorded paths via the
// internal/textinput primitives, exactly like the desktop backends.
const (
	mobileA11yTextActSetSelection = 1 // a=start, b=end; equal offsets place a bare caret
	mobileA11yTextActReplace      = 2 // replace [a, b) with text (cut: text == "")
	mobileA11yTextActSetText      = 3 // replace the whole document with text
	mobileA11yTextActInsert       = 4 // replace the current selection / insert at the caret
)

// a11yScrollPageFraction is how much of the scroller's viewport one screen-reader
// scroll gesture pages by — matching a native list's page-scroll feel.
const a11yScrollPageFraction = 0.8

// mobileA11yNode is one flattened element in bridge wire format: everything
// the platform side needs, pre-converted (bounds in native pixels).
type mobileA11yNode struct {
	handle             uintptr
	role, flags        int
	label, value, hint string
	x, y, w, h         int

	// gridHandle is the handle of the enclosing mobileRoleGrid container
	// element, 0 when the element is not inside a virtualized grid. The
	// bridges use it to associate cells with their collection (pre-rebrand tracker #817).
	gridHandle uintptr
	// cellRow/cellCol are the element's display grid coordinates
	// (core.AccessibleGridItem space), -1 when it is not a grid item.
	cellRow, cellCol int
	// rowCount/colCount are the full presented extent of a mobileRoleGrid
	// container (core.AccessibleGridDimensioner), 0 otherwise.
	rowCount, colCount int

	// customActs are the node's declared actions that no native semantic
	// consumes (pre-rebrand tracker #1025) — undo/redo on an editor, expand/rename/delete on a
	// tree row — offered to the AT as platform custom actions (TalkBack's
	// actions menu; the iOS mapping is a follow-up). Labels are the actions'
	// human-readable descriptions; ids index the process-wide name registry
	// the dispatch resolves against.
	customActs []mobileA11yCustomAct

	// text/selStart/selEnd carry the editable document for a text-input node
	// whose object implements core.AccessibleTexter (pre-rebrand tracker #974 option D): text is
	// AccessibleText — the live document, the mask for a password field —
	// which can differ from the spoken value (AccessibleValue is empty for
	// passwords and lags the live content under Entry.DeferTextSync), and
	// selStart/selEnd are its half-open selection as rune offsets (equal =
	// bare caret; -1/-1 when the node carries no document).
	text             string
	selStart, selEnd int

	// textLayout is the visual-layout sidecar of the FOCUSED editable node
	// (pre-rebrand tracker #995), "" for every other node: a bounded window of visual
	// (soft-wrapped) line boundaries around the caret plus per-character
	// rectangles around the selection, encoded by mobileA11yTextLayout. The
	// Android bridge consumes it for legacy-TalkBack LINE granularity and the
	// EXTRA_DATA_TEXT_CHARACTER_LOCATION_KEY request behind the granular
	// highlight/magnifier; iOS ignores it for now.
	textLayout string
}

// a11yTarget is what a dispatched action resolves against: the window the
// element belongs to and its node snapshot from the most recent gather. For
// an on-demand realized cell (pre-rebrand tracker #817) grid holds the handle of the virtualized
// container it was realized from — the cell has no CanvasObject, so activating
// it routes back to the grid's scroll-into-view instead.
type a11yTarget struct {
	win  *window
	node *accessibility.Node
	grid uintptr
}

// a11yWindowState keys the stable handles by object identity, or by a stable
// NodeID for nodes that opt into it (virtual nodes with no object, pre-rebrand tracker #533, and
// realized nodes of a virtualized collection whose object is recycled across
// records, AccessibleIdentified pre-rebrand tracker #635), so an element keeps its handle across
// frames and the screen reader's cursor survives unrelated repaints.
type a11yWindowState struct {
	handles map[core.CanvasObject]uintptr
	virtual map[any]uintptr

	// realized holds elements built on demand for a screen reader navigating a
	// virtualized grid past its viewport (pre-rebrand tracker #817), keyed by the cell's stable
	// NodeID. They are delivered outside the gather walk, so the sweep never
	// sees them: they leave by FIFO eviction, by promotion into virtual when
	// the same NodeID scrolls into the walked window, or with the window.
	realized      map[any]uintptr
	realizedOrder []any // FIFO of realized NodeIDs, oldest first, for eviction
}

// The gather runs on the main goroutine (from the paint loop), but the
// platform action callbacks arrive on the OS accessibility thread (UIKit main
// thread / Android UI thread), which under gomobile is not the main
// goroutine. The registry is therefore mutex-guarded — unlike the desktop
// backends, where both sides share one dispatch context. Widget code is still
// only ever called on the main goroutine: callbacks look up the target under
// the lock, then queue the actual dispatch via DoFromGoroutine.
var (
	a11yMu         sync.Mutex
	a11yWindows    = map[*window]*a11yWindowState{}
	a11yTargets    = map[uintptr]a11yTarget{}
	a11yNextHandle uintptr

	// Process-wide custom-action name registry (pre-rebrand tracker #1025): each non-standard
	// declared action name gets a small stable id the platform bridges carry
	// in place of the string, so the action up-call stays the existing
	// (handle, int-kind) channel. Append-only, so an id never changes meaning
	// — a stale menu pick after the action list changed resolves to the same
	// name (and is then re-validated against the node's current declaration).
	// Guarded by a11yMu.
	a11yCustomActionIDs   = map[string]int{}
	a11yCustomActionNames []string
)

// mobileA11yCustomAct is one wire custom action: the registry id of its name
// and its human-readable label.
type mobileA11yCustomAct struct {
	id    int
	label string
}

// mobileA11yGather flattens the current semantic tree of w into wire nodes,
// reusing handles for objects seen before. removed lists the handles whose
// objects left the tree this frame (the iOS bridge destroys their elements;
// Android rebuilds wholesale and only needs the registry pruned).
func (w *window) mobileA11yGather() (nodes []mobileA11yNode, removed []uintptr) {
	c := w.canvas
	forest := accessibility.BuildTree(w.mobileA11yRoots()...)

	var focusedObj core.CanvasObject
	if f := c.Focused(); f != nil {
		focusedObj, _ = f.(core.CanvasObject)
	}

	a11yMu.Lock()
	defer a11yMu.Unlock()

	st := a11yWindows[w]
	if st == nil {
		st = &a11yWindowState{
			handles:  make(map[core.CanvasObject]uintptr),
			virtual:  make(map[any]uintptr),
			realized: make(map[any]uintptr),
		}
		a11yWindows[w] = st
	}

	seen := make(map[core.CanvasObject]bool, len(st.handles))
	seenVirtual := make(map[any]bool, len(st.virtual))
	for _, n := range forest {
		nodes = w.flattenA11yNode(n, st, focusedObj, seen, seenVirtual, 0, nodes)
	}
	mobileA11ySortGridChildren(nodes)

	for obj, h := range st.handles {
		if !seen[obj] {
			removed = append(removed, h)
			delete(a11yTargets, h)
			delete(st.handles, obj)
		}
	}
	for id, h := range st.virtual {
		if !seenVirtual[id] {
			removed = append(removed, h)
			delete(a11yTargets, h)
			delete(st.virtual, id)
		}
	}
	return nodes, removed
}

// mobileA11yRoots returns the object roots the flat sequence is built from,
// honouring modal overlays: a modal (non-input-pass-through) overlay makes
// everything beneath it inert, so the sequence contains only that overlay and
// any transient (pass-through) overlays stacked above it — matching native
// modal semantics and the input hit-test barrier
// (internal/driver.FindObjectAtPositionMatching, pre-rebrand tracker #569). Without this the content
// under a dialog stays focusable and activatable by a screen reader (pre-rebrand tracker #807).
// Toast/Snackbar overlays pass input through, so they are not barriers and the
// content below stays reachable.
func (w *window) mobileA11yRoots() []core.CanvasObject {
	c := w.canvas
	overlays := c.Overlays().List()
	for i := len(overlays) - 1; i >= 0; i-- {
		if !overlayA11yPassesThrough(overlays[i]) {
			return overlays[i:]
		}
	}
	var roots []core.CanvasObject
	if content := c.Content(); content != nil {
		roots = append(roots, content)
	}
	if c.windowHead != nil {
		roots = append(roots, c.windowHead)
	}
	if c.menu != nil {
		roots = append(roots, c.menu)
	}
	return append(roots, overlays...)
}

// overlayA11yPassesThrough reports whether an overlay lets input (and thus a
// screen reader) reach the layers beneath it — a tooltip/Toast/Snackbar — rather
// than acting as a modal barrier. Mirrors internal/driver.overlayPassesInput.
func overlayA11yPassesThrough(o core.CanvasObject) bool {
	pt, ok := o.(intdriver.InputPassThrough)
	return ok && pt.PassesInputThrough()
}

// flattenA11yNode appends the wire node for n (when it is exposable) and
// recurses into its children. A node scrolled wholly outside its clip bounds
// is culled with its subtree, like the desktop backends. gridHandle is the
// handle of the nearest enclosing virtualized-grid element (0 outside one);
// a virtualized grid keeps a container element of its own — the one container
// the flat sequence preserves — and its descendants carry its handle (pre-rebrand tracker #817).
func (w *window) flattenA11yNode(n *accessibility.Node, st *a11yWindowState, focusedObj core.CanvasObject,
	seen map[core.CanvasObject]bool, seenVirtual map[any]bool, gridHandle uintptr, out []mobileA11yNode,
) []mobileA11yNode {
	if n.HasClip && !n.Size.IsZero() && !a11yRectsOverlap(geo.Rectangle{Pos: n.Pos, Size: n.Size}, n.Clip) {
		return out
	}

	if node, ok := w.a11yWireNode(n, st, focusedObj, seen, seenVirtual, gridHandle); ok {
		out = append(out, node)
		if node.role == mobileRoleGrid {
			gridHandle = node.handle
		}
	}
	for _, c := range n.Children {
		out = w.flattenA11yNode(c, st, focusedObj, seen, seenVirtual, gridHandle, out)
	}
	return out
}

func (w *window) a11yWireNode(n *accessibility.Node, st *a11yWindowState, focusedObj core.CanvasObject,
	seen map[core.CanvasObject]bool, seenVirtual map[any]bool, gridHandle uintptr,
) (mobileA11yNode, bool) {
	role, ok := mobileA11yRole(n)
	if !ok {
		// A virtualized grid container is the one container role that keeps an
		// element: the platform virtualized-collection patterns (pre-rebrand tracker #817) hang off
		// it. Gated on the realize seam plus a reported extent, so plain
		// tables/lists keep the flat container-free sequence.
		if _, realizer := n.Object.(core.AccessibleVirtualCellRealizer); realizer && n.HasGridDims {
			role = mobileRoleGrid
			ok = true
		}
	}
	if !ok || n.Size.Width <= 0 || n.Size.Height <= 0 {
		return mobileA11yNode{}, false
	}
	// Bounds are clipped to the visibly-usable viewport: a virtualized row
	// realized just past the fold (overscan) partially overlaps its scroll clip,
	// so it survives the subtree cull, but its full bounds run below the viewport
	// and under the system nav bar. Reporting the clipped bounds keeps the screen
	// reader (and the in-scene focus ring, pre-rebrand tracker #759) inside the usable area, and a
	// row wholly outside it is dropped from the focus order (pre-rebrand tracker #808).
	bounds, visible := w.a11yVisibleBounds(n)
	if !visible {
		return mobileA11yNode{}, false
	}

	var handle uintptr
	var exists bool
	switch {
	case n.NodeID != nil:
		// Nodes with a stable identity reconcile against it rather than their
		// CanvasObject: virtual nodes with no object (pre-rebrand tracker #533) and realized nodes
		// of a virtualized collection whose object is recycled across records
		// (AccessibleIdentified, pre-rebrand tracker #635), so the element tracks the record.
		handle, exists = st.virtual[n.NodeID]
		if !exists {
			// Promotion (pre-rebrand tracker #817): a cell realized on demand while off-viewport
			// has scrolled into the walked window. Reuse its element so the
			// screen reader's cursor survives the realize → visible transition.
			if h, ok := st.realized[n.NodeID]; ok {
				handle, exists = h, true
				st.realizedOrder = accessibility.DropRealizedID(st.realizedOrder, n.NodeID)
				delete(st.realized, n.NodeID)
			}
		}
	case n.Object != nil:
		handle, exists = st.handles[n.Object]
	default:
		return mobileA11yNode{}, false
	}
	if !exists {
		a11yNextHandle++
		handle = a11yNextHandle
	}
	if n.NodeID != nil {
		st.virtual[n.NodeID] = handle
		seenVirtual[n.NodeID] = true
	} else {
		st.handles[n.Object] = handle
		seen[n.Object] = true
	}
	a11yTargets[handle] = a11yTarget{win: w, node: n, grid: gridHandle}

	flags := mobileA11yFlags(n)
	if n.Object != nil && n.Object == focusedObj {
		flags |= mobileA11yFlagFocused
	}
	node := mobileA11yNode{
		handle:     handle,
		role:       role,
		flags:      flags,
		label:      n.Label,
		value:      n.Value,
		hint:       n.Description,
		x:          scale.ToScreenCoordinate(w.canvas, bounds.Pos.X),
		y:          scale.ToScreenCoordinate(w.canvas, bounds.Pos.Y),
		w:          scale.ToScreenCoordinate(w.canvas, bounds.Size.Width),
		h:          scale.ToScreenCoordinate(w.canvas, bounds.Size.Height),
		gridHandle: gridHandle,
		cellRow:    -1,
		cellCol:    -1,
		selStart:   -1,
		selEnd:     -1,
	}
	if n.HasCellPosition {
		node.cellRow, node.cellCol = n.CellRow, n.CellCol
	}
	if role == mobileRoleGrid {
		node.rowCount, node.colCount = n.GridRows, n.GridCols
	}
	node.customActs = mobileA11yCustomActions(n)
	// A text-input node backed by a core.AccessibleTexter publishes its live
	// document and selection so the platform side can run the native editable-
	// text protocol against it (pre-rebrand tracker #974 option D). Reading the document is cheap
	// per frame: Entry's provider memoizes the materialized string between
	// edits. Read-only and disabled editors keep the document (the AT can
	// still read and select) but do not enable the edit protocol.
	if mobileA11yTextInputRole(role) {
		if ed, ok := n.Object.(core.AccessibleTexter); ok {
			view := textinput.Read(ed)
			node.text = view.Text
			node.selStart, node.selEnd = view.SelStart, view.SelEnd
			if !n.State.Has(core.StateReadOnly) && !n.State.Has(core.StateDisabled) {
				node.flags |= mobileA11yFlagTextEditable
			}
			// The focused editor additionally publishes its visual layout
			// (pre-rebrand tracker #995) when the widget exposes one (core.AccessibleTextLayouter,
			// Entry does) — bounded around the caret, so the per-frame cost
			// stays O(window) however large the document.
			if node.flags&mobileA11yFlagFocused != 0 && node.text != "" && node.selStart >= 0 {
				if lay, ok := n.Object.(core.AccessibleTextLayouter); ok {
					node.textLayout = mobileA11yTextLayout(lay, node.text,
						node.selStart, node.selEnd, n.Pos,
						func(v float32) int { return scale.ToScreenCoordinate(w.canvas, v) })
				}
			}
		}
	}
	return node, true
}

// Bounds of the per-frame visual-layout sidecar (pre-rebrand tracker #995): visual lines fed
// either side of the caret line, and the per-character rectangle window
// around the selection. Both keep the sidecar O(window) per dirty frame on
// arbitrarily large documents; a granularity move outside the line window
// falls back to logical lines on the Java side, and character-location
// requests outside the rectangle window answer null rects (TalkBack then
// falls back to the element bounds).
const (
	mobileA11yLineWindow    = 100
	mobileA11yCharWindowMax = 256
)

// a11yVisualLayout is the layout subset of [core.AccessibleTextLayouter] the
// sidecar encoder consumes — narrowed so tests can fake it without the full
// AccessibleTexter surface.
type a11yVisualLayout interface {
	AccessibleLineCount() int
	AccessibleLineForOffset(off int) int
	AccessibleLineRange(line int) (start, end int, ok bool)
	AccessibleRangeBounds(start, end int) []geo.Rectangle
}

// mobileA11yTextLayout encodes the focused editable node's visual-layout
// sidecar (pre-rebrand tracker #995). Two sections, '\n'-separated:
//
//	L:<b0>,<b1>,...   — UTF-16 boundary offsets of consecutive visual lines
//	                    [caretLine-window, caretLine+window]: line i of the
//	                    window is [b_i, b_i+1); the final boundary is the end
//	                    of the last covered line (== the text length when the
//	                    window reaches the document end).
//	C:<start16>:<x>,<y>,<w>,<h>;-;...
//	                  — per-UTF-16-unit character rectangles in native pixels
//	                    (canvas space, the same space as the node bounds) for
//	                    a bounded window around the selection; '-' = no
//	                    geometry (scrolled out of the viewport). Both units
//	                    of a surrogate pair repeat their rune's rectangle, so
//	                    the Java side indexes by UTF-16 char directly.
//
// selStart/selEnd are rune offsets (the wire selection); origin is the
// node's absolute canvas position in dp; toPx converts dp to native pixels.
func mobileA11yTextLayout(lay a11yVisualLayout, text string,
	selStart, selEnd int, origin geo.Position, toPx func(float32) int,
) string {
	ix := accessibility.BuildTextIndex(text)
	lineCount := lay.AccessibleLineCount()
	if lineCount <= 0 {
		return ""
	}

	var b strings.Builder
	caretLine := lay.AccessibleLineForOffset(selEnd)
	lo := max(0, caretLine-mobileA11yLineWindow)
	hi := min(lineCount-1, caretLine+mobileA11yLineWindow)
	b.WriteString("L:")
	wrote := false
	lastEnd := 0
	for line := lo; line <= hi; line++ {
		start, end, ok := lay.AccessibleLineRange(line)
		if !ok {
			continue
		}
		if wrote {
			b.WriteByte(',')
		}
		b.WriteString(strconv.Itoa(ix.UTF16FromRune(start)))
		wrote = true
		lastEnd = end
	}
	if !wrote {
		return ""
	}
	b.WriteByte(',')
	b.WriteString(strconv.Itoa(ix.UTF16FromRune(lastEnd)))

	// Character rectangles: the visual lines around the selection, capped at
	// mobileA11yCharWindowMax chars around the free cursor (selEnd). The span
	// matches the L: line window (±mobileA11yLineWindow) rather than only the
	// caret's line ±1: iOS answers VoiceOver's per-line ring (pre-rebrand tracker #1107) from these
	// rects via selectionRectsForRange:, and VoiceOver queries the ring for any
	// nearby line the user taps or navigates to — not just the caret's — so a
	// ±1-line window left every other line to fall back to the proxy's
	// mismatched geometry (pre-rebrand tracker #1113). Android's granular highlight/magnifier only
	// reads the caret's own line, so the wider (still 256-char-capped) window is
	// harmless there.
	selLo, selHi := selStart, selEnd
	if selLo > selHi {
		selLo, selHi = selHi, selLo
	}
	c0, _, ok0 := lay.AccessibleLineRange(max(0, lay.AccessibleLineForOffset(selLo)-mobileA11yLineWindow))
	_, c1, ok1 := lay.AccessibleLineRange(min(lineCount-1, lay.AccessibleLineForOffset(selHi)+mobileA11yLineWindow))
	if !ok0 || !ok1 || c1 <= c0 {
		return b.String()
	}
	if c1-c0 > mobileA11yCharWindowMax {
		c0 = max(c0, selEnd-mobileA11yCharWindowMax/2)
		c1 = min(c1, c0+mobileA11yCharWindowMax)
	}
	b.WriteString("\nC:")
	b.WriteString(strconv.Itoa(ix.UTF16FromRune(c0)))
	b.WriteByte(':')
	runes := []rune(text)
	for i := c0; i < c1 && i < len(runes); i++ {
		if i > c0 {
			b.WriteByte(';')
		}
		entry := "-"
		if rects := lay.AccessibleRangeBounds(i, i+1); len(rects) > 0 {
			r := rects[0]
			entry = strconv.Itoa(toPx(origin.X+r.Pos.X)) + "," +
				strconv.Itoa(toPx(origin.Y+r.Pos.Y)) + "," +
				strconv.Itoa(toPx(r.Size.Width)) + "," +
				strconv.Itoa(toPx(r.Size.Height))
		}
		b.WriteString(entry)
		if runes[i] > 0xFFFF {
			// Both UTF-16 units of a surrogate pair carry the rune's rect.
			b.WriteByte(';')
			b.WriteString(entry)
		}
	}
	return b.String()
}

// mobileA11yTextInputRole reports whether the wire role is a text-input field
// — the roles whose nodes carry the editable document when their object
// implements core.AccessibleTexter.
func mobileA11yTextInputRole(role int) bool {
	switch role {
	case mobileRoleTextField, mobileRoleTextArea, mobileRolePassword, mobileRoleSearch:
		return true
	}
	return false
}

// a11yVisibleBounds returns the node's rectangle clipped to the region a screen
// reader may focus, and whether any of it remains. A node inside a scroll area
// (HasClip) is bounded by its clip intersected with the canvas interactive area
// (the safe viewport minus system insets / nav bar and the window head), so an
// overscan row never reports focus bounds past the viewport or under system UI
// (pre-rebrand tracker #808). Unclipped nodes — the window head, top-level content, dialogs — keep
// their full bounds (clipping the head to the interactive area, which excludes
// it, would make the back button unfocusable).
func (w *window) a11yVisibleBounds(n *accessibility.Node) (geo.Rectangle, bool) {
	rect := geo.Rectangle{Pos: n.Pos, Size: n.Size}
	if !n.HasClip {
		return rect, rect.Size.Width > 0 && rect.Size.Height > 0
	}
	ipos, isize := w.canvas.InteractiveArea()
	visible := a11yIntersectRects(n.Clip, geo.Rectangle{Pos: ipos, Size: isize})
	clipped := a11yIntersectRects(rect, visible)
	return clipped, clipped.Size.Width > 0 && clipped.Size.Height > 0
}

// a11yIntersectRects returns the overlapping rectangle of a and b, or a
// zero-size rectangle when they are disjoint.
func a11yIntersectRects(a, b geo.Rectangle) geo.Rectangle {
	x1 := max(a.Pos.X, b.Pos.X)
	y1 := max(a.Pos.Y, b.Pos.Y)
	x2 := min(a.Pos.X+a.Size.Width, b.Pos.X+b.Size.Width)
	y2 := min(a.Pos.Y+a.Size.Height, b.Pos.Y+b.Size.Height)
	if x2 <= x1 || y2 <= y1 {
		return geo.Rectangle{}
	}
	return geo.Rectangle{Pos: geo.NewPos(x1, y1), Size: geo.NewSize(x2-x1, y2-y1)}
}

// mobileA11yRole maps the semantic role onto the wire vocabulary. Container
// roles (group, list, table, dialog, toolbar, ...) return ok=false: mobile
// screen readers navigate a flat sequence, so containers contribute no
// element of their own and their children surface at the top level — the
// walker's reading order keeps them contiguous. Text-ish leaves with nothing
// to say (unlabeled images, empty labels) are dropped as decorative.
func mobileA11yRole(n *accessibility.Node) (int, bool) {
	empty := n.Label == "" && n.Value == ""
	switch n.Role {
	case core.RoleButton:
		return mobileRoleButton, true
	case core.RoleLabel, core.RoleStatus, core.RoleTooltip, core.RoleBadge:
		return mobileRoleText, !empty
	case core.RoleHeading:
		return mobileRoleHeading, !empty
	case core.RoleLink:
		return mobileRoleLink, true
	case core.RoleImage:
		return mobileRoleImage, !empty
	case core.RoleCheckbox:
		return mobileRoleCheckbox, true
	case core.RoleSwitch:
		return mobileRoleSwitch, true
	case core.RoleRadioButton:
		return mobileRoleRadio, true
	case core.RoleTab:
		return mobileRoleTab, true
	case core.RoleTextBox:
		return mobileRoleTextField, true
	case core.RoleMultiLineTextBox:
		return mobileRoleTextArea, true
	case core.RolePasswordField:
		return mobileRolePassword, true
	case core.RoleSearchField:
		return mobileRoleSearch, true
	case core.RoleComboBox:
		return mobileRoleCombo, true
	case core.RoleSlider:
		return mobileRoleSlider, true
	case core.RoleProgressBar:
		return mobileRoleProgress, true
	case core.RoleSpinButton:
		return mobileRoleSpin, true
	case core.RoleListItem:
		return mobileRoleListItem, true
	case core.RoleMenuItem:
		return mobileRoleMenuItem, true
	case core.RoleTreeItem:
		return mobileRoleTreeItem, true
	case core.RoleTableCell:
		return mobileRoleCell, true
	case core.RoleTableRow:
		// A labeled row is a real reading stop — a grid group header
		// ("West (125)" with expand/collapse actions, pre-rebrand tracker #951) — while an
		// unlabeled row is just a container and stays dropped like the other
		// container roles. Heading, not plain text: TalkBack/VoiceOver then
		// announce it as a heading and offer heading navigation, which is
		// what distinguishes a section header from the data rows around it.
		return mobileRoleHeading, !empty
	}
	return 0, false
}

// mobileA11ySortGridChildren reorders each virtualized grid's member nodes
// into reading order — by row band top-to-bottom, then across the row — in
// place. The tree walk emits a grid's regions block by block (a pinned-column
// widget subtree in full, then the main cells subtree), so without this a
// linear swipe walks the whole pinned column before crossing into the row's
// remaining cells, and lateral navigation out of a pinned cell falls off the
// collection (pre-rebrand tracker #951). Within one row band two grid items order by their display
// column coordinate (visual/current-view space, so RTL presentations keep
// their reading order); a non-item row node (a group header spanning the band
// alone) falls back to the mirrored x bounds.
func mobileA11ySortGridChildren(nodes []mobileA11yNode) {
	byGrid := make(map[uintptr][]int)
	for i, n := range nodes {
		if n.gridHandle != 0 && n.role != mobileRoleGrid {
			byGrid[n.gridHandle] = append(byGrid[n.gridHandle], i)
		}
	}
	for _, idx := range byGrid {
		run := make([]mobileA11yNode, len(idx))
		for j, i := range idx {
			run[j] = nodes[i]
		}
		slices.SortStableFunc(run, func(a, b mobileA11yNode) int {
			if a.y != b.y {
				return a.y - b.y
			}
			if a.cellRow >= 0 && b.cellRow >= 0 && a.cellRow == b.cellRow {
				return a.cellCol - b.cellCol
			}
			return a.x - b.x
		})
		for j, i := range idx {
			nodes[i] = run[j]
		}
	}
}

func mobileA11yFlags(n *accessibility.Node) int {
	flags := 0
	if n.State.Has(core.StateDisabled) {
		flags |= mobileA11yFlagDisabled
	}
	if n.State.Has(core.StateSelected) {
		flags |= mobileA11yFlagSelected
	}
	switch n.Role {
	case core.RoleCheckbox, core.RoleSwitch, core.RoleRadioButton:
		flags |= mobileA11yFlagCheckable
		if n.State.Has(core.StateChecked) || n.State.Has(core.StateSelected) {
			flags |= mobileA11yFlagChecked
		}
	}
	if mobileA11yPressAction(n) != "" || a11yIsTappable(n.Object) {
		flags |= mobileA11yFlagPressable
	}
	if mobileA11yHasAction(n, core.ActionIncrement) || mobileA11yHasAction(n, core.ActionDecrement) {
		flags |= mobileA11yFlagAdjustable
	}
	if mobileA11yHasAction(n, core.ActionDismiss) {
		flags |= mobileA11yFlagDismissable
	}
	// Expose the AT scroll gesture (pre-rebrand tracker #762). A screen reader (TalkBack, VoiceOver)
	// scrolls a scrollable *container*, not a leaf inside it — so the node that
	// IS the scroller (its object implements event.Scrollable: a Scroll, List,
	// grid, …) must carry the flag, else the two-finger scroll gesture finds no
	// target and is ignored. Descendants inside a scroll are marked too so the
	// gesture still routes when the focus sits on one. Adjustable nodes (sliders)
	// keep their own increment/decrement gesture and are not marked scrollable.
	if flags&mobileA11yFlagAdjustable == 0 {
		if _, isScroller := n.Object.(event.Scrollable); isScroller || n.ScrollAncestor != nil {
			flags |= mobileA11yFlagScrollable
		}
	}
	if a11yIsLongPressable(n.Object) {
		flags |= mobileA11yFlagLongPressable
	}
	return flags
}

// a11yIsLongPressable reports whether obj responds to a long-press: either the
// touch-native LongPressable or the desktop-parity SecondaryTappable, matching
// the touch handler's own fallback order (internal/driver/common/touch.go).
func a11yIsLongPressable(obj core.CanvasObject) bool {
	if _, ok := obj.(event.LongPressable); ok {
		return true
	}
	_, ok := obj.(event.SecondaryTappable)
	return ok
}

// mobileA11yPressAction picks the accessible action a double-tap activation
// maps to, or "" when the node declares none that fits a press. Mirrors the
// desktop backends' a11yPressAction.
func mobileA11yPressAction(n *accessibility.Node) string {
	for _, act := range n.Actions {
		switch act.Name {
		case core.ActionPress, core.ActionToggle, core.ActionSelect, core.ActionDismiss:
			return act.Name
		}
	}
	return ""
}

// mobileA11yCustomActions returns the node's declared actions that no native
// mobile semantic consumes, in declaration order, as (registry id, label)
// wire pairs (pre-rebrand tracker #1025): the press-mapped action, the increment/decrement pair
// (the adjustable gesture) and dismiss (the scrub/back gesture) are excluded;
// everything else — undo, redo, expand, collapse, rename, delete, and any
// future names — is offered as a platform custom action. The mobile mirror of
// the darwin backend's a11yCustomActions (pre-rebrand tracker #501). Caller must hold a11yMu (the
// registry is shared with the dispatch path).
func mobileA11yCustomActions(n *accessibility.Node) []mobileA11yCustomAct {
	press := mobileA11yPressAction(n)
	var out []mobileA11yCustomAct
	for _, act := range n.Actions {
		switch act.Name {
		case press, core.ActionIncrement, core.ActionDecrement, core.ActionDismiss:
			continue
		}
		id, ok := a11yCustomActionIDs[act.Name]
		if !ok {
			id = len(a11yCustomActionNames)
			a11yCustomActionIDs[act.Name] = id
			a11yCustomActionNames = append(a11yCustomActionNames, act.Name)
		}
		label := act.Description
		if label == "" {
			label = act.Name
		}
		out = append(out, mobileA11yCustomAct{id: id, label: label})
	}
	return out
}

// mobileA11yNodesEqual reports whether two wire nodes are identical — the
// per-frame diff the iOS bridge elides unchanged ObjC dispatch with. A plain
// == stopped compiling when the node gained the customActs slice (pre-rebrand tracker #1036).
// KEEP IN SYNC with mobileA11yNode: a field added there must be compared
// here, or the iOS bridge silently stops propagating its changes.
func mobileA11yNodesEqual(a, b mobileA11yNode) bool {
	return a.handle == b.handle &&
		a.role == b.role && a.flags == b.flags &&
		a.label == b.label && a.value == b.value && a.hint == b.hint &&
		a.x == b.x && a.y == b.y && a.w == b.w && a.h == b.h &&
		a.gridHandle == b.gridHandle &&
		a.cellRow == b.cellRow && a.cellCol == b.cellCol &&
		a.rowCount == b.rowCount && a.colCount == b.colCount &&
		a.text == b.text && a.selStart == b.selStart && a.selEnd == b.selEnd &&
		a.textLayout == b.textLayout &&
		slices.Equal(a.customActs, b.customActs)
}

// a11yEncodeCustomActs flattens the (registry id, label) custom-action pairs
// (pre-rebrand tracker #1025) into the single-string wire form the platform bridges parse
// (GoNativeActivity.java and accessibility_ios.m): entries separated by
// U+001E (record separator), id from label by U+001F (unit separator) —
// labels are arbitrary human text, so no printable delimiter is safe. Empty
// when the node declares no custom actions. Lives in the platform-neutral
// slice so the encoding is host-testable.
func a11yEncodeCustomActs(acts []mobileA11yCustomAct) string {
	if len(acts) == 0 {
		return ""
	}
	var b strings.Builder
	for i, a := range acts {
		if i > 0 {
			b.WriteByte(0x1e)
		}
		b.WriteString(strconv.Itoa(a.id))
		b.WriteByte(0x1f)
		b.WriteString(a.label)
	}
	return b.String()
}

// mobileA11yCustomActionName resolves a custom-action registry id back to its
// action name; callable from any thread.
func mobileA11yCustomActionName(id int) (string, bool) {
	a11yMu.Lock()
	defer a11yMu.Unlock()
	if id < 0 || id >= len(a11yCustomActionNames) {
		return "", false
	}
	return a11yCustomActionNames[id], true
}

func mobileA11yHasAction(n *accessibility.Node, name string) bool {
	for _, act := range n.Actions {
		if act.Name == name {
			return true
		}
	}
	return false
}

func a11yIsTappable(obj core.CanvasObject) bool {
	_, ok := obj.(event.Tappable)
	return ok
}

func a11yRectsOverlap(a, b geo.Rectangle) bool {
	return a.Pos.X < b.Pos.X+b.Size.Width && b.Pos.X < a.Pos.X+a.Size.Width &&
		a.Pos.Y < b.Pos.Y+b.Size.Height && b.Pos.Y < a.Pos.Y+a.Size.Height
}

// mobileA11yLookup resolves a handle to its dispatch target; callable from
// any thread.
func mobileA11yLookup(handle uintptr) (a11yTarget, bool) {
	a11yMu.Lock()
	defer a11yMu.Unlock()
	t, ok := a11yTargets[handle]
	return t, ok
}

// mobileA11yForgetWindow drops all registry state for a closing window.
func mobileA11yForgetWindow(w *window) {
	a11yMu.Lock()
	defer a11yMu.Unlock()
	st := a11yWindows[w]
	if st == nil {
		return
	}
	for _, h := range st.handles {
		delete(a11yTargets, h)
	}
	for _, h := range st.virtual {
		delete(a11yTargets, h)
	}
	for _, h := range st.realized {
		delete(a11yTargets, h)
	}
	delete(a11yWindows, w)
}

// mobileA11yQueue runs fn on the main goroutine, the only context widget
// code may be called from. The platform callbacks arrive on OS threads, so
// every dispatch below is queued — the mobile mirror of the desktop
// backends' direct calls (which already sit on the main thread).
func mobileA11yQueue(fn func()) {
	app := bellui.CurrentApp()
	if app == nil {
		return
	}
	d, ok := app.Driver().(*driver)
	if !ok {
		return
	}
	d.DoFromGoroutine(fn, false)
}

// mobileA11yDispatch performs a bridge action against a handle on the main
// goroutine. Activate mirrors the desktop press dispatch: the node's press
// action when it declares one, else a synthesized Tapped at the element's
// center.
func mobileA11yDispatch(handle uintptr, action int) {
	mobileA11yQueue(func() {
		mobileA11yDispatchOnMain(handle, action)
	})
}

// mobileA11yDispatchOnMain is the dispatch body; main goroutine only.
func mobileA11yDispatchOnMain(handle uintptr, action int) {
	t, ok := mobileA11yLookup(handle)
	if !ok {
		return
	}
	n := t.node
	// A custom action (pre-rebrand tracker #1025): resolve the registry id back to its name and
	// perform it — mobileA11yPerform re-validates against the node's current
	// declaration, so a stale menu pick (the action list changed while the
	// AT's menu was open) is a no-op rather than a misfire.
	if action >= mobileA11yActionCustom {
		if name, ok := mobileA11yCustomActionName(action - mobileA11yActionCustom); ok {
			mobileA11yPerform(n, name)
		}
		return
	}
	switch action {
	case mobileA11yActionActivate:
		// An on-demand realized cell has no widget to activate; pressing it
		// (a double-tap on an off-viewport cell) brings it into view
		// instead, after which the real host handles selection (pre-rebrand tracker #817, the
		// mobile mirror of the desktop bridges' press-on-realized path).
		if n.Object == nil && t.grid != 0 && n.HasCellPosition {
			if gt, ok := mobileA11yLookup(t.grid); ok {
				accessibility.ScrollCellIntoView(gt.node, n.CellRow, n.CellCol)
			}
			return
		}
		// Focus handling is the touch pipeline's job, not the widget's:
		// Entry.Tapped only adjusts selection state. Mirror the pipeline —
		// activating a keyboard-consuming widget focuses it (raising the
		// soft keyboard); activating anything else unfocuses the current
		// focus first, as a real tap would (otherwise a focused entry
		// keeps grabbing the keyboard back after unrelated activations).
		_, wantsKeyboard := n.Object.(mobile.Keyboardable)
		if f, isFocusable := n.Object.(event.Focusable); isFocusable && wantsKeyboard {
			t.win.canvas.Focus(f)
		} else if cur := t.win.canvas.Focused(); cur != nil {
			if obj, isObj := cur.(core.CanvasObject); !isObj || obj != n.Object {
				t.win.canvas.Unfocus()
			}
		}
		if name := mobileA11yPressAction(n); name != "" {
			if actioner, ok := n.Object.(core.AccessibleActioner); ok {
				actioner.PerformAccessibleAction(name)
				core.NotifyActivation(n.Object, false)
				return
			}
		}
		if tap, ok := n.Object.(event.Tappable); ok {
			center := geo.NewPos(n.Size.Width/2, n.Size.Height/2)
			tap.Tapped(&event.PointEvent{
				AbsolutePosition: geo.NewPos(n.Pos.X+center.X, n.Pos.Y+center.Y),
				Position:         center,
			})
			core.NotifyActivation(n.Object, false)
		}
	case mobileA11yActionIncrement:
		mobileA11yPerform(n, core.ActionIncrement)
	case mobileA11yActionDecrement:
		mobileA11yPerform(n, core.ActionDecrement)
	case mobileA11yActionDismiss:
		mobileA11yPerform(n, core.ActionDismiss)
	case mobileA11yActionScrollForward:
		mobileA11yScroll(n, true)
	case mobileA11yActionScrollBackward:
		mobileA11yScroll(n, false)
	case mobileA11yActionLongPress:
		mobileA11yLongPress(n)
	}
}

// mobileA11yTextDispatch performs a bridge text-edit action against a handle
// on the main goroutine (pre-rebrand tracker #974 option D): the platform side resolves granularity
// segments and code-unit offsets itself and sends rune offsets here, where the
// edit lands on the editor's own gated, undo-recorded paths via the
// internal/textinput primitives — the same seam the macOS and Windows text
// protocols consume. The next gather republishes the node's text + selection
// (the wire node, not the severed shadow EditText — pre-rebrand tracker #1006 Phase 3), so the
// platform's optimistically-applied state reconciles against widget truth.
func mobileA11yTextDispatch(handle uintptr, kind, a, b int, text string) {
	mobileA11yQueue(func() {
		mobileA11yTextDispatchOnMain(handle, kind, a, b, text)
	})
}

// mobileA11yTextDispatchOnMain is the text-edit dispatch body; main goroutine
// only.
func mobileA11yTextDispatchOnMain(handle uintptr, kind, a, b int, text string) {
	t, ok := mobileA11yLookup(handle)
	if !ok || t.node.Object == nil {
		return
	}
	ed, ok := t.node.Object.(core.AccessibleTexter)
	if !ok {
		return
	}
	switch kind {
	case mobileA11yTextActSetSelection:
		textinput.Select(ed, a, b)
	case mobileA11yTextActReplace:
		textinput.ReplaceRange(ed, a, b, text)
	case mobileA11yTextActSetText:
		textinput.ReplaceAll(ed, text)
	case mobileA11yTextActInsert:
		textinput.InsertText(ed, text)
	}
}

// mobileA11yScroll pages the node's nearest scroller by ~a screenful in
// response to a screen reader's scroll gesture (pre-rebrand tracker #762). Forward reveals content
// further along (down / trailing); the scroller's own axis handling maps a
// vertical delta to a horizontal-only scroller.
func mobileA11yScroll(n *accessibility.Node, forward bool) {
	// The scroll target is the node's own object when it IS the scroller (a
	// container marked scrollable for the AT), else its nearest scroll ancestor —
	// the container's own ScrollAncestor is the scroll ABOVE it (usually nil), so
	// routing only through ScrollAncestor would never scroll the container itself.
	target := n.Object
	sc, ok := target.(event.Scrollable)
	if !ok {
		target = n.ScrollAncestor
		sc, ok = target.(event.Scrollable)
	}
	if !ok {
		return
	}
	// One page = a fraction of the scroller's viewport. A negative DY increases
	// the scroll offset (reveals content below), so forward is negative.
	page := target.Size().Height * a11yScrollPageFraction
	if page <= 0 {
		page = target.Size().Width * a11yScrollPageFraction
	}
	dy := -page
	if !forward {
		dy = page
	}
	sc.Scrolled(&event.ScrollEvent{
		Scrolled: geo.NewDelta(0, dy),
		// Deliberately not ScrollSourceTouch: a screen-reader page is a
		// logical "next screenful" request, not a finger tracking content,
		// and a direct-touch event skips the scroller's pointer conventions
		// (#291) — including the vertical→horizontal mapping this relies on.
		Source: event.ScrollSourceUnknown,
	})
}

// mobileA11yLongPress routes a screen reader long-press to the node's own
// long-press behaviour, preferring the touch-native LongPressable and falling
// back to SecondaryTappable — the same order the touch handler uses.
func mobileA11yLongPress(n *accessibility.Node) {
	center := geo.NewPos(n.Size.Width/2, n.Size.Height/2)
	ev := &event.PointEvent{
		AbsolutePosition: geo.NewPos(n.Pos.X+center.X, n.Pos.Y+center.Y),
		Position:         center,
	}
	if lp, ok := n.Object.(event.LongPressable); ok {
		lp.LongPressed(ev)
		return
	}
	if st, ok := n.Object.(event.SecondaryTappable); ok {
		st.TappedSecondary(ev)
	}
}

func mobileA11yPerform(n *accessibility.Node, name string) {
	if !mobileA11yHasAction(n, name) {
		return
	}
	if actioner, ok := n.Object.(core.AccessibleActioner); ok {
		actioner.PerformAccessibleAction(name)
	}
}

// a11yFocusedHandle is the wire handle of the element the screen reader has
// accessibility focus on, or 0 when none. Set from the platform focus callback
// (Android TalkBack a11y-focus event; iOS is a follow-up) on an OS thread and
// read on the main goroutine at paint time, so it is atomic. It drives the
// in-scene focus ring (pre-rebrand tracker #759): TalkBack's own green highlight cannot reach
// Bellui's flat GPU (WebGPU/Vulkan) surface — the a11y layer is a parallel tree
// of invisible zero-pixel proxy Views — so Bellui paints its own ring over the
// focused element instead, the way Flutter does under NativeActivity.
var a11yFocusedHandle atomic.Uintptr

// mobileA11ySetFocus records (or clears) the accessibility-focused element and
// requests a repaint so the in-scene focus ring appears, follows or clears.
// Called on an OS accessibility thread; the repaint is queued to the main
// goroutine.
func mobileA11ySetFocus(handle uintptr, focused bool) {
	if focused {
		a11yFocusedHandle.Store(handle)
	} else if a11yFocusedHandle.Load() == handle {
		// Only clear if this handle is still the focused one — a focus-cleared
		// for the old element can arrive after focus already moved on.
		a11yFocusedHandle.Store(0)
	}
	mobileA11yQueue(func() {
		app := bellui.CurrentApp()
		if app == nil {
			return
		}
		if d, ok := app.Driver().(*driver); ok {
			if w := d.currentWindow(); w != nil {
				w.canvas.SetDirty()
			}
		}
	})
}

// a11yFocusRing returns the canvas-space bounds (device-independent units) of
// the element the screen reader currently has accessibility focus on within
// this window, for the in-scene focus ring (pre-rebrand tracker #759). ok is false when nothing is
// focused, the focus is in another window, or the focused element has left the
// tree since the last gather.
func (w *window) a11yFocusRing() (geo.Rectangle, bool) {
	h := a11yFocusedHandle.Load()
	if h == 0 {
		return geo.Rectangle{}, false
	}
	t, ok := mobileA11yLookup(h)
	if !ok || t.win != w || t.node == nil {
		return geo.Rectangle{}, false
	}
	// Clip the ring to the usable viewport so it never draws over the system nav
	// bar for an overscan row (pre-rebrand tracker #808).
	return w.a11yVisibleBounds(t.node)
}

// mobileA11yRefreshContent forces a repaint whose paint hook rebuilds a tree
// the backend was skipping while no assistive technology was running. Called
// when the platform reports the screen reader was switched on mid-session.
func mobileA11yRefreshContent() {
	mobileA11yQueue(func() {
		belluiApp := bellui.CurrentApp()
		if belluiApp == nil {
			return
		}
		d, ok := belluiApp.Driver().(*driver)
		if !ok {
			return
		}
		if w := d.currentWindow(); w != nil {
			if content := w.canvas.Content(); content != nil {
				content.Refresh()
			}
		}
	})
}
