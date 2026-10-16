//go:build accessibility

package accessibility

// Bookkeeping for the on-demand realized-cell caches the platform backends keep
// for off-viewport virtualized cells (pre-rebrand tracker #637): pure helpers shared by the desktop
// (darwin NSAccessibility, windows UIA) and mobile (pre-rebrand tracker #817) bridges, so the cache
// bound and FIFO discipline have one definition and are unit-testable without
// any native element lifecycle.

// RealizedCap bounds the on-demand realized-cell cache each backend keeps for
// off-viewport virtualized cells (pre-rebrand tracker #637). A screen reader reads a handful of
// cells around its cursor, so a small FIFO is ample; past this the oldest
// element is disconnected and the client re-fetches it (GridPattern.GetItem on
// Windows, accessibilityCellForColumn:row: on macOS,
// accessibilityDataTableCellElementForRow:column: on iOS) if it navigates back.
const RealizedCap = 128

// OverflowRealized splits a FIFO of realized-cell ids into the newest limit to
// keep and the oldest to evict.
func OverflowRealized(order []any, limit int) (keep, evict []any) {
	if len(order) <= limit {
		return order, nil
	}
	n := len(order) - limit
	return order[n:], order[:n]
}

// DropRealizedID removes one id from a realized-cell FIFO order, keeping it in
// step with the cache map when a cell leaves by promotion or grid teardown
// rather than eviction. It returns the (possibly shortened) slice.
func DropRealizedID(order []any, id any) []any {
	for i, o := range order {
		if o == id {
			return append(order[:i], order[i+1:]...)
		}
	}
	return order
}
