package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// Desktop-leg coverage for the pre-rebrand tracker #944 selectable long-press machinery: the
// word-select path is device-independent, while the handle overlay is
// mobile-only (see selectable_handles_mobile_test.go for the overlay tests).

func TestSelectableLabel_LongPressed_SelectsWord_NoHandlesOnDesktop(t *testing.T) {
	test.NewTempApp(t)
	if bellui.CurrentDevice().IsMobile() {
		t.Skip("the mobile leg exercises the overlay in selectable_handles_mobile_test.go")
	}
	l := NewLabel("hello world")
	l.Selectable = true
	w := test.NewTempWindow(t, l)
	w.Resize(geo.NewSize(200, 60))

	sel := &l.selection.selectable
	pos := sel.selectionHandleCaretRect(0, 2).Pos // inside "hello"
	l.selection.LongPressed(&event.PointEvent{Position: pos})

	assert.Equal(t, "hello", l.SelectedText(), "long-press word-select works regardless of device")
	assert.Nil(t, sel.selHandles, "no handle overlay off mobile")
	assert.Empty(t, w.Canvas().Overlays().List())
}

// fakeSelectableHost records forwarded gestures; the LongPressable variant
// also takes long-press.
type fakeSelectableHost struct{ tapped, longPressed int }

func (f *fakeSelectableHost) Tapped(*event.PointEvent)       { f.tapped++ }
func (f *fakeSelectableHost) DoubleTapped(*event.PointEvent) {}
func (f *fakeSelectableHost) Dragged(*event.DragEvent)       {}
func (f *fakeSelectableHost) DragEnd()                       {}

type fakeLongPressableHost struct{ fakeSelectableHost }

func (f *fakeLongPressableHost) LongPressed(*event.PointEvent) { f.longPressed++ }

func TestSelectable_LongPressed_HostOwnsGesture(t *testing.T) {
	test.NewTempApp(t)
	rich := NewRichTextWithText("hello world")

	// A host that is not LongPressable: the selectable must not drive its own
	// selection model under a host (mirrors Tapped/Dragged forwarding).
	inert := &fakeSelectableHost{}
	s := &selectable{theme: rich.Theme(), provider: rich, host: inert}
	s.LongPressed(&event.PointEvent{Position: geo.NewPos(10, 5)})
	assert.False(t, s.selecting, "no self-driven selection under a host")

	// A LongPressable host receives the forwarded gesture.
	lp := &fakeLongPressableHost{}
	s2 := &selectable{theme: rich.Theme(), provider: rich, host: lp}
	s2.LongPressed(&event.PointEvent{Position: geo.NewPos(10, 5)})
	assert.Equal(t, 1, lp.longPressed, "long-press forwards to a LongPressable host")
	assert.False(t, s2.selecting)
}

func TestSelectable_PositionBeforeAfter(t *testing.T) {
	test.NewTempApp(t)
	rich := NewRichTextWithText("ab\ncd")
	s := &selectable{theme: rich.Theme(), provider: rich}

	r, c, ok := s.positionBefore(0, 1)
	require.True(t, ok)
	assert.Equal(t, [2]int{0, 0}, [2]int{r, c})

	r, c, ok = s.positionBefore(1, 0) // steps over the row boundary
	require.True(t, ok)
	assert.Equal(t, [2]int{0, 2}, [2]int{r, c})

	_, _, ok = s.positionBefore(0, 0)
	assert.False(t, ok, "no position before the document start")

	r, c, ok = s.positionAfter(0, 2) // steps over the row boundary
	require.True(t, ok)
	assert.Equal(t, [2]int{1, 0}, [2]int{r, c})

	_, _, ok = s.positionAfter(1, 2)
	assert.False(t, ok, "no position after the document end")
}

func TestSelectable_OwnsSelectionHandles(t *testing.T) {
	test.NewTempApp(t)

	l := NewLabel("hello")
	l.Selectable = true
	test.NewTempWindow(t, l)
	assert.True(t, l.selection.ownsSelectionHandles(), "a standalone selectable owns its handles")

	e := NewEntry()
	test.NewTempWindow(t, e)
	e.syncSelectable()
	assert.False(t, e.sel.ownsSelectionHandles(), "Entry's embedded selectable defers to the Entry")

	re := NewRichEntry()
	test.NewTempWindow(t, re)
	re.syncSel()
	assert.False(t, re.sel.ownsSelectionHandles(), "RichEntry's embedded selectable defers to its host")
}
