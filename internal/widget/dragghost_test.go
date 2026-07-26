package widget

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/driver"
)

func TestDragGhost_PassesInputThrough(t *testing.T) {
	g := NewDragGhost(canvas.NewRectangle(color.White), geo.NewPos(0, 0))
	// It must be input-transparent so drag/drop hit-tests fall through to the UI beneath.
	var pt driver.InputPassThrough = g
	assert.True(t, pt.PassesInputThrough())
}

func TestDragGhost_LayoutPositionsContent(t *testing.T) {
	content := canvas.NewRectangle(color.White)
	content.SetMinSize(geo.NewSize(20, 10))
	g := NewDragGhost(content, geo.NewPos(5, 7))

	r := g.CreateRenderer()
	r.Layout(geo.Size{})
	assert.Equal(t, geo.NewPos(5, 7), content.Position(), "content painted at the ghost position")
	assert.Equal(t, content.MinSize(), content.Size(), "content sized to its MinSize")
	assert.Equal(t, []core.CanvasObject{content}, r.Objects())
	assert.Equal(t, geo.Size{}, r.MinSize(), "ghost root spans nothing")
}

func TestDragGhost_SetPositionFollows(t *testing.T) {
	content := canvas.NewRectangle(color.White)
	content.SetMinSize(geo.NewSize(20, 10))
	g := NewDragGhost(content, geo.NewPos(0, 0))

	g.SetPosition(geo.NewPos(30, 40))
	r := g.CreateRenderer()
	r.Layout(geo.Size{})
	assert.Equal(t, geo.NewPos(30, 40), content.Position(), "content follows the updated position")
}
