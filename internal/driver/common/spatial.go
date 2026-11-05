package common

import (
	"slices"
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
	// cut is the innermost turned clip the object lies in, an index into
	// SpatialIndex.cuts, or -1.
	cut int32
}

// turnedCut is a clip whose world transform turns or skews it. The clip
// boxes the entries carry hold the box around it, so a point in that box is
// also tested against the turned rectangle itself (#1003).
type turnedCut struct {
	wt     geo.Matrix
	w, h   float32
	parent int32 // the turned clip this one lies in, or -1
}

// SpatialIndex provides O(1) broadphase hit testing via a uniform grid.
//
// The canvas is divided into cells of fixed size. Each cell lists the
// [spatialEntry] values whose screen-space AABB overlaps that cell. Entries
// are inserted in DFS order during the tree walk so that within each cell the
// last matching entry is the deepest (innermost) object — matching the
// existing "last match wins" semantics.
//
// Each entry is stored once, in entries; a cell holds int32 indices into it,
// all cells packed into one items slice (cell i is items[cellStart[i]:
// cellStart[i+1]]). Copying the entry into every cell it overlapped made a
// full-window background ~510 entry copies, and with the slices regrown from
// zero each paint the rebuild was 81% of a static frame's allocation (#435).
// The slices are reused by reset, so a canvas rebuilding every paint
// allocates only when the scene outgrows them.
//
// The index is immutable once built and safe for concurrent reads.
type SpatialIndex struct {
	cellW, cellH float32
	cols, rows   int

	entries   []spatialEntry
	cuts      []turnedCut
	cellStart []int32
	items     []int32

	// spans holds each entry's cell range between add and finish.
	spans []cellSpan
}

// cellSpan is the inclusive range of grid cells one entry overlaps.
type cellSpan struct{ colMin, colMax, rowMin, rowMax int32 }

// spatialIndexBuilder collects entries during a tree walk and finalises them
// into a [SpatialIndex].
type spatialIndexBuilder struct {
	idx *SpatialIndex
}

func newSpatialIndexBuilder(canvasW, canvasH float32) *spatialIndexBuilder {
	idx := &SpatialIndex{}
	idx.reset(canvasW, canvasH)
	return &spatialIndexBuilder{idx: idx}
}

// reset empties the index for a canvas of the given size, keeping its slices'
// capacity for the rebuild.
func (idx *SpatialIndex) reset(canvasW, canvasH float32) {
	idx.cellW, idx.cellH = defaultCellSize, defaultCellSize
	idx.cols = max(1, int(canvasW/idx.cellW)+1)
	idx.rows = max(1, int(canvasH/idx.cellH)+1)
	idx.entries = idx.entries[:0]
	idx.cuts = idx.cuts[:0]
	idx.spans = idx.spans[:0]
	idx.items = idx.items[:0]
	idx.cellStart = idx.cellStart[:0]
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
	b.idx.add(obj, wt, w, h, clipPos, clipSize, -1)
}

func (idx *SpatialIndex) add(
	obj core.CanvasObject,
	wt geo.Matrix,
	w, h float32,
	clipPos geo.Position, clipSize geo.Size,
	cut int32,
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

	// The grid cells that the screen AABB overlaps.
	colMin := max(0, int(sMinX/idx.cellW))
	colMax := min(idx.cols-1, int(sMaxX/idx.cellW))
	rowMin := max(0, int(sMinY/idx.cellH))
	rowMax := min(idx.rows-1, int(sMaxY/idx.cellH))
	if colMin > colMax || rowMin > rowMax {
		return
	}

	idx.entries = append(idx.entries, spatialEntry{
		obj: obj, wt: wt, w: w, h: h,
		screenMinX: sMinX, screenMinY: sMinY, screenMaxX: sMaxX, screenMaxY: sMaxY,
		clipMinX: cMinX, clipMinY: cMinY, clipMaxX: cMaxX, clipMaxY: cMaxY,
		cut: cut,
	})
	idx.spans = append(idx.spans, cellSpan{int32(colMin), int32(colMax), int32(rowMin), int32(rowMax)})
}

