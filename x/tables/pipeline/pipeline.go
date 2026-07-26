package pipeline

import "sync"

// Pipeline chains transforms and maintains a RowIndex mapping display rows to source rows.
type Pipeline struct {
	mu         sync.RWMutex
	source     DataSource
	transforms []Transform
	index      *RowIndex
	dirty      bool
}

// New creates a new Pipeline with the given data source.
func New(source DataSource) *Pipeline {
	p := &Pipeline{
		source: source,
		dirty:  true,
	}
	return p
}

// SetSource replaces the data source and marks the pipeline dirty.
func (p *Pipeline) SetSource(source DataSource) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.source = source
	p.dirty = true
}

// AddTransform appends a transform to the pipeline.
func (p *Pipeline) AddTransform(t Transform) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.transforms = append(p.transforms, t)
	p.dirty = true
}

// SetTransforms replaces all transforms.
func (p *Pipeline) SetTransforms(transforms []Transform) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.transforms = transforms
	p.dirty = true
}

// ClearTransforms removes all transforms.
func (p *Pipeline) ClearTransforms() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.transforms = nil
	p.dirty = true
}

// MarkDirty forces recomputation on the next access.
func (p *Pipeline) MarkDirty() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.dirty = true
}

// Recompute runs all transforms and rebuilds the RowIndex.
func (p *Pipeline) Recompute() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.recompute()
}

func (p *Pipeline) recompute() {
	n := p.source.Length()
	indices := make([]int, n)
	for i := range indices {
		indices[i] = i
	}

	for _, t := range p.transforms {
		indices = t.Apply(indices, p.source)
	}

	p.index = NewRowIndex(indices)
	p.dirty = false
}

// Index returns the current RowIndex, recomputing if dirty.
func (p *Pipeline) Index() *RowIndex {
	p.mu.RLock()
	if !p.dirty && p.index != nil {
		idx := p.index
		p.mu.RUnlock()
		return idx
	}
	p.mu.RUnlock()

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.dirty || p.index == nil {
		p.recompute()
	}
	return p.index
}

// DisplayLength returns the number of rows after all transforms.
func (p *Pipeline) DisplayLength() int {
	return p.Index().Length()
}

// DisplayRow returns the source data for the given display row.
func (p *Pipeline) DisplayRow(displayIndex int) any {
	idx := p.Index()
	sourceRow := idx.SourceRow(displayIndex)
	if sourceRow < 0 {
		return nil
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.source.Row(sourceRow)
}

// Source returns the underlying data source.
func (p *Pipeline) Source() DataSource {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.source
}
