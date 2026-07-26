package inspector

import (
	"fmt"
	"strings"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/driver"
)

// objectKind classifies a CanvasObject for display in the tree.
type objectKind int

const (
	kindPrimitive objectKind = iota // a leaf canvas primitive (text, rectangle, image, ...)
	kindContainer                   // a *bellui.Container
	kindWidget                      // a core.Widget with a renderer
)

func (k objectKind) String() string {
	switch k {
	case kindContainer:
		return "container"
	case kindWidget:
		return "widget"
	default:
		return "primitive"
	}
}

// node is one entry in the inspected object tree. It is a snapshot taken at
// walk time; the live object is retained so the detail pane and highlight can
// re-read its current geometry on demand.
type node struct {
	uid      string
	obj      core.CanvasObject
	typeName string
	kind     objectKind
	absPos   geo.Position
	size     geo.Size
	clip     geo.Rectangle
	hasClip  bool
	visible  bool
	children []*node
}

// snapshot is an immutable capture of a target canvas's visible object forest,
// with each node addressable by a stable per-refresh UID for widget.Tree, or by
// the live object it captured (for pick mode).
type snapshot struct {
	roots []*node
	byUID map[string]*node
	byObj map[core.CanvasObject]*node
}

// childUIDs returns the child UIDs of uid; the empty UID is the synthetic tree
// root whose children are the forest roots.
func (s *snapshot) childUIDs(uid string) []string {
	if s == nil {
		return nil
	}
	var children []*node
	if uid == "" {
		children = s.roots
	} else if n := s.byUID[uid]; n != nil {
		children = n.children
	}
	ids := make([]string, len(children))
	for i, c := range children {
		ids[i] = c.uid
	}
	return ids
}

func (s *snapshot) isBranch(uid string) bool {
	if s == nil {
		return false
	}
	if uid == "" {
		return len(s.roots) > 0
	}
	n := s.byUID[uid]
	return n != nil && len(n.children) > 0
}

// snapshotRoots walks the given root objects and returns their combined visible
// forest. The walk descends both containers and widget-renderer internals, and
// records absolute positions and clip bounds exactly as the paint and hit-test
// walks do (it is driven by the same driver.WalkVisibleObjectTree).
func snapshotRoots(roots []core.CanvasObject) *snapshot {
	s := &snapshot{byUID: map[string]*node{}, byObj: map[core.CanvasObject]*node{}}
	var stack []*node

	before := func(obj core.CanvasObject, pos, clipPos geo.Position, clipSize geo.Size) bool {
		n := &node{
			obj:      obj,
			typeName: fmt.Sprintf("%T", obj),
			kind:     kindOf(obj),
			absPos:   pos,
			size:     obj.Size(),
			visible:  obj.Visible(),
		}
		if clipSize != driver.NoClipBounds {
			n.clip = geo.Rectangle{Pos: clipPos, Size: clipSize}
			n.hasClip = true
		}
		if len(stack) > 0 {
			parent := stack[len(stack)-1]
			n.uid = fmt.Sprintf("%s/%d", parent.uid, len(parent.children))
			parent.children = append(parent.children, n)
		} else {
			n.uid = fmt.Sprintf("%d", len(s.roots))
			s.roots = append(s.roots, n)
		}
		s.byUID[n.uid] = n
		s.byObj[obj] = n
		stack = append(stack, n)
		return false
	}
	after := func(obj core.CanvasObject, _ geo.Position, _ core.CanvasObject) {
		if len(stack) > 0 && stack[len(stack)-1].obj == obj {
			stack = stack[:len(stack)-1]
		}
	}

	for _, root := range roots {
		if root == nil {
			continue
		}
		driver.WalkVisibleObjectTree(root, before, after)
		stack = stack[:0] // defensive: each root is a fresh top-level walk
	}
	return s
}

func kindOf(obj core.CanvasObject) objectKind {
	switch obj.(type) {
	case *bellui.Container:
		return kindContainer
	case core.Widget:
		return kindWidget
	default:
		return kindPrimitive
	}
}

// typeName is the concrete Go type of obj (e.g. "*widget.Button").
func typeName(obj core.CanvasObject) string { return fmt.Sprintf("%T", obj) }

// nodeLabel is the one-line tree row for a node: its type, with a marker for
// hidden objects.
func nodeLabel(n *node) string {
	if n == nil {
		return "?"
	}
	if !n.visible {
		return n.typeName + "  (hidden)"
	}
	return n.typeName
}

// parentUID returns the UID of a node's parent given its path UID
// ("0/2/1" → "0/2"); the empty string is the synthetic tree root.
func parentUID(uid string) string {
	if i := strings.LastIndex(uid, "/"); i >= 0 {
		return uid[:i]
	}
	return ""
}
