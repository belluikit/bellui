package render

import (
	"sync"

	"github.com/belluikit/bellui/core"
)

// CellPool manages recycling of cell CanvasObjects.
// Uses sync.Pool internally for efficient allocation/recycling.
type CellPool struct {
	mu       sync.Mutex
	pools    map[string]*pool
	creators map[string]func() core.CanvasObject
}

type pool struct {
	p sync.Pool
}

// NewCellPool creates a new CellPool.
func NewCellPool() *CellPool {
	return &CellPool{
		pools:    make(map[string]*pool),
		creators: make(map[string]func() core.CanvasObject),
	}
}

// RegisterType registers a cell creator function for a column type.
func (cp *CellPool) RegisterType(typeName string, creator func() core.CanvasObject) {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	cp.creators[typeName] = creator
	if _, ok := cp.pools[typeName]; !ok {
		cp.pools[typeName] = &pool{}
	}
}

// Get returns a recycled or newly created cell for the given type.
func (cp *CellPool) Get(typeName string) core.CanvasObject {
	cp.mu.Lock()
	p, ok := cp.pools[typeName]
	creator := cp.creators[typeName]
	cp.mu.Unlock()

	if ok {
		if obj, _ := p.p.Get().(core.CanvasObject); obj != nil {
			return obj
		}
	}
	if creator != nil {
		return creator()
	}
	return nil
}

// Put returns a cell to the pool for recycling.
func (cp *CellPool) Put(typeName string, obj core.CanvasObject) {
	cp.mu.Lock()
	p, ok := cp.pools[typeName]
	if !ok {
		p = &pool{}
		cp.pools[typeName] = p
	}
	cp.mu.Unlock()
	p.p.Put(obj)
}
