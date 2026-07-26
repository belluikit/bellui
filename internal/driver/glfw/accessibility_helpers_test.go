//go:build accessibility

package glfw

import (
	"testing"
	"unicode/utf8"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/accessibility"
	"github.com/stretchr/testify/assert"
)

func TestA11yTextEditDiff(t *testing.T) {
	for name, tt := range map[string]struct {
		prev, cur         string
		deleted, inserted string
	}{
		"backspace one char":      {"hello", "hell", "o", ""},
		"backspace mid":           {"hello", "helo", "l", ""},
		"delete a space at end":   {"ab ", "ab", " ", ""},
		"type one char":           {"hell", "hello", "", "o"},
		"insert mid":              {"helo", "hello", "", "l"},
		"replace selection":       {"hello world", "hello there", "world", "there"},
		"clear all":               {"abc", "", "abc", ""},
		"set from empty":          {"", "abc", "", "abc"},
		"no change":               {"same", "same", "", ""},
		"multibyte delete":        {"héllo", "hllo", "é", ""},
		"multibyte insert":        {"hllo", "héllo", "", "é"},
		"emoji backspace":         {"hi👋", "hi", "👋", ""},
		"ambiguous repeat delete": {"aaa", "aa", "a", ""},
	} {
		t.Run(name, func(t *testing.T) {
			deleted, inserted := a11yTextEditDiff(tt.prev, tt.cur)
			assert.Equal(t, tt.deleted, deleted)
			assert.Equal(t, tt.inserted, inserted)
			assert.True(t, utf8.ValidString(deleted))
			assert.True(t, utf8.ValidString(inserted))
		})
	}
}

// a11yBaseObject is a no-op CanvasObject the focus test stubs embed.
type a11yBaseObject struct{}

func (o *a11yBaseObject) MinSize() geo.Size      { return geo.Size{} }
func (o *a11yBaseObject) Move(geo.Position)      {}
func (o *a11yBaseObject) Position() geo.Position { return geo.Position{} }
func (o *a11yBaseObject) Resize(geo.Size)        {}
func (o *a11yBaseObject) Size() geo.Size         { return geo.Size{} }
func (o *a11yBaseObject) Hide()                  {}
func (o *a11yBaseObject) Visible() bool          { return true }
func (o *a11yBaseObject) Show()                  {}
func (o *a11yBaseObject) Refresh()               {}

// a11yPlainObject is a CanvasObject that does not implement
// AccessibleActiveDescendant.
type a11yPlainObject struct{ a11yBaseObject }

// a11yFocusObject delegates AT focus to a descendant identity when delegates is
// set (implements core.AccessibleActiveDescendant).
type a11yFocusObject struct {
	a11yBaseObject
	designated any
	delegates  bool
}

func (o *a11yFocusObject) AccessibleActiveDescendant() any {
	if !o.delegates {
		panic("AccessibleActiveDescendant called on non-delegating object")
	}
	return o.designated
}

var (
	_ core.CanvasObject               = (*a11yPlainObject)(nil)
	_ core.AccessibleActiveDescendant = (*a11yFocusObject)(nil)
)

func TestA11yActiveDescendant(t *testing.T) {
	assert.Nil(t, a11yActiveDescendant(nil), "nil object delegates nothing")

	// An object that does not implement AccessibleActiveDescendant → nil.
	assert.Nil(t, a11yActiveDescendant(&a11yPlainObject{}), "non-delegating object")

	// A container with no internal cursor implements the interface but returns nil.
	assert.Nil(t, a11yActiveDescendant(&a11yFocusObject{delegates: true}),
		"container with no internal cursor")

	del := &a11yFocusObject{delegates: true, designated: [2]int{4, 2}}
	assert.Equal(t, [2]int{4, 2}, a11yActiveDescendant(del), "designated descendant key surfaces")
}

func TestA11yNodeFocused(t *testing.T) {
	container := &a11yFocusObject{}
	cellNode := func(id any, state core.AccessibleState) *accessibility.Node {
		n := &accessibility.Node{}
		n.NodeID = id
		n.State = state
		return n
	}

	// No active descendant: focus is by object identity.
	containerNode := &accessibility.Node{}
	containerNode.Object = container
	assert.True(t, a11yNodeFocused(containerNode, container, nil), "focused object is focused")
	assert.False(t, a11yNodeFocused(&accessibility.Node{}, container, nil), "unfocused object")

	// Active descendant designated: the container hands focus off to the
	// descendant whose NodeID matches, never itself.
	key := [2]int{4, 2}
	assert.False(t, a11yNodeFocused(containerNode, container, key),
		"delegating container is no longer focused")
	assert.True(t, a11yNodeFocused(cellNode(key, 0), container, key),
		"descendant matched by identity is focused")
	assert.False(t, a11yNodeFocused(cellNode([2]int{4, 3}, 0), container, key),
		"a different descendant is not focused")

	// StateFocused makes a node report focus even without an identity match —
	// the node-level flag that keeps HasKeyboardFocus correct for the cursor cell.
	assert.True(t, a11yNodeFocused(cellNode([2]int{9, 9}, core.StateFocused), container, key),
		"explicit StateFocused wins")
	assert.True(t, a11yNodeFocused(cellNode(nil, core.StateFocused), container, key),
		"StateFocused works without a NodeID")
}

func TestIntersectRect(t *testing.T) {
	rect := func(x, y, w, h float32) geo.Rectangle {
		return geo.Rectangle{Pos: geo.NewPos(x, y), Size: geo.NewSize(w, h)}
	}

	// Partial overlap trims to the shared area — the #1003 case of a range
	// rectangle extending past the node's ancestor clip.
	got, ok := intersectRect(rect(10, 10, 100, 40), rect(0, 0, 60, 30))
	assert.True(t, ok)
	assert.Equal(t, rect(10, 10, 50, 20), got)

	// Fully inside the clip: unchanged.
	got, ok = intersectRect(rect(5, 5, 10, 10), rect(0, 0, 100, 100))
	assert.True(t, ok)
	assert.Equal(t, rect(5, 5, 10, 10), got)

	// Disjoint: empty.
	_, ok = intersectRect(rect(0, 0, 10, 10), rect(20, 20, 5, 5))
	assert.False(t, ok)

	// Edge-touching (zero-area intersection) counts as empty.
	_, ok = intersectRect(rect(0, 0, 10, 10), rect(10, 0, 5, 5))
	assert.False(t, ok)
}
