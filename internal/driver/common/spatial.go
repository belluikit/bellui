package common

import (
	"sync/atomic"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/driver"
)

const defaultCellSize float32 = 64

// spatialEntry stores broadphase + narrowphase data for one CanvasObject.
// Entries within a grid cell are ordered by DFS insertion order so that
// iterating and keeping the last match reproduces "deepest match wins."
type spatialEntry struct {
	obj  core.CanvasObject
	wt   geo.Matrix // world transform (for inverse-transform narrowphase)
	w, h float32    // local size (obj.Size())

	// screenBB: axis-aligned bounding box in screen coordinates.
	screenMinX, screenMinY, screenMaxX, screenMaxY float32
	// clipBB: effective clip region in screen coordinates.
	clipMinX, clipMinY, clipMaxX, clipMaxY float32
}

// SpatialIndex provides O(1) broadphase hit testing via a uniform grid.
//
// The canvas is divided into cells of fixed size. Each cell contains a list
// of [spatialEntry] values whose screen-space AABB overlaps that cell.
// Entries are inserted in DFS order during the tree walk so that within
// each cell the last matching entry is the deepest (innermost) object —
// matching the existing "last match wins" semantics.
//
// The index is immutable once built and safe for concurrent reads.
type SpatialIndex struct {
	cellW, cellH float32
	cols, rows   int
	cells        [][]spatialEntry
}

// spatialIndexBuilder collects entries during a tree walk and finalises them
// into a [SpatialIndex].
type spatialIndexBuilder struct {
	cellW, cellH float32
	cols, rows   int
	cells        [][]spatialEntry
}

func newSpatialIndexBuilder(canvasW, canvasH float32) *spatialIndexBuilder {
	cellW := defaultCellSize
	cellH := defaultCellSize
	cols := int(canvasW/cellW) + 1
	rows := int(canvasH/cellH) + 1
	if cols < 1 {
		cols = 1
	}
	if rows < 1 {
		rows = 1
	}
	return &spatialIndexBuilder{
		cellW: cellW, cellH: cellH,
		cols: cols, rows: rows,
		cells: make([][]spatialEntry, cols*rows),
	}
}

// add inserts an object into the grid. The world transform, local size, and
// effective clip bounds are used to compute the screen-space AABB and
// determine which cells the object overlaps.
func (b *spatialIndexBuilder) add(
	obj core.CanvasObject,
	wt geo.Matrix,
	w, h float32,
	clipPos geo.Position, clipSize geo.Size,
) {
	if w <= 0 || h <= 0 {
		return
	}

	// Compute screen-space AABB using the world transform.
	sMinX, sMinY, sMaxX, sMaxY := wt.AABB(w, h)

	// Effective clip bounds.
	cMinX := clipPos.X
	cMinY := clipPos.Y
	cMaxX := clipPos.X + clipSize.Width
	cMaxY := clipPos.Y + clipSize.Height

	// Skip objects entirely outside their clip region.
	if sMaxX <= cMinX || sMinX >= cMaxX || sMaxY <= cMinY || sMinY >= cMaxY {
		return
	}

	entry := spatialEntry{
		obj: obj, wt: wt, w: w, h: h,
		screenMinX: sMinX, screenMinY: sMinY, screenMaxX: sMaxX, screenMaxY: sMaxY,
		clipMinX: cMinX, clipMinY: cMinY, clipMaxX: cMaxX, clipMaxY: cMaxY,
	}

	// Insert into every grid cell that the screen AABB overlaps.
	colMin := max(0, int(sMinX/b.cellW))
	colMax := min(b.cols-1, int(sMaxX/b.cellW))
	rowMin := max(0, int(sMinY/b.cellH))
	rowMax := min(b.rows-1, int(sMaxY/b.cellH))

	for r := rowMin; r <= rowMax; r++ {
		for c := colMin; c <= colMax; c++ {
			idx := r*b.cols + c
			b.cells[idx] = append(b.cells[idx], entry)
		}
	}
}

func (b *spatialIndexBuilder) build() *SpatialIndex {
	return &SpatialIndex{
		cellW: b.cellW, cellH: b.cellH,
		cols: b.cols, rows: b.rows,
		cells: b.cells,
	}
}

