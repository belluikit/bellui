package desktop_test

import (
	"testing"

	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/test"
	"github.com/stretchr/testify/assert"
)

// TestStartDrag_UnsupportedDriver verifies the package helper degrades to false
// (so the caller keeps its in-app drag behavior) when the active driver does not
// implement desktop.DragSource — as the test driver does not. The real native
// drag path is macOS-only and needs GUI verification (hack/dragoutprobe).
func TestStartDrag_UnsupportedDriver(t *testing.T) {
	test.NewTempApp(t)

	assert.False(t, desktop.StartDrag(desktop.DragData{Text: "hi"}),
		"a driver without DragSource declines the drag-out")
}

// dragOutBox is a Draggable that also exposes its drag-out payload via the
// pull-based DragOutProvider seam (#874), as the web backend (#857) consumes it.
type dragOutBox struct {
	data desktop.DragData
	ok   bool
}

func (b *dragOutBox) Dragged(*event.DragEvent)              {}
func (b *dragOutBox) DragEnd()                              {}
func (b *dragOutBox) DragOutData() (desktop.DragData, bool) { return b.data, b.ok }

var (
	_ event.Draggable         = (*dragOutBox)(nil)
	_ desktop.DragOutProvider = (*dragOutBox)(nil)
)

// TestDragOutProvider_PullSeam documents the pull-based drag-out contract (#874):
// a backend that suppresses pointer events during a native drag (wasm) reads the
// payload once at drag start via DragOutData instead of the push Dragged path,
// and a false return declines so the in-app drag is kept.
func TestDragOutProvider_PullSeam(t *testing.T) {
	box := &dragOutBox{data: desktop.DragData{Text: "pull"}, ok: true}

	var src event.Draggable = box
	p, ok := src.(desktop.DragOutProvider)
	assert.True(t, ok, "a draggable may expose the pull seam via a type assertion")

	data, ok := p.DragOutData()
	assert.True(t, ok)
	assert.Equal(t, "pull", data.Text)

	box.ok = false
	_, ok = p.DragOutData()
	assert.False(t, ok, "a false return declines the drag-out, keeping the in-app drag")
}