func (b *spatialIndexBuilder) build() *SpatialIndex {
	b.idx.finish()
	return b.idx
}

// finish packs the added entries' indices into the cells, in insertion (DFS)
// order within each cell: count each cell, turn the counts into start offsets,
// then place each entry's index at its cells' running write positions.
func (idx *SpatialIndex) finish() {
	n := idx.cols * idx.rows
	idx.cellStart = slices.Grow(idx.cellStart[:0], n+1)[:n+1]
	clear(idx.cellStart)
	for _, sp := range idx.spans {
		for r := sp.rowMin; r <= sp.rowMax; r++ {
			row := int(r) * idx.cols
			for c := sp.colMin; c <= sp.colMax; c++ {
				idx.cellStart[row+int(c)+1]++
			}
		}
	}
	for i := 1; i <= n; i++ {
		idx.cellStart[i] += idx.cellStart[i-1]
	}
	total := int(idx.cellStart[n])
	idx.items = slices.Grow(idx.items[:0], total)[:total]

	// Fill: cellStart[i] doubles as cell i's write position, which leaves it at
	// the start of cell i+1; shifting it back one cell restores the starts.
	for e, sp := range idx.spans {
		for r := sp.rowMin; r <= sp.rowMax; r++ {
			row := int(r) * idx.cols
			for c := sp.colMin; c <= sp.colMax; c++ {
				i := row + int(c)
				idx.items[idx.cellStart[i]] = int32(e)
				idx.cellStart[i]++
			}
		}
	}
	copy(idx.cellStart[1:], idx.cellStart[:n])
	idx.cellStart[0] = 0
	idx.spans = idx.spans[:0]
}

// cell returns the entry indices of the cell holding mouse, or nil (false)
// when mouse lies outside the grid.
func (idx *SpatialIndex) cell(mouse geo.Position) ([]int32, bool) {
	col := int(mouse.X / idx.cellW)
	row := int(mouse.Y / idx.cellH)
	if col < 0 || col >= idx.cols || row < 0 || row >= idx.rows {
		return nil, false
	}
	i := row*idx.cols + col
	return idx.items[idx.cellStart[i]:idx.cellStart[i+1]], true
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
	cell, ok := idx.cell(mouse)
	if !ok {
		return nil, geo.Position{}, false
	}

	var found core.CanvasObject
	var foundPos geo.Position

	for _, i := range cell {
		e := &idx.entries[i]

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
		if !ok || !idx.insideCuts(e.cut, mouse) {
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
	cell, ok := idx.cell(mouse)
	if !ok {
		return nil
	}

	var results []driver.ObjectAtPosition

	for _, i := range cell {
		e := &idx.entries[i]

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
		if !ok || !idx.insideCuts(e.cut, mouse) {
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
	local, ok := driver.LocalHitPoint(e.wt, mouse, e.w, e.h)
	return local.X, local.Y, ok
}

// insideCuts reports whether mouse lies inside the turned clip cut and every
// turned clip it lies in.
func (idx *SpatialIndex) insideCuts(cut int32, mouse geo.Position) bool {
	for ; cut >= 0; cut = idx.cuts[cut].parent {
		c := &idx.cuts[cut]
		if _, in := driver.LocalHitPoint(c.wt, mouse, c.w, c.h); !in {
			return false
		}
	}
	return true
}

// spatialIndexHolder wraps an atomic pointer for lock-free read/write of
// the spatial index between the paint goroutine (writer) and the event
// goroutine (reader).
type spatialIndexHolder struct {
	ptr atomic.Pointer[SpatialIndex]
}

func (h *spatialIndexHolder) Store(idx *SpatialIndex) { h.ptr.Store(idx) }
func (h *spatialIndexHolder) Load() *SpatialIndex     { return h.ptr.Load() }