// FindObjectAtPositionMatching finds the deepest object at mouse whose
// screen-space AABB and clip bounds contain the point, that passes the
// narrowphase inverse-transform test, and for which matches returns true.
//
// The return values mirror [driver.FindObjectAtPositionMatching]: the matched
// object, the mouse position in object-local coordinates, and a boolean
// indicating whether a match was found.
func (idx *SpatialIndex) FindObjectAtPositionMatching(
	mouse geo.Position,
	matches func(core.CanvasObject) bool,
) (core.CanvasObject, geo.Position, bool) {
	col := int(mouse.X / idx.cellW)
	row := int(mouse.Y / idx.cellH)
	if col < 0 || col >= idx.cols || row < 0 || row >= idx.rows {
		return nil, geo.Position{}, false
	}

	cell := idx.cells[row*idx.cols+col]

	var found core.CanvasObject
	var foundPos geo.Position

	for i := range cell {
		e := &cell[i]

		if !e.obj.Visible() {
			continue
		}

		// Clip check.
		if mouse.X < e.clipMinX || mouse.Y < e.clipMinY ||
			mouse.X >= e.clipMaxX || mouse.Y >= e.clipMaxY {
			continue
		}

		// Broadphase: screen-space AABB.
		if mouse.X < e.screenMinX || mouse.Y < e.screenMinY ||
			mouse.X >= e.screenMaxX || mouse.Y >= e.screenMaxY {
			continue
		}

		// Narrowphase: inverse-transform into object-local coordinates.
		localX, localY, ok := inverseHitTest(e, mouse)
		if !ok {
			continue
		}

		if matches(e.obj) {
			found = e.obj
			foundPos = geo.NewPos(localX, localY)
		}
	}

	if found == nil {
		return nil, geo.Position{}, false
	}
	return found, foundPos, true
}

// FindAllObjectsAtPositionMatching returns all matching objects at mouse,
// ordered from deepest (innermost) to shallowest (outermost). This is used
// for scroll event propagation.
func (idx *SpatialIndex) FindAllObjectsAtPositionMatching(
	mouse geo.Position,
	matches func(core.CanvasObject) bool,
) []driver.ObjectAtPosition {
	col := int(mouse.X / idx.cellW)
	row := int(mouse.Y / idx.cellH)
	if col < 0 || col >= idx.cols || row < 0 || row >= idx.rows {
		return nil
	}

	cell := idx.cells[row*idx.cols+col]
	var results []driver.ObjectAtPosition

	for i := range cell {
		e := &cell[i]

		if !e.obj.Visible() {
			continue
		}

		if mouse.X < e.clipMinX || mouse.Y < e.clipMinY ||
			mouse.X >= e.clipMaxX || mouse.Y >= e.clipMaxY {
			continue
		}

		if mouse.X < e.screenMinX || mouse.Y < e.screenMinY ||
			mouse.X >= e.screenMaxX || mouse.Y >= e.screenMaxY {
			continue
		}

		localX, localY, ok := inverseHitTest(e, mouse)
		if !ok {
			continue
		}

		if matches(e.obj) {
			results = append(results, driver.ObjectAtPosition{
				Object:   e.obj,
				Position: geo.NewPos(localX, localY),
			})
		}
	}

	// Reverse so deepest (innermost) is first — entries are in DFS order
	// where parents precede children.
	for i, j := 0, len(results)-1; i < j; i, j = i+1, j-1 {
		results[i], results[j] = results[j], results[i]
	}
	return results
}

// inverseHitTest maps the mouse point into the entry's object-local
// coordinate system and checks whether it falls within [0, w) × [0, h).
func inverseHitTest(e *spatialEntry, mouse geo.Position) (localX, localY float32, ok bool) {
	if e.wt.B == 0 && e.wt.C == 0 {
		// Axis-aligned (translation + optional scale). Fast path.
		if e.wt.A == 0 || e.wt.D == 0 {
			return 0, 0, false // degenerate scale
		}
		localX = (mouse.X - e.wt.TX) / e.wt.A
		localY = (mouse.Y - e.wt.TY) / e.wt.D
	} else {
		// General affine transform — use full matrix inverse.
		inv, invertible := e.wt.Inverse()
		if !invertible {
			return 0, 0, false
		}
		localX, localY = inv.TransformPoint(mouse.X, mouse.Y)
	}

	if localX < 0 || localY < 0 || localX >= e.w || localY >= e.h {
		return 0, 0, false
	}
	return localX, localY, true
}

// spatialIndexHolder wraps an atomic pointer for lock-free read/write of
// the spatial index between the paint goroutine (writer) and the event
// goroutine (reader).
type spatialIndexHolder struct {
	ptr atomic.Pointer[SpatialIndex]
}

func (h *spatialIndexHolder) Store(idx *SpatialIndex) { h.ptr.Store(idx) }
func (h *spatialIndexHolder) Load() *SpatialIndex     { return h.ptr.Load() }
