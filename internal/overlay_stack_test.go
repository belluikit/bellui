package internal_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/internal"
	"github.com/belluikit/bellui/internal/app"
	intWidget "github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
)

func TestOverlayStack(t *testing.T) {
	s := &internal.OverlayStack{Canvas: test.NewCanvas()}
	o1 := widget.NewLabel("A")
	o2 := widget.NewLabel("B")
	o3 := widget.NewLabel("C")
	o4 := widget.NewLabel("D")
	o5 := widget.NewLabel("E")

	// initial empty
	assert.Empty(t, s.List())
	assert.Nil(t, s.Top())
	assert.Nil(t, s.TopFocusManager())
	assert.Empty(t, s.ListFocusManagers())

	// add one & remove
	s.Add(o1)
	assert.Equal(t, []core.CanvasObject{o1}, s.List())
	assert.Equal(t, o1, s.Top())
	fm := s.TopFocusManager()
	assert.NotNil(t, fm)
	assert.Equal(t, []*app.FocusManager{fm}, s.ListFocusManagers())
	// remove other does nothing
	s.Remove(o2)
	assert.Equal(t, []core.CanvasObject{o1}, s.List())
	assert.Equal(t, o1, s.Top())
	assert.Equal(t, fm, s.TopFocusManager())
	assert.Equal(t, []*app.FocusManager{fm}, s.ListFocusManagers())
	// remove the correct one
	s.Remove(o1)
	assert.Empty(t, s.List())
	assert.Nil(t, s.Top())
	assert.Nil(t, s.TopFocusManager())
	assert.Empty(t, s.ListFocusManagers())

	// add multiple & remove
	s.Add(o1)
	fm1 := s.TopFocusManager()
	assert.NotNil(t, fm1)
	assert.Equal(t, []*app.FocusManager{fm1}, s.ListFocusManagers())
	s.Add(o2)
	fm2 := s.TopFocusManager()
	assert.NotNil(t, fm2)
	assert.NotEqual(t, fm1, fm2)
	assert.Equal(t, []*app.FocusManager{fm1, fm2}, s.ListFocusManagers())
	s.Add(o3)
	fm3 := s.TopFocusManager()
	assert.NotNil(t, fm3)
	assert.NotEqual(t, fm2, fm3)
	assert.Equal(t, []*app.FocusManager{fm1, fm2, fm3}, s.ListFocusManagers())
	s.Add(o4)
	fm4 := s.TopFocusManager()
	assert.NotNil(t, fm4)
	assert.NotEqual(t, fm3, fm4)
	assert.Equal(t, []*app.FocusManager{fm1, fm2, fm3, fm4}, s.ListFocusManagers())
	s.Add(o5)
	assert.Equal(t, []core.CanvasObject{o1, o2, o3, o4, o5}, s.List())
	assert.Equal(t, o5, s.Top())
	fm5 := s.TopFocusManager()
	assert.NotNil(t, fm5)
	assert.NotEqual(t, fm4, fm5)
	assert.Equal(t, []*app.FocusManager{fm1, fm2, fm3, fm4, fm5}, s.ListFocusManagers())
	s.Remove(o5)
	assert.Equal(t, []core.CanvasObject{o1, o2, o3, o4}, s.List())
	assert.Equal(t, o4, s.Top())
	assert.Equal(t, fm4, s.TopFocusManager())
	assert.Equal(t, []*app.FocusManager{fm1, fm2, fm3, fm4}, s.ListFocusManagers())
	// remove cuts the stack
	s.Remove(o2)
	assert.Equal(t, []core.CanvasObject{o1}, s.List())
	assert.Equal(t, o1, s.Top())
	assert.Equal(t, fm1, s.TopFocusManager())
	assert.Equal(t, []*app.FocusManager{fm1}, s.ListFocusManagers())
	s.Remove(o1)
	assert.Empty(t, s.List())
	assert.Nil(t, s.Top())
	assert.Nil(t, s.TopFocusManager())
	assert.Empty(t, s.ListFocusManagers())
}

// TestOverlayStack_FocusTransparent pins the pre-rebrand tracker #442 seam: a focus-transparent
// OverlayContainer is skipped by TopFocusManager, so the layer beneath keeps
// keyboard focus; opaque overlays above or below participate as usual.
func TestOverlayStack_FocusTransparent(t *testing.T) {
	c := test.NewCanvas()
	s := &internal.OverlayStack{Canvas: c}

	transparent := intWidget.NewOverlayContainer(widget.NewLabel("popup"), c, nil)
	transparent.FocusTransparent = true

	s.Add(transparent)
	assert.Nil(t, s.TopFocusManager(),
		"a lone focus-transparent overlay leaves focus with the canvas content")

	opaque := widget.NewLabel("modal")
	s.Add(opaque)
	mgrs := s.ListFocusManagers()
	assert.Equal(t, mgrs[1], s.TopFocusManager(),
		"an opaque overlay above captures focus as usual")

	s.Remove(opaque)
	assert.Nil(t, s.TopFocusManager())

	below := widget.NewLabel("dialog")
	s.Remove(transparent)
	s.Add(below)
	s.Add(transparent)
	mgrs = s.ListFocusManagers()
	assert.Equal(t, mgrs[0], s.TopFocusManager(),
		"a transparent overlay on top yields to the opaque overlay beneath")
}
