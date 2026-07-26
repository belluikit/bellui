//go:build accessibility && linux && !android && !wasm && !js

package glfw

import (
	"strconv"
	"sync"

	"github.com/godbus/dbus/v5"

	bellui "github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/internal/accessibility"
)

// This file implements a pure-Go AT-SPI2 accessibility provider (#981), the
// Linux counterpart of the macOS NSAccessibility (#485) and Windows UIA (#487)
// backends. It speaks the org.a11y.atspi.* D-Bus protocol directly over
// github.com/godbus/dbus/v5 — no cgo, no Rust/AccessKit — reusing the
// platform-neutral semantic tree (internal/accessibility) exactly as the other
// backends do. The route was de-risked by the hack/atspiprobe spike (#489/#982).
//
// One AT-SPI application root exists per process (the toolkit is one accessible
// application); each bellui window contributes a "frame" subtree beneath it. The
// D-Bus connection is served on godbus goroutines, so every accessible object's
// method handler and the reconcile that mutates the tree are guarded by the
// provider mutex; AT callbacks that touch bellui widgets hop to the main
// goroutine via bellui.Do.

const (
	ifaceAccessible  = "org.a11y.atspi.Accessible"
	ifaceApplication = "org.a11y.atspi.Application"
	ifaceComponent   = "org.a11y.atspi.Component"
	ifaceAction      = "org.a11y.atspi.Action"
	ifaceValue       = "org.a11y.atspi.Value"
	ifaceText        = "org.a11y.atspi.Text"
	ifaceEditText    = "org.a11y.atspi.EditableText"
	ifaceSelection   = "org.a11y.atspi.Selection"
	ifaceTable       = "org.a11y.atspi.Table"
	ifaceTableCell   = "org.a11y.atspi.TableCell"
	ifaceSocket      = "org.a11y.atspi.Socket"
	ifaceCache       = "org.a11y.atspi.Cache"
	ifaceEventObject = "org.a11y.atspi.Event.Object"
	ifaceEventWindow = "org.a11y.atspi.Event.Window"
	ifaceProperties  = "org.freedesktop.DBus.Properties"

	atspiRegistryName = "org.a11y.atspi.Registry"
	atspiRootPath     = "/org/a11y/atspi/accessible/root"
	atspiCachePath    = "/org/a11y/atspi/cache"
	atspiNullPath     = "/org/a11y/atspi/null"
)

// AT-SPI2 coordinate types (AtspiCoordType).
const (
	coordScreen = 0
	coordWindow = 1
	coordParent = 2
)

// objectRef is the AT-SPI2 (so) object reference — a (bus name, object path)
// pair naming an accessible on the a11y bus. godbus marshals it as "(so)".
type objectRef struct {
	Name string
	Path dbus.ObjectPath
}

// atspiRect is the AT-SPI2 (iiii) rectangle returned by Component.GetExtents.
type atspiRect struct{ X, Y, W, H int32 }

// atspiRelation is one entry of the a(ua(so)) relation set.
type atspiRelation struct {
	Type    uint32
	Targets []objectRef
}

// atspiNode is one accessible object exported on the a11y bus. A single Go type
// backs every object; each instance is exported at its own path under the atspi
// interfaces it advertises. Fields are filled from an internal/accessibility
// Node during reconcile and read back by D-Bus method handlers, all under
// provider.mu.
type atspiNode struct {
	p    *atspiProvider
	win  *window // owning window; nil for the application root
	path dbus.ObjectPath

	role     uint32
	roleName string
	name     string
	desc     string
	states   [2]uint32
	extents  atspiRect // window-relative device pixels
	screenX  int32     // window origin in screen px (best-effort; 0 on Wayland)
	screenY  int32
	layer    uint32
	attrs    map[string]string

	parent   objectRef // full ref: our bus name for widgets, the desktop socket for the root
	children []dbus.ObjectPath
	ifaces   []string

	// Reconcile back-references for the reverse (AT → app) path.
	obj         core.CanvasObject
	pressAction string
	focusable   bool

	// P1 snapshots, captured on the main goroutine during reconcile so D-Bus
	// reads on godbus goroutines never touch a live widget. Edits dispatch back
	// via bellui.Do; the next reconcile re-snapshots and emits change events.
	actions  []core.AccessibleAction // org.a11y.atspi.Action
	hasRange bool                    // org.a11y.atspi.Value
	valMin   float64
	valMax   float64
	valCur   float64
	valStr   string
	isText   bool // org.a11y.atspi.Text / EditableText
	editable bool
	text     *accessibility.TextIndex
	prevText string // last snapshot, for text-changed diffing
	selStart int
	selEnd   int
	caret    int

	// P3 snapshots (#988), captured like the P1 ones.
	hasSelection bool // org.a11y.atspi.Selection (item containers)
	hasGridDims  bool // org.a11y.atspi.Table (grids reporting full extent)
	gridRows     int
	gridCols     int
	gridSem      *accessibility.Node // grids: last-reconciled semantic node, for on-demand realize
	hasCell      bool                // org.a11y.atspi.TableCell
	cellRow      int
	cellCol      int
	tablePath    dbus.ObjectPath // cells: owning grid's exported path ("" when unknown)
	level        int             // 1-based tree depth (0 = not a tree row)
	relations    []atspiRelation // NODE_CHILD_OF / NODE_PARENT_OF for tree rows

	isRoot bool
}

