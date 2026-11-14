package dagwidget

import (
	"image/color"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/theme"
)

var _ core.Disableable = (*DAGCanvas)(nil)

// Disable makes the canvas visible but inert (belluikit/player#641): it keeps
// its place and its graph, draws its own nodes, edges, groups, ports and
// annotations in the disabled colours, and takes no input. Taps and secondary
// taps select, activate and fire nothing; drags neither pan, move, resize nor
// connect; the wheel, a two-finger scroll and a pinch neither zoom nor pan;
// keys and shortcuts (Tab traversal, Delete, Enter, Escape, the Menu key,
// undo and redo) do nothing. The resize and connector handles are hidden.
// Tab skips the canvas, and a focus on it is dropped.
//
// A connection drag in progress is cancelled. A move or resize drag in
// progress stops moving; what it already moved is recorded on its release,
// as before, so undo stays in step with the graph.
//
// The app's own calls (AddNode, DeleteSelected, SelectNode, the Graph, the
// Commands stack) still work: Disable refuses the user's input, not the app.
// A node's Content is the app's and is left alone, and so are the Palette
// and the PropertiesPanel, which are separate widgets.
//
// Disable is safe before the canvas is drawn.
func (c *DAGCanvas) Disable() {
	if c.disabled {
		return
	}
	c.disabled = true
	c.cancelConnections()
	c.pinching = false
	if c.dragState == dragPanning {
		c.dragState = dragNone
	}
	c.dropFocus()
	c.refreshParts()
}

// Enable undoes Disable: the canvas takes input again and draws its normal
// look. A focus Disable dropped does not come back.
func (c *DAGCanvas) Enable() {
	if !c.disabled {
		return
	}
	c.disabled = false
	c.refreshParts()
}

// Disabled reports whether the canvas is disabled.
func (c *DAGCanvas) Disabled() bool {
	return c.disabled
}

// inert reports whether input to the canvas, or to a part of it, is refused.
// Nil-safe: a part built without a canvas is never inert.
func (c *DAGCanvas) inert() bool {
	return c != nil && c.disabled
}

// caps is the canvas's Capabilities as the user's gestures see them: none
// while disabled, so every capability-gated gesture is refused and every
// capability-gated affordance (resize and connector handles) hidden. The
// app's own entry points keep reading Capabilities.
func (c *DAGCanvas) caps() Capabilities {
	if c.inert() {
		return Capabilities{}
	}
	return c.Capabilities
}

// cancelConnections ends a port, group or link connection drag the way a
// drop on empty space does, so a Disable mid-drag adds no edge.
func (c *DAGCanvas) cancelConnections() {
	switch c.dragState {
	case dragConnecting:
		if c.pendingEdge != nil {
			c.pendingEdge.snapTarget = nil
		}
		c.finishConnection()
	case dragConnectingGroup:
		if c.pendingGroupLink != nil {
			c.pendingGroupLink.snapTarget = ""
		}
		c.finishGroupEdgeConnection()
	case dragConnectingLink:
		if c.pendingNodeLink != nil {
			c.pendingNodeLink.snapTarget = ""
		}
		c.finishLinkEdgeConnection()
	}
}

// dropFocus unfocuses the canvas's window when the canvas has the focus. The
// canvas is the only focusable part it builds. The toolkit's own Disable also
// drops a focus inside a widget, which an x module cannot find; a node's
// Content is the app's.
func (c *DAGCanvas) dropFocus() {
	if !c.rendered {
		return // never drawn: it cannot hold a focus
	}
	a := bellui.CurrentApp()
	if a == nil {
		return
	}
	cnv := a.Driver().CanvasForObject(c)
	if cnv == nil {
		return
	}
	if f := cnv.Focused(); f != nil && any(f) == any(c) {
		cnv.Unfocus()
	}
}

// refreshParts repaints the canvas and every element widget: a canvas
// Refresh lays its children out but does not refresh them.
func (c *DAGCanvas) refreshParts() {
	c.Refresh()
	for _, w := range c.groups {
		w.Refresh()
	}
	for _, w := range c.nodes {
		w.Refresh()
	}
	for _, w := range c.edges {
		w.Refresh()
	}
	for _, w := range c.groupEdges {
		w.Refresh()
	}
	for _, w := range c.linkEdges {
		w.Refresh()
	}
	for _, w := range c.annotations {
		w.Refresh()
	}
}

// partTheme is the theme a part of the canvas draws with: th itself, or,
// while the canvas is disabled, th with the disabled colours (see
// disabledTheme).
func partTheme(th bellui.Theme, c *DAGCanvas) bellui.Theme {
	if c.inert() {
		return disabledTheme{th}
	}
	return th
}

// disabledTheme draws the canvas's own text, strokes and accents in
// ColorNameDisabled and its card fills in ColorNameDisabledButton, as
// widget.Button draws a disabled label and fill. Backgrounds and separators
// keep their colours.
type disabledTheme struct{ bellui.Theme }

func (t disabledTheme) Color(n bellui.ThemeColorName, v bellui.ThemeVariant) color.Color {
	switch n {
	case theme.ColorNameForeground, theme.ColorNamePrimary, theme.ColorNameOutline,
		theme.ColorNamePlaceHolder:
		n = theme.ColorNameDisabled
	case theme.ColorNameButton, theme.ColorNameSelection:
		n = theme.ColorNameDisabledButton
	}
	return t.Theme.Color(n, v)
}
