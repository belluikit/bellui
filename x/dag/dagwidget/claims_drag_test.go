package dagwidget

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
)

// Each part of the canvas claims a touch drag exactly when its gesture is on
// (bellui#1266): with every capability, every part takes the drag; with none,
// only the canvas does (to pan); disabled, nothing does, so the drag reaches
// the scroll around the canvas.
func TestDAGCanvas_ClaimsDragFollowsCapabilities(t *testing.T) {
	c := fullCanvas(t)
	nw := c.nodes["a"]
	require.NotEmpty(t, nw.Outputs)
	require.NotEmpty(t, nw.handles)
	gw := c.groups["g1"]
	require.NotNil(t, gw)
	item := newPaletteItem(nil, &Palette{DAGCanvas: c})

	parts := map[string]event.DragClaimer{
		"node":            nw,
		"node connector":  nw.connector,
		"output port":     nw.Outputs[0],
		"resize handle":   nw.handles[0],
		"group title":     gw.titleBar,
		"group connector": gw.connector,
		"annotation":      c.annotations["an1"],
		"palette item":    item,
	}
	up := geo.Delta{DY: -20}

	c.Capabilities = AllCapabilities()
	assert.True(t, c.ClaimsDrag(up), "canvas")
	for name, p := range parts {
		assert.True(t, p.ClaimsDrag(up), "%s, every capability on", name)
	}

	c.Capabilities = Capabilities{}
	assert.True(t, c.ClaimsDrag(up), "the canvas still pans")
	for name, p := range parts {
		assert.False(t, p.ClaimsDrag(up), "%s, no capability on, should leave the drag to the canvas", name)
	}

	c.Capabilities = AllCapabilities()
	c.Disable()
	assert.False(t, c.ClaimsDrag(up), "a disabled canvas should leave the drag to the scroll around it")
	for name, p := range parts {
		if name == "palette item" {
			continue // the palette is a separate widget, left alone by Disable
		}
		assert.False(t, p.ClaimsDrag(up), "%s, canvas disabled", name)
	}
}

// An input port with no edge has nothing to drag: it leaves the drag to the
// canvas, while a connected one reroutes its edge.
func TestPortWidget_ClaimsDragOnlyWithSomethingToDrag(t *testing.T) {
	c := fullCanvas(t)
	c.Capabilities = AllCapabilities()
	in := c.nodes["b"].Inputs[0]
	require.True(t, in.Connected, "e1 ends at b's input")
	assert.True(t, in.ClaimsDrag(geo.Delta{DX: 5}))
	in.Connected = false
	assert.False(t, in.ClaimsDrag(geo.Delta{DX: 5}))
}