func (p *atspiProvider) ref(path dbus.ObjectPath) objectRef {
	return objectRef{p.busName, path}
}

// --- org.a11y.atspi.Accessible -------------------------------------------

func (n *atspiNode) GetRole() (uint32, *dbus.Error) {
	n.p.mu.Lock()
	defer n.p.mu.Unlock()
	return n.role, nil
}

func (n *atspiNode) GetRoleName() (string, *dbus.Error) {
	n.p.mu.Lock()
	defer n.p.mu.Unlock()
	return n.roleName, nil
}

func (n *atspiNode) GetLocalizedRoleName() (string, *dbus.Error) {
	return n.GetRoleName()
}

func (n *atspiNode) GetState() ([]uint32, *dbus.Error) {
	n.p.mu.Lock()
	defer n.p.mu.Unlock()
	return []uint32{n.states[0], n.states[1]}, nil
}

func (n *atspiNode) GetChildCount() (int32, *dbus.Error) {
	n.p.mu.Lock()
	defer n.p.mu.Unlock()
	return int32(len(n.children)), nil
}

func (n *atspiNode) GetChildAtIndex(i int32) (objectRef, *dbus.Error) {
	n.p.mu.Lock()
	defer n.p.mu.Unlock()
	if i < 0 || int(i) >= len(n.children) {
		return n.p.ref(atspiNullPath), nil
	}
	return n.p.ref(n.children[i]), nil
}

func (n *atspiNode) GetChildren() ([]objectRef, *dbus.Error) {
	n.p.mu.Lock()
	defer n.p.mu.Unlock()
	refs := make([]objectRef, len(n.children))
	for i, c := range n.children {
		refs[i] = n.p.ref(c)
	}
	return refs, nil
}

func (n *atspiNode) GetIndexInParent() (int32, *dbus.Error) {
	n.p.mu.Lock()
	defer n.p.mu.Unlock()
	if n.parent.Name == n.p.busName {
		if parent := n.p.nodes[n.parent.Path]; parent != nil {
			for i, c := range parent.children {
				if c == n.path {
					return int32(i), nil
				}
			}
		}
	}
	return -1, nil
}

func (n *atspiNode) GetApplication() (objectRef, *dbus.Error) {
	return n.p.ref(atspiRootPath), nil
}

func (n *atspiNode) GetInterfaces() ([]string, *dbus.Error) {
	n.p.mu.Lock()
	defer n.p.mu.Unlock()
	return append([]string(nil), n.ifaces...), nil
}

func (n *atspiNode) GetAttributes() (map[string]string, *dbus.Error) {
	n.p.mu.Lock()
	defer n.p.mu.Unlock()
	out := make(map[string]string, len(n.attrs))
	for k, v := range n.attrs {
		out[k] = v
	}
	return out, nil
}

func (n *atspiNode) GetRelationSet() ([]atspiRelation, *dbus.Error) {
	n.p.mu.Lock()
	defer n.p.mu.Unlock()
	if len(n.relations) == 0 {
		// An empty a(ua(so)), not nil: godbus needs the typed slice.
		return []atspiRelation{}, nil
	}
	out := make([]atspiRelation, len(n.relations))
	for i, r := range n.relations {
		out[i] = atspiRelation{Type: r.Type, Targets: append([]objectRef(nil), r.Targets...)}
	}
	return out, nil
}

// --- org.a11y.atspi.Component --------------------------------------------

func (n *atspiNode) Contains(x, y int32, coord uint32) (bool, *dbus.Error) {
	n.p.mu.Lock()
	defer n.p.mu.Unlock()
	r := n.extentsFor(coord)
	return x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H, nil
}

