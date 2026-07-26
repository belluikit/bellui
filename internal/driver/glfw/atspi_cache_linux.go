//go:build accessibility && linux && !android && !wasm && !js

package glfw

import "github.com/godbus/dbus/v5"

// P2 of the Linux AT-SPI2 backend (#987): the org.a11y.atspi.Cache fast path.
// Orca prefers a single Cache.GetItems round-trip over walking the tree
// property-by-property (the P0/P1 per-node reads still work as the fallback the
// spike proved). AddAccessible / RemoveAccessible keep Orca's cache coherent as
// the tree grows and shrinks; RemoveAccessible on teardown is mandatory or
// libatspi leaks the item (GNOME bug 650090).

// cacheItem is one entry of Cache.GetItems / AddAccessible. godbus marshals it
// as the AT-SPI CACHE_ITEM struct ((so)(so)(so)iiassusau): reference,
// application, parent, index-in-parent, child-count, interfaces, name, role,
// description, states.
type cacheItem struct {
	Ref           objectRef
	App           objectRef
	Parent        objectRef
	IndexInParent int32
	ChildCount    int32
	Interfaces    []string
	Name          string
	Role          uint32
	Description   string
	States        []uint32
}

// atspiCache is the object exported at atspiCachePath implementing
// org.a11y.atspi.Cache.
type atspiCache struct{ p *atspiProvider }

// GetItems returns a snapshot of every exported accessible as one array, so a
// client populates its whole cache in a single call.
func (c *atspiCache) GetItems() ([]cacheItem, *dbus.Error) {
	p := c.p
	p.mu.Lock()
	defer p.mu.Unlock()
	items := make([]cacheItem, 0, len(p.nodes))
	for _, n := range p.nodes {
		items = append(items, p.cacheItemLocked(n))
	}
	return items, nil
}

// cacheItemLocked builds the cache item for a node. Caller holds p.mu.
func (p *atspiProvider) cacheItemLocked(n *atspiNode) cacheItem {
	idx := int32(-1)
	if n.parent.Name == p.busName {
		if parent := p.nodes[n.parent.Path]; parent != nil {
			for i, ch := range parent.children {
				if ch == n.path {
					idx = int32(i)
					break
				}
			}
		}
	}
	return cacheItem{
		Ref:           p.ref(n.path),
		App:           p.ref(atspiRootPath),
		Parent:        n.parent,
		IndexInParent: idx,
		ChildCount:    int32(len(n.children)),
		Interfaces:    append([]string(nil), n.ifaces...),
		Name:          n.name,
		Role:          n.role,
		Description:   n.desc,
		States:        []uint32{n.states[0], n.states[1]},
	}
}

// emitCacheAdd emits Cache.AddAccessible for a freshly-populated node. Caller
// must NOT hold p.mu (Emit can block); pass a prebuilt item.
func (p *atspiProvider) emitCacheAdd(item cacheItem) {
	_ = p.conn.Emit(atspiCachePath, ifaceCache+".AddAccessible", item)
}

// emitCacheRemove emits Cache.RemoveAccessible for a torn-down node. Caller must
// NOT hold p.mu. Emitting this on every removal is mandatory (GNOME bug 650090).
func (p *atspiProvider) emitCacheRemove(ref objectRef) {
	_ = p.conn.Emit(atspiCachePath, ifaceCache+".RemoveAccessible", ref)
}
