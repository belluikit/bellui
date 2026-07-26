//go:build accessibility

package glfw

import (
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/accessibility"
)

// Platform-neutral helpers shared by the screen-reader backends (darwin
// NSAccessibility #485, windows UIA #487). They interpret the semantic
// node's action list and geometry; nothing here touches a platform API.

// a11yPressAction picks the accessible action a press (AXPress, UIA Invoke)
// maps to, or "" when the node declares none that fits a press.
func a11yPressAction(n *accessibility.Node) string {
	for _, act := range n.Actions {
		switch act.Name {
		case core.ActionPress, core.ActionToggle, core.ActionSelect, core.ActionDismiss:
			return act.Name
		}
	}
	return ""
}

// a11yActiveDescendant returns the stable identity key of the AT-focused
// descendant designated by a focusable container, or nil when the object does
// not delegate — canvas focus then stays on the object itself. It carries the
// aria-activedescendant channel (#688): a data grid, tree or virtualized list
// moves an internal cursor across descendant items and names the focused one,
// so the platform bridge resolves AT focus (and raises the focus-change event)
// to that descendant instead of the whole container.
func a11yActiveDescendant(obj core.CanvasObject) any {
	if obj == nil {
		return nil
	}
	if ad, ok := obj.(core.AccessibleActiveDescendant); ok {
		return ad.AccessibleActiveDescendant()
	}
	return nil
}

// a11yNodeFocused reports whether node n should carry AT keyboard focus.
// Normally exactly the canvas-focused object is focused; when that object
// delegates to an active descendant (#688), focus moves off the container onto
// the descendant node whose stable identity matches the designated key — or one
// that explicitly reports [core.StateFocused], the node-level focus flag that
// makes HasKeyboardFocus/IsKeyboardFocusable correct for the cell the cursor is
// on.
func a11yNodeFocused(n *accessibility.Node, focusedObj core.CanvasObject, activeDescendant any) bool {
	if activeDescendant != nil {
		return (n.NodeID != nil && n.NodeID == activeDescendant) || n.State.Has(core.StateFocused)
	}
	return n.Object != nil && n.Object == focusedObj
}

// a11yHasAction reports whether the node declares the named accessible action.
func a11yHasAction(n *accessibility.Node, name string) bool {
	for _, act := range n.Actions {
		if act.Name == name {
			return true
		}
	}
	return false
}

// a11yDisclosed reports the disclosure state for expandable nodes:
// -1 not disclosable, 0 collapsed, 1 expanded. Branch rows declare exactly
// one of the expand/collapse actions depending on their current state.
func a11yDisclosed(n *accessibility.Node) int {
	if n.State.Has(core.StateExpanded) || a11yHasAction(n, core.ActionCollapse) {
		return 1
	}
	if a11yHasAction(n, core.ActionExpand) {
		return 0
	}
	return -1
}

// intersectRect returns the intersection of two rectangles; ok is false when
// they do not overlap (the intersection is empty).
func intersectRect(a, b geo.Rectangle) (geo.Rectangle, bool) {
	x1 := max(a.Pos.X, b.Pos.X)
	y1 := max(a.Pos.Y, b.Pos.Y)
	x2 := min(a.Pos.X+a.Size.Width, b.Pos.X+b.Size.Width)
	y2 := min(a.Pos.Y+a.Size.Height, b.Pos.Y+b.Size.Height)
	if x2 <= x1 || y2 <= y1 {
		return geo.Rectangle{}, false
	}
	return geo.Rectangle{Pos: geo.NewPos(x1, y1), Size: geo.NewSize(x2-x1, y2-y1)}, true
}

func rectsOverlap(a, b geo.Rectangle) bool {
	return a.Pos.X < b.Pos.X+b.Size.Width && b.Pos.X < a.Pos.X+a.Size.Width &&
		a.Pos.Y < b.Pos.Y+b.Size.Height && b.Pos.Y < a.Pos.Y+a.Size.Height
}

func n2AbsCenter(n *accessibility.Node) geo.Position {
	return geo.NewPos(n.Pos.X+n.Size.Width/2, n.Pos.Y+n.Size.Height/2)
}

// a11ySelected reports AXSelected/SelectionItem state for selectable items:
// -1 not applicable, 0 not selected, 1 selected. Radio buttons and tabs are
// excluded — their selection travels as the checked value (darwin) or
// SelectionItem pattern (windows) already.
func a11ySelected(n *accessibility.Node) int {
	switch n.Role {
	case core.RoleTableCell, core.RoleListItem, core.RoleTreeItem, core.RoleTableRow:
		if n.State.Has(core.StateSelected) {
			return 1
		}
		return 0
	}
	return -1
}

// a11yTextEditDiff strips the longest common prefix and suffix (aligned to
// rune boundaries) from the two versions of a text and returns what the
// edit removed and added. Plain keystrokes cost O(edit); the pathological
// case is a full-document replace.
func a11yTextEditDiff(prev, cur string) (deleted, inserted string) {
	p := 0
	for p < len(prev) && p < len(cur) && prev[p] == cur[p] {
		p++
	}
	// Back off to a rune start so the reported strings are valid UTF-8.
	for p > 0 && ((p < len(prev) && prev[p]&0xC0 == 0x80) || (p < len(cur) && cur[p]&0xC0 == 0x80)) {
		p--
	}
	s := 0
	for s < len(prev)-p && s < len(cur)-p && prev[len(prev)-1-s] == cur[len(cur)-1-s] {
		s++
	}
	for s > 0 && ((prev[len(prev)-s]&0xC0) == 0x80 || (cur[len(cur)-s]&0xC0) == 0x80) {
		s--
	}
	return prev[p : len(prev)-s], cur[p : len(cur)-s]
}
