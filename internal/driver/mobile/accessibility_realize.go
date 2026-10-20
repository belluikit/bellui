//go:build accessibility && !wasm

package mobile

import (
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/internal/accessibility"
	"github.com/belluikit/bellui/internal/scale"
)

// Off-viewport virtualized-cell realize for the mobile screen-reader backends
// (pre-rebrand tracker #817, the mobile half of pre-rebrand tracker #637). A virtualized grid publishes only its
// near-viewport cells through the normal gather; when the platform side needs a
// cell beyond that window — iOS VoiceOver addressing a data-table cell through
// UIAccessibilityContainerDataTable — it is realized on demand through the
// neutral seam (internal/accessibility.RealizeCell) and delivered as a
// transient element that never routes through the culling walk. Android
// TalkBack has no point-addressed cell lookup; its analog is
// ACTION_SCROLL_TO_POSITION, which routes to mobileA11yScrollCellIntoView so
// the target scrolls into the walked window and realizes normally.

// mobileA11yRealizeTimeout bounds how long an OS accessibility thread waits for
// the main goroutine to realize a cell. The wait must be bounded: the iOS
// callback arrives on the UIKit main thread, and the main goroutine itself
// sometimes blocks on that thread (dispatch_sync in the WebGPU backend) — an
// unbounded wait could deadlock, while a timed one degrades to "this read
// failed, retry lands on the cache".
const mobileA11yRealizeTimeout = 250 * time.Millisecond

// mobileA11yRealizeCellOnMain realizes the cell at display (row, col) of the
// virtualized grid element gridHandle and registers it as a transient element.
// It returns the cell's wire node, plus the handles of any older realized cells
// evicted from the FIFO-capped cache (the platform side destroys their
// elements). Main goroutine only — it calls widget code; OS-thread callers go
// through mobileA11yRealizeCellWait.
//
// The realized cell keeps its off-viewport bounds (it carries no clip and never
// meets the pre-rebrand tracker #808 viewport clamp): it is addressed, not focused into the reading
// order, and activating it scrolls it into view rather than tapping it.
func mobileA11yRealizeCellOnMain(gridHandle uintptr, row, col int) (mobileA11yNode, []uintptr, bool) {
	t, ok := mobileA11yLookup(gridHandle)
	if !ok || t.node == nil {
		return mobileA11yNode{}, nil, false
	}

	// Widget code runs outside the registry lock, like every other dispatch.
	node, ok := accessibility.RealizeCell(t.node, row, col)
	if !ok || node.NodeID == nil {
		return mobileA11yNode{}, nil, false
	}
	role, ok := mobileA11yRole(node)
	if !ok {
		return mobileA11yNode{}, nil, false
	}

	a11yMu.Lock()
	defer a11yMu.Unlock()

	st := a11yWindows[t.win]
	if st == nil {
		return mobileA11yNode{}, nil, false
	}

	var evicted []uintptr
	handle, exists := st.virtual[node.NodeID] // already visible in the walked window
	if !exists {
		handle, exists = st.realized[node.NodeID] // repeat read of the same cell
	}
	if !exists {
		a11yNextHandle++
		handle = a11yNextHandle
		st.realized[node.NodeID] = handle
		st.realizedOrder = append(st.realizedOrder, node.NodeID)

		var evict []any
		st.realizedOrder, evict = accessibility.OverflowRealized(st.realizedOrder, accessibility.RealizedCap)
		for _, id := range evict {
			h, ok := st.realized[id]
			if !ok {
				continue // already gone by promotion
			}
			evicted = append(evicted, h)
			delete(st.realized, id)
			delete(a11yTargets, h)
		}
	}
	a11yTargets[handle] = a11yTarget{win: t.win, node: node, grid: gridHandle}

	wire := mobileA11yNode{
		handle:     handle,
		role:       role,
		flags:      mobileA11yFlags(node),
		label:      node.Label,
		value:      node.Value,
		hint:       node.Description,
		x:          scale.ToScreenCoordinate(t.win.canvas, node.Pos.X),
		y:          scale.ToScreenCoordinate(t.win.canvas, node.Pos.Y),
		w:          scale.ToScreenCoordinate(t.win.canvas, node.Size.Width),
		h:          scale.ToScreenCoordinate(t.win.canvas, node.Size.Height),
		gridHandle: gridHandle,
		cellRow:    -1,
		cellCol:    -1,
	}
	if node.HasCellPosition {
		wire.cellRow, wire.cellCol = node.CellRow, node.CellCol
	}
	return wire, evicted, true
}

// mobileA11yRealizeCellWait is the OS-thread entry for a synchronous platform
// callback (iOS accessibilityDataTableCellElementForRow:column:): it queues the
// realize onto the main goroutine and waits, bounded by
// mobileA11yRealizeTimeout. On timeout the read reports failure but the queued
// realize still completes and populates the cache, so the screen reader's next
// request for the same cell returns instantly.
func mobileA11yRealizeCellWait(gridHandle uintptr, row, col int) (mobileA11yNode, []uintptr, bool) {
	type result struct {
		node    mobileA11yNode
		evicted []uintptr
		ok      bool
	}
	ch := make(chan result, 1)

	app := bellui.CurrentApp()
	if app == nil {
		return mobileA11yNode{}, nil, false
	}
	d, ok := app.Driver().(*driver)
	if !ok {
		return mobileA11yNode{}, nil, false
	}
	d.DoFromGoroutine(func() {
		node, evicted, ok := mobileA11yRealizeCellOnMain(gridHandle, row, col)
		ch <- result{node: node, evicted: evicted, ok: ok}
	}, false)

	select {
	case r := <-ch:
		return r.node, r.evicted, r.ok
	case <-time.After(mobileA11yRealizeTimeout):
		return mobileA11yNode{}, nil, false
	}
}

// mobileA11yScrollCellIntoView routes a bring-into-view request for display
// (row, col) to the virtualized grid element gridHandle, so the cell scrolls
// into the walked window and realizes as a normal element. Callable from any
// thread; the widget call is queued to the main goroutine, like every other
// dispatched action.
func mobileA11yScrollCellIntoView(gridHandle uintptr, row, col int) {
	mobileA11yQueue(func() {
		t, ok := mobileA11yLookup(gridHandle)
		if !ok || t.node == nil {
			return
		}
		accessibility.ScrollCellIntoView(t.node, row, col)
	})
}