func (n *atspiNode) GetAccessibleAtPoint(x, y int32, coord uint32) (objectRef, *dbus.Error) {
	// Hit-testing descendants is a later refinement; Orca does not need it to
	// follow focus. Return the null object.
	return n.p.ref(atspiNullPath), nil
}

// extentsFor returns the node's rectangle in the requested coordinate space.
// Caller holds p.mu. Under Wayland the window's screen origin is unknowable, so
// screen coordinates collapse to window-relative (a limitation shared with
// AccessKit); on X11 the origin captured during reconcile makes them correct.
func (n *atspiNode) extentsFor(coord uint32) atspiRect {
	r := n.extents
	if coord == coordScreen {
		r.X += n.screenX
		r.Y += n.screenY
	}
	return r
}

func (n *atspiNode) GetExtents(coord uint32) (atspiRect, *dbus.Error) {
	n.p.mu.Lock()
	defer n.p.mu.Unlock()
	return n.extentsFor(coord), nil
}

func (n *atspiNode) GetPosition(coord uint32) (int32, int32, *dbus.Error) {
	n.p.mu.Lock()
	defer n.p.mu.Unlock()
	r := n.extentsFor(coord)
	return r.X, r.Y, nil
}

func (n *atspiNode) GetSize() (int32, int32, *dbus.Error) {
	n.p.mu.Lock()
	defer n.p.mu.Unlock()
	return n.extents.W, n.extents.H, nil
}

func (n *atspiNode) GetLayer() (uint32, *dbus.Error) {
	n.p.mu.Lock()
	defer n.p.mu.Unlock()
	return n.layer, nil
}

func (n *atspiNode) GetMDIZOrder() (int16, *dbus.Error) { return 0, nil }

func (n *atspiNode) GetAlpha() (float64, *dbus.Error) { return 1.0, nil }

func (n *atspiNode) GrabFocus() (bool, *dbus.Error) {
	n.p.mu.Lock()
	obj := n.obj
	win := n.win
	n.p.mu.Unlock()
	if obj == nil || win == nil {
		return false, nil
	}
	if foc, ok := obj.(event.Focusable); ok {
		bellui.Do(func() { win.canvas.Focus(foc) })
		return true, nil
	}
	return false, nil
}

// --- org.a11y.atspi.Application (root only) ------------------------------

func (n *atspiNode) GetLocale(uint32) (string, *dbus.Error) { return "en_US.UTF-8", nil }

func (n *atspiNode) GetApplicationBusAddress() (string, *dbus.Error) {
	return n.p.busAddr, nil
}

// --- org.freedesktop.DBus.Properties -------------------------------------

func (n *atspiNode) propValue(iface, name string) (dbus.Variant, bool) {
	// Caller holds p.mu.
	switch iface {
	case ifaceAccessible:
		switch name {
		case "Name":
			return dbus.MakeVariant(n.name), true
		case "Description":
			return dbus.MakeVariant(n.desc), true
		case "Parent":
			return dbus.MakeVariant(n.parent), true
		case "ChildCount":
			return dbus.MakeVariant(int32(len(n.children))), true
		case "Locale":
			return dbus.MakeVariant("en_US.UTF-8"), true
		case "AccessibleId":
			return dbus.MakeVariant(string(n.path)), true
		case "HelpText":
			return dbus.MakeVariant(n.desc), true
		}
	case ifaceApplication:
		if n.isRoot {
			switch name {
			case "ToolkitName":
				return dbus.MakeVariant("Bellui"), true
			case "Version", "ToolkitVersion":
				return dbus.MakeVariant("3"), true
			case "AtspiVersion":
				return dbus.MakeVariant("2.1"), true
			case "Id":
				return dbus.MakeVariant(n.p.appID), true
			}
		}
	case ifaceText:
		if n.isText {
			switch name {
			case "CharacterCount":
				return dbus.MakeVariant(int32(n.text.RuneLen())), true
			case "CaretOffset":
				return dbus.MakeVariant(int32(n.caret)), true
			}
		}
	case ifaceValue:
		if n.hasRange {
			switch name {
			case "MinimumValue":
				return dbus.MakeVariant(n.valMin), true
			case "MaximumValue":
				return dbus.MakeVariant(n.valMax), true
			case "CurrentValue":
				return dbus.MakeVariant(n.valCur), true
			case "MinimumIncrement":
				return dbus.MakeVariant(float64(0)), true
			}
		}
	case ifaceSelection, ifaceTable, ifaceTableCell:
		return n.p3PropValueLocked(iface, name)
	}
	return dbus.Variant{}, false
}

