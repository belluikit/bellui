//go:build accessibility && linux && !android && !wasm && !js

package glfw

import (
	"testing"

	"github.com/godbus/dbus/v5"
)

func TestCacheItemLocked(t *testing.T) {
	p := &atspiProvider{busName: ":1.5", nodes: map[dbus.ObjectPath]*atspiNode{}}
	parent := &atspiNode{
		p:        p,
		path:     "/org/a11y/atspi/accessible/1",
		children: []dbus.ObjectPath{"/org/a11y/atspi/accessible/2", "/org/a11y/atspi/accessible/3"},
	}
	child := &atspiNode{
		p:        p,
		path:     "/org/a11y/atspi/accessible/3",
		parent:   p.ref(parent.path),
		name:     "Click me",
		desc:     "a button",
		role:     atspiRolePushButton,
		ifaces:   []string{ifaceAccessible, ifaceComponent, ifaceAction},
		children: []dbus.ObjectPath{"/org/a11y/atspi/accessible/9"},
	}
	setState(&child.states, atspiStateFocusable)
	setState(&child.states, atspiStateFocused)
	p.nodes[parent.path] = parent
	p.nodes[child.path] = child

	item := p.cacheItemLocked(child)

	if item.Ref.Name != ":1.5" || item.Ref.Path != child.path {
		t.Errorf("Ref = %+v", item.Ref)
	}
	if item.App.Path != atspiRootPath {
		t.Errorf("App = %+v; want root", item.App)
	}
	if item.Parent.Path != parent.path {
		t.Errorf("Parent = %+v; want %s", item.Parent, parent.path)
	}
	if item.IndexInParent != 1 { // child is parent.children[1]
		t.Errorf("IndexInParent = %d; want 1", item.IndexInParent)
	}
	if item.ChildCount != 1 {
		t.Errorf("ChildCount = %d; want 1", item.ChildCount)
	}
	if item.Name != "Click me" || item.Description != "a button" {
		t.Errorf("Name/Description = %q / %q", item.Name, item.Description)
	}
	if item.Role != atspiRolePushButton {
		t.Errorf("Role = %d; want %d", item.Role, atspiRolePushButton)
	}
	if len(item.Interfaces) != 3 || item.Interfaces[2] != ifaceAction {
		t.Errorf("Interfaces = %v", item.Interfaces)
	}
	if len(item.States) != 2 || !hasState([2]uint32{item.States[0], item.States[1]}, atspiStateFocused) {
		t.Errorf("States = %v; want focused set", item.States)
	}
}

func TestCacheItemLocked_RootHasNoParentIndex(t *testing.T) {
	p := &atspiProvider{busName: ":1.5", nodes: map[dbus.ObjectPath]*atspiNode{}}
	// A node whose parent is the desktop socket (a different bus name) has no
	// resolvable index in our own tree.
	root := &atspiNode{
		p:      p,
		path:   atspiRootPath,
		parent: objectRef{Name: ":1.0", Path: "/org/a11y/atspi/accessible/root"},
	}
	p.nodes[root.path] = root
	item := p.cacheItemLocked(root)
	if item.IndexInParent != -1 {
		t.Errorf("IndexInParent = %d; want -1 for foreign parent", item.IndexInParent)
	}
}
