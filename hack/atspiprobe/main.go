// Command atspiprobe is a research spike for pre-rebrand tracker #489: a *pure-Go* AT-SPI2
// accessibility provider, speaking the org.a11y.atspi.* D-Bus protocol directly
// through github.com/godbus/dbus/v5 (already a fork dependency) with no cgo and
// no Rust/AccessKit.
//
// It exposes a tiny, static accessible tree (an application → a frame → a button
// and a label) to the AT-SPI2 registry so a Linux screen reader (Orca) — or any
// AT client (busctl, Accerciser, pyatspi) — can discover and read it. The point
// is to prove the provider handshake works end-to-end on a real desktop:
//
//	session bus → org.a11y.Bus.GetAddress → dial the a11y bus →
//	export Accessible/Application/Component objects → Socket.Embed against the
//	registry root → stay alive, emit a focus StateChanged.
//
// This is throwaway spike code, deliberately standalone (it does NOT touch the
// bellui canvas or the internal/accessibility node tree); the production backend
// is the follow-up implementation issue. Run on the linux a11y box:
//
//	go run ./hack/atspiprobe          # then: busctl / Orca reads the tree
//
// See hack/atspiprobe/README.md for the verification recipe.
package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/prop"
)

// AT-SPI2 role numbers (AtspiRole enum). Only the handful the spike uses.
const (
	roleApplication = 75
	roleFrame       = 23
	rolePushButton  = 43
	roleLabel       = 29
)

// AT-SPI2 state bits (AtspiStateType enum), packed into word 0 of the `au`
// state bitfield the Accessible.GetState method returns.
const (
	stateActive    = 1
	stateEnabled   = 8
	stateFocusable = 11
	stateFocused   = 12
	stateSensitive = 24
	stateShowing   = 25
	stateVisible   = 30
)

const (
	ifaceAccessible  = "org.a11y.atspi.Accessible"
	ifaceApplication = "org.a11y.atspi.Application"
	ifaceComponent   = "org.a11y.atspi.Component"
	ifaceSocket      = "org.a11y.atspi.Socket"
	ifaceEventObject = "org.a11y.atspi.Event.Object"

	registryName = "org.a11y.atspi.Registry"
	rootPath     = "/org/a11y/atspi/accessible/root"
)

// objectRef is the AT-SPI2 (so) object reference: a (bus name, object path)
// pair naming an accessible anywhere on the a11y bus. godbus marshals this
// struct as the D-Bus struct type "(so)".
type objectRef struct {
	Name string
	Path dbus.ObjectPath
}

// rect is the AT-SPI2 (iiii) rectangle returned by Component.GetExtents.
type rect struct{ X, Y, W, H int32 }

// node is one accessible in the spike tree. A single Go type backs every
// object; each instance is exported at its own path under the three atspi
// interfaces it needs. In the real backend these fields come from the
// internal/accessibility.Node walk instead of being hand-built.
type node struct {
	app      *provider
	path     dbus.ObjectPath
	role     uint32
	roleName string
	name     string
	desc     string
	states   uint32 // packed word-0 state bits
	extents  rect
	parent   objectRef
	children []dbus.ObjectPath
	ifaces   []string
}

func (n *node) ref() objectRef { return objectRef{n.app.busName, n.path} }

// --- org.a11y.atspi.Accessible -------------------------------------------

func (n *node) GetRole() (uint32, *dbus.Error)              { return n.role, nil }
func (n *node) GetRoleName() (string, *dbus.Error)          { return n.roleName, nil }
func (n *node) GetLocalizedRoleName() (string, *dbus.Error) { return n.roleName, nil }

func (n *node) GetState() ([]uint32, *dbus.Error) {
	// Two 32-bit words; every state the spike uses lives in word 0.
	return []uint32{n.states, 0}, nil
}

func (n *node) GetChildCount() (int32, *dbus.Error) { return int32(len(n.children)), nil }

func (n *node) GetChildAtIndex(i int32) (objectRef, *dbus.Error) {
	if i < 0 || int(i) >= len(n.children) {
		return objectRef{}, nil
	}
	return objectRef{n.app.busName, n.children[i]}, nil
}

func (n *node) GetChildren() ([]objectRef, *dbus.Error) {
	refs := make([]objectRef, len(n.children))
	for i, c := range n.children {
		refs[i] = objectRef{n.app.busName, c}
	}
	return refs, nil
}

func (n *node) GetIndexInParent() (int32, *dbus.Error) {
	if p := n.app.byPath(n.parent.Path); p != nil {
		for i, c := range p.children {
			if c == n.path {
				return int32(i), nil
			}
		}
	}
	return -1, nil
}

func (n *node) GetApplication() (objectRef, *dbus.Error) {
	return objectRef{n.app.busName, rootPath}, nil
}

func (n *node) GetInterfaces() ([]string, *dbus.Error) { return n.ifaces, nil }

func (n *node) GetAttributes() (map[string]string, *dbus.Error) {
	return map[string]string{}, nil
}

