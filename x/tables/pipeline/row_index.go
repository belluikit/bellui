package pipeline

import "sync"

// RowIndex maps display row indices to source row indices.
// This is the output of the pipeline, used by the rendering engine
// to translate visible row positions back to actual data.
//
// A RowIndex is immutable once built: the pipeline mints a fresh one on every
// recompute (see Pipeline.recompute), so the lazily-built reverse map below is
// invalidated automatically — a new view means a new RowIndex with an unbuilt
// reverse map.
type RowIndex struct {
	indices []int

	// reverse is the lazily-built source->display map, guarded by reverseOnce.
	// It stays nil until the first DisplayRow call, so forward-only rendering
	// (SourceRow/Indices) never allocates it.
	reverseOnce sync.Once
	reverse     map[int]int
}

// NewRowIndex creates a RowIndex from a slice of source indices.
func NewRowIndex(indices []int) *RowIndex {
	return &RowIndex{indices: indices}
}

// NewIdentityIndex creates an identity mapping for n rows (0, 1, 2, ..., n-1).
func NewIdentityIndex(n int) *RowIndex {
	indices := make([]int, n)
	for i := range indices {
		indices[i] = i
	}
	return &RowIndex{indices: indices}
}

// Length returns the number of display rows.
func (r *RowIndex) Length() int {
	return len(r.indices)
}

// SourceRow returns the source row index for the given display row index.
func (r *RowIndex) SourceRow(displayRow int) int {
	if displayRow < 0 || displayRow >= len(r.indices) {
		return -1
	}
	return r.indices[displayRow]
}

// DisplayRow returns the display row currently showing the given source row, or
// -1 if that source row is not in the view (filtered out / on another page).
//
// The reverse (source->display) map is built lazily on first call and cached for
// the life of this index, so grids that never do a reverse lookup pay nothing
// and the cost is amortized across the many lookups a copy/remap performs. O(1)
// after the first call; the first call is O(n) to build the map.
func (r *RowIndex) DisplayRow(sourceRow int) int {
	if sourceRow < 0 {
		return -1
	}
	r.reverseOnce.Do(func() {
		reverse := make(map[int]int, len(r.indices))
		for d, s := range r.indices {
			if s < 0 {
				continue // synthetic rows (e.g. group headers) have no source identity
			}
			reverse[s] = d
		}
		r.reverse = reverse
	})
	if d, ok := r.reverse[sourceRow]; ok {
		return d
	}
	return -1
}

// Indices returns the raw index slice.
func (r *RowIndex) Indices() []int {
	return r.indices
}