func (n *atspiNode) Get(iface, name string) (dbus.Variant, *dbus.Error) {
	n.p.mu.Lock()
	defer n.p.mu.Unlock()
	if v, ok := n.propValue(iface, name); ok {
		return v, nil
	}
	return dbus.Variant{}, dbus.NewError("org.freedesktop.DBus.Error.InvalidArgs",
		[]any{"no such property " + iface + "." + name})
}

func (n *atspiNode) GetAll(iface string) (map[string]dbus.Variant, *dbus.Error) {
	n.p.mu.Lock()
	defer n.p.mu.Unlock()
	out := map[string]dbus.Variant{}
	names := []string{"Name", "Description", "Parent", "ChildCount", "Locale", "AccessibleId", "HelpText"}
	switch iface {
	case ifaceApplication:
		names = []string{"ToolkitName", "Version", "ToolkitVersion", "AtspiVersion", "Id"}
	case ifaceText:
		names = []string{"CharacterCount", "CaretOffset"}
	case ifaceValue:
		names = []string{"MinimumValue", "MaximumValue", "CurrentValue", "MinimumIncrement"}
	case ifaceSelection:
		names = []string{"NSelectedChildren"}
	case ifaceTable:
		names = []string{"NRows", "NColumns", "NSelectedRows", "NSelectedColumns", "Caption", "Summary"}
	case ifaceTableCell:
		names = []string{"RowSpan", "ColumnSpan", "Position", "Table"}
	}
	for _, name := range names {
		if v, ok := n.propValue(iface, name); ok {
			out[name] = v
		}
	}
	return out, nil
}

func (n *atspiNode) Set(iface, name string, v dbus.Variant) *dbus.Error {
	if iface == ifaceApplication && name == "Id" {
		n.p.mu.Lock()
		if id, ok := v.Value().(int32); ok {
			n.p.appID = id
		}
		n.p.mu.Unlock()
	}
	return nil
}

// atspiProvider owns the a11y-bus connection and the exported object tree. It is
// a process-wide singleton (accessed only under mu); windows register/unregister
// their frame subtrees against it.
type atspiProvider struct {
	mu      sync.Mutex
	conn    *dbus.Conn
	busName string
	busAddr string
	appID   int32

	root     *atspiNode
	nodes    map[dbus.ObjectPath]*atspiNode
	wins     map[*window]*atspiWinState
	nextID   uint64
	embedded bool
}

// atspiWinState is the per-window slice of the exported tree: the synthetic
// frame node beneath the application root, plus the maps reconciling each
// semantic node's exported object across frames (keyed by CanvasObject, or by
// stable NodeID for virtual/realized nodes).
type atspiWinState struct {
	frame    *atspiNode
	elements map[core.CanvasObject]*atspiNode
	virtual  map[any]*atspiNode
	focused  *atspiNode
	active   bool

	// On-demand realized cells for off-viewport grid coordinates (#988), keyed
	// by the cell's stable identity with FIFO eviction — the AT-SPI mirror of
	// the darwin/UIA realized caches (#637). Entries live outside the
	// reconcile walk (never pruned as unseen); a cell scrolling into view is
	// promoted into virtual, keeping its exported path.
	realized      map[any]*atspiNode
	realizedOrder []any
}