// GetRelationSet returns the a(ua(so)) relation array — none for the spike.
func (n *node) GetRelationSet() ([]struct {
	Type uint32
	Refs []objectRef
}, *dbus.Error,
) {
	return nil, nil
}

// --- org.a11y.atspi.Component --------------------------------------------

func (n *node) Contains(x, y int32, coord uint32) (bool, *dbus.Error) {
	return x >= n.extents.X && x < n.extents.X+n.extents.W &&
		y >= n.extents.Y && y < n.extents.Y+n.extents.H, nil
}

func (n *node) GetAccessibleAtPoint(x, y int32, coord uint32) (objectRef, *dbus.Error) {
	return objectRef{}, nil
}

func (n *node) GetExtents(coord uint32) (rect, *dbus.Error) { return n.extents, nil }

func (n *node) GetPosition(coord uint32) (int32, int32, *dbus.Error) {
	return n.extents.X, n.extents.Y, nil
}

func (n *node) GetSize() (int32, int32, *dbus.Error) { return n.extents.W, n.extents.H, nil }

func (n *node) GetLayer() (uint32, *dbus.Error) { return 3, nil } // ATSPI_LAYER_WIDGET

func (n *node) GetMDIZOrder() (int16, *dbus.Error) { return 0, nil }

func (n *node) GrabFocus() (bool, *dbus.Error) {
	log.Printf("GrabFocus on %s (%q)", n.path, n.name)
	n.app.setFocus(n)
	return true, nil
}

func (n *node) GetAlpha() (float64, *dbus.Error) { return 1.0, nil }

// --- org.a11y.atspi.Application (root only) -------------------------------

func (n *node) GetLocale(lctype uint32) (string, *dbus.Error) { return "en_US.UTF-8", nil }

func (n *node) GetApplicationBusAddress() (string, *dbus.Error) {
	return n.app.busAddr, nil
}

// provider owns the a11y-bus connection and the exported node tree.
type provider struct {
	conn    *dbus.Conn
	busName string
	busAddr string
	nodes   map[dbus.ObjectPath]*node
	desktop objectRef // our parent socket, from Socket.Embed
	props   map[dbus.ObjectPath]*prop.Properties
}

func (p *provider) byPath(path dbus.ObjectPath) *node { return p.nodes[path] }

// setFocus flips the focused state on n and emits the AT-SPI2 focus event Orca
// listens for: object:state-changed:focused with detail1=1. Signal signature
// is siiva{sv} (verified against live gnome events on the box).
func (p *provider) setFocus(n *node) {
	for _, other := range p.nodes {
		if other != n && other.states&(1<<stateFocused) != 0 {
			other.states &^= 1 << stateFocused
			p.emitStateChanged(other, "focused", 0)
		}
	}
	n.states |= 1 << stateFocused
	p.emitStateChanged(n, "focused", 1)
}

func (p *provider) emitStateChanged(n *node, detail string, detail1 int32) {
	err := p.conn.Emit(
		n.path, ifaceEventObject+".StateChanged",
		detail, detail1, int32(0),
		dbus.MakeVariant(""),      // any_data
		map[string]dbus.Variant{}, // properties a{sv}
	)
	if err != nil {
		log.Printf("emit StateChanged(%s=%d) on %s: %v", detail, detail1, n.path, err)
	}
}

func main() {
	log.SetFlags(log.Ltime)
	log.Println("atspiprobe — pure-Go AT-SPI2 provider spike (pre-rebrand tracker #489)")

	// 1. Discover the a11y bus address via the session bus.
	sess, err := dbus.SessionBus()
	if err != nil {
		log.Fatalf("session bus: %v", err)
	}
	var addr string
	err = sess.Object("org.a11y.Bus", "/org/a11y/bus").
		Call("org.a11y.Bus.GetAddress", 0).Store(&addr)
	if err != nil {
		log.Fatalf("org.a11y.Bus.GetAddress: %v (is the a11y bus running?)", err)
	}
	log.Printf("a11y bus: %s", addr)

	// 2. Dial the a11y bus (a *secondary* bus; Connect does Dial+Auth+Hello).
	conn, err := dbus.Connect(addr)
	if err != nil {
		log.Fatalf("dial a11y bus: %v", err)
	}
	defer conn.Close()

	p := &provider{
		conn:    conn,
		busName: conn.Names()[0], // our unique name on the a11y bus, e.g. :1.42
		busAddr: addr,
		nodes:   map[dbus.ObjectPath]*node{},
		props:   map[dbus.ObjectPath]*prop.Properties{},
	}
	log.Printf("our a11y bus name: %s", p.busName)

	// 3. Build a tiny static tree: application → frame → { button, label }.
	buildTree(p)

	// 4. Export every node under the interfaces it advertises, plus its
	//    Accessible/Application properties.
	for _, n := range p.nodes {
		exportNode(p, n)
	}

	// 5. Register with the registry: Socket.Embed(our root) → desktop socket,
	//    which becomes our application root's Parent.
	var socket objectRef
	err = conn.Object(registryName, rootPath).
		Call(ifaceSocket+".Embed", 0, p.nodes[rootPath].ref()).Store(&socket)
	if err != nil {
		log.Fatalf("Socket.Embed: %v", err)
	}
	p.desktop = socket
	p.nodes[rootPath].parent = socket
	// The root's Parent property was exported before Embed returned the desktop
	// socket; publish the real value now so Orca sees the app under the desktop.
	p.props[rootPath].SetMust(ifaceAccessible, "Parent", socket)
	log.Printf("embedded — desktop socket parent = %s %s", socket.Name, socket.Path)
	log.Printf("tree is live; %d objects exported. Point Orca / busctl at bus name %s", len(p.nodes), p.busName)

	// 6. Announce focus on the button so a running Orca speaks it.
	go func() {
		time.Sleep(500 * time.Millisecond)
		p.setFocus(p.nodes["/org/a11y/atspi/accessible/2"])
		log.Println("emitted focus on the button")
	}()

	fmt.Println("\nprovider running — Ctrl-C to quit")
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Println("shutting down")
}

