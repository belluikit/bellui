package dagwidget

// Capabilities gates which user interactions a DAGCanvas allows. It controls
// interaction entry points only — keyboard shortcuts, drags, taps, palette
// insertion, and properties-panel writes. Programmatic mutation through the
// Graph or the canvas Add/Remove methods, and undo/redo of already-recorded
// commands, are never gated: a consumer that disables editing simply stops
// new commands from being created.
//
// NewDAGCanvas defaults to AllCapabilities. The zero value is fully
// read-only, which suits monitoring surfaces.
type Capabilities struct {
	// AddNodes allows inserting nodes from the palette.
	AddNodes bool
	// RemoveNodes allows deleting selected nodes.
	RemoveNodes bool
	// ConnectEdges allows creating, detaching, and deleting data-flow edges.
	ConnectEdges bool
	// GroupEdges allows drawing and deleting group dependency edges; the
	// group connector handle is hidden when off.
	GroupEdges bool
	// LinkEdges allows drawing and deleting link edges; the node connector
	// handle is hidden when off.
	LinkEdges bool
	// EditGroups allows creating and deleting groups.
	EditGroups bool
	// GroupMembership allows drag-assigning nodes into and out of groups.
	// When off, node drags never change membership and skip the boundary
	// enforcement (group growth, nudge-out) that membership implies.
	GroupMembership bool
	// Annotations allows creating and deleting annotations and editing
	// their text and anchor.
	Annotations bool
	// MoveResize allows moving and resizing nodes, groups, and annotations;
	// resize handles are hidden when off.
	MoveResize bool
	// EditProperties allows property writes from the properties panel
	// (node metadata, group label/color, link edge label).
	EditProperties bool
}

// AllCapabilities returns a Capabilities value with every interaction
// allowed — the NewDAGCanvas default.
func AllCapabilities() Capabilities {
	return Capabilities{
		AddNodes:        true,
		RemoveNodes:     true,
		ConnectEdges:    true,
		GroupEdges:      true,
		LinkEdges:       true,
		EditGroups:      true,
		GroupMembership: true,
		Annotations:     true,
		MoveResize:      true,
		EditProperties:  true,
	}
}