// newAtspiProvider discovers the a11y bus, dials it, exports the application
// root and completes the Socket.Embed handshake. It returns nil when no a11y
// bus is available (accessibility off), so the backend degrades to a no-op.
func newAtspiProvider() *atspiProvider {
	sess, err := dbus.SessionBus()
	if err != nil {
		return nil
	}
	var addr string
	err = sess.Object("org.a11y.Bus", "/org/a11y/bus").
		Call("org.a11y.Bus.GetAddress", 0).Store(&addr)
	if err != nil || addr == "" {
		return nil
	}
	conn, err := dbus.Connect(addr)
	if err != nil {
		return nil
	}

	p := &atspiProvider{
		conn:    conn,
		busName: conn.Names()[0],
		busAddr: addr,
		nodes:   map[dbus.ObjectPath]*atspiNode{},
		wins:    map[*window]*atspiWinState{},
	}

	root := &atspiNode{
		p:        p,
		path:     atspiRootPath,
		role:     atspiRoleApplication,
		roleName: "application",
		name:     atspiAppName(),
		layer:    3,
		parent:   p.ref(atspiNullPath),
		ifaces:   []string{ifaceAccessible, ifaceApplication, ifaceComponent},
		isRoot:   true,
	}
	setState(&root.states, atspiStateVisible)
	p.root = root
	p.nodes[atspiRootPath] = root
	p.exportNode(root)

	// The AT-SPI cache (#987): Orca prefers one GetItems round-trip over walking
	// the tree property-by-property. Exported at the well-known cache path.
	conn.Export(&atspiCache{p: p}, atspiCachePath, ifaceCache)

	// Register with the registry: Socket.Embed(root) → the desktop socket that
	// becomes the application root's parent, so we appear under the AT-SPI tree.
	var socket objectRef
	err = conn.Object(atspiRegistryName, atspiRootPath).
		Call(ifaceSocket+".Embed", 0, p.ref(atspiRootPath)).Store(&socket)
	if err == nil {
		// The embed reply carries the desktop socket (its own bus name + path),
		// which becomes the application root's parent so we appear under the
		// AT-SPI desktop tree.
		root.parent = socket
		p.embedded = true
	}
	return p
}

// atspiAppName resolves the accessible application name from the app metadata,
// falling back to a stable default.
func atspiAppName() string {
	if app := bellui.CurrentApp(); app != nil {
		if name := app.Metadata().Name; name != "" {
			return name
		}
	}
	return "Bellui Application"
}

func itoa(v uint64) string { return strconv.FormatUint(v, 10) }

// exportNode publishes a node under every atspi interface it advertises plus
// org.freedesktop.DBus.Properties. Caller holds p.mu (or is in construction).
func (p *atspiProvider) exportNode(n *atspiNode) {
	for _, iface := range atspiNodeIfaces {
		p.conn.Export(n, n.path, iface)
	}
	if n.isRoot {
		p.conn.Export(n, n.path, ifaceApplication)
	}
}

// unexportNode removes a node's D-Bus handlers. Caller holds p.mu.
func (p *atspiProvider) unexportNode(n *atspiNode) {
	for _, iface := range atspiNodeIfaces {
		p.conn.Export(nil, n.path, iface)
	}
	delete(p.nodes, n.path)
}

// atspiNodeIfaces is the full set of interfaces whose method handlers every
// node exports. Method handlers gate on per-node capability flags, so a call to
// an interface the node does not support returns empty/false; GetInterfaces
// (built per frame in snapshotP1Locked) advertises only the supported subset,
// so Orca only queries the right ones. Value is served through the Properties
// export. Application is exported separately on the root only.
var atspiNodeIfaces = []string{
	ifaceAccessible, ifaceComponent, ifaceProperties,
	ifaceAction, ifaceText, ifaceEditText,
	ifaceSelection, ifaceTable, ifaceTableCell,
}

// newPath allocates a fresh, stable object path. Caller holds p.mu.
func (p *atspiProvider) newPath() dbus.ObjectPath {
	p.nextID++
	return dbus.ObjectPath("/org/a11y/atspi/accessible/" + itoa(p.nextID))
}

// emitStateChanged emits object:state-changed:<detail> in the modern siiv a{sv}
// wire format (verified against live gnome events in the #489 spike). Caller
// must NOT hold p.mu (Emit can block).
func (p *atspiProvider) emitStateChanged(path dbus.ObjectPath, detail string, on int32) {
	_ = p.conn.Emit(path, ifaceEventObject+".StateChanged",
		detail, on, int32(0), dbus.MakeVariant(""), map[string]dbus.Variant{})
}

// emitChildrenChanged emits object:children-changed:add|remove with the child
// reference as any_data. Caller must NOT hold p.mu.
func (p *atspiProvider) emitChildrenChanged(parent dbus.ObjectPath, add bool, index int32, child objectRef) {
	detail := "remove"
	if add {
		detail = "add"
	}
	_ = p.conn.Emit(parent, ifaceEventObject+".ChildrenChanged",
		detail, index, int32(0), dbus.MakeVariant(child), map[string]dbus.Variant{})
}

// emitWindowActivate emits window:activate|deactivate for a frame. Caller must
// NOT hold p.mu.
func (p *atspiProvider) emitWindowActivate(path dbus.ObjectPath, name string, active bool) {
	member := ".Deactivate"
	if active {
		member = ".Activate"
	}
	_ = p.conn.Emit(path, ifaceEventWindow+member,
		"", int32(0), int32(0), dbus.MakeVariant(name), map[string]dbus.Variant{})
}