func buildTree(p *provider) {
	root := &node{
		app: p, path: rootPath, role: roleApplication, roleName: "application",
		name: "Bellui AT-SPI Spike", states: 1 << stateVisible,
		children: []dbus.ObjectPath{"/org/a11y/atspi/accessible/1"},
		ifaces:   []string{ifaceAccessible, ifaceApplication, ifaceComponent},
	}
	frame := &node{
		app: p, path: "/org/a11y/atspi/accessible/1", role: roleFrame, roleName: "frame",
		name:    "Spike Window",
		states:  (1 << stateActive) | (1 << stateShowing) | (1 << stateVisible) | (1 << stateEnabled) | (1 << stateSensitive),
		extents: rect{100, 100, 400, 300},
		parent:  root.ref(),
		children: []dbus.ObjectPath{
			"/org/a11y/atspi/accessible/2",
			"/org/a11y/atspi/accessible/3",
		},
		ifaces: []string{ifaceAccessible, ifaceComponent},
	}
	button := &node{
		app: p, path: "/org/a11y/atspi/accessible/2", role: rolePushButton, roleName: "push button",
		name:    "Click me",
		desc:    "A button exposed from pure Go",
		states:  (1 << stateShowing) | (1 << stateVisible) | (1 << stateEnabled) | (1 << stateSensitive) | (1 << stateFocusable),
		extents: rect{120, 160, 100, 40},
		parent:  frame.ref(),
		ifaces:  []string{ifaceAccessible, ifaceComponent},
	}
	label := &node{
		app: p, path: "/org/a11y/atspi/accessible/3", role: roleLabel, roleName: "label",
		name:    "Hello from Go",
		states:  (1 << stateShowing) | (1 << stateVisible) | (1 << stateEnabled) | (1 << stateSensitive),
		extents: rect{120, 210, 200, 24},
		parent:  frame.ref(),
		ifaces:  []string{ifaceAccessible, ifaceComponent},
	}
	for _, n := range []*node{root, frame, button, label} {
		p.nodes[n.path] = n
	}
}

func exportNode(p *provider, n *node) {
	conn := p.conn
	must := func(err error, what string) {
		if err != nil {
			log.Fatalf("export %s on %s: %v", what, n.path, err)
		}
	}
	must(conn.Export(n, n.path, ifaceAccessible), ifaceAccessible)
	must(conn.Export(n, n.path, ifaceComponent), ifaceComponent)
	if n.path == rootPath {
		must(conn.Export(n, n.path, ifaceApplication), ifaceApplication)
	}

	// Accessible + Application properties (Name, Parent, ChildCount, toolkit
	// identity, ...). prop.Export also wires org.freedesktop.DBus.Properties.
	props := prop.Map{
		ifaceAccessible: {
			"Name":         {Value: n.name, Writable: false},
			"Description":  {Value: n.desc, Writable: false},
			"Parent":       {Value: n.parent, Writable: false},
			"ChildCount":   {Value: int32(len(n.children)), Writable: false},
			"Locale":       {Value: "en_US.UTF-8", Writable: false},
			"AccessibleId": {Value: string(n.path), Writable: false},
			"HelpText":     {Value: n.desc, Writable: false},
		},
	}
	if n.path == rootPath {
		props[ifaceApplication] = map[string]*prop.Prop{
			"ToolkitName":      {Value: "Bellui", Writable: false},
			"Version":          {Value: "3", Writable: false},
			"ToolkitVersion":   {Value: "3", Writable: false},
			"AtspiVersion":     {Value: "2.1", Writable: false},
			"InterfaceVersion": {Value: uint32(0), Writable: false},
			"Id":               {Value: int32(0), Writable: true},
		}
	}
	pr, err := prop.Export(conn, n.path, props)
	if err != nil {
		log.Fatalf("prop.Export on %s: %v", n.path, err)
	}
	p.props[n.path] = pr
}
