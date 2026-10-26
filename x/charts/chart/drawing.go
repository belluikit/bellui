package chart

import (
	"image/color"

	"github.com/belluikit/bellui"

	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/x/charts/interact"
)

// Default drawing colors — cycled per new drawing.
var drawingColors = []color.Color{
	color.NRGBA{R: 91, G: 143, B: 249, A: 220},  // blue
	color.NRGBA{R: 255, G: 99, B: 132, A: 220},  // red
	color.NRGBA{R: 50, G: 205, B: 50, A: 220},   // green
	color.NRGBA{R: 255, G: 180, B: 0, A: 220},   // orange
	color.NRGBA{R: 200, G: 100, B: 255, A: 220}, // purple
	color.NRGBA{R: 0, G: 200, B: 200, A: 220},   // cyan
}

// --- Public API ---

// SetDrawingTool activates a drawing tool. Pass DrawingToolNone to deactivate.
// A change emits interact.EventDrawingToolChanged, which keeps a drawing-tools
// UI in sync (DrawingToolbar, or an app's own).
func (c *Chart) SetDrawingTool(tool interact.DrawingToolMode) {
	c.mu.Lock()
	// Finalize any in-progress text label before switching tools.
	if c.drawingGesture.Active && c.drawingGesture.Tool == interact.DrawingToolTextLabel && c.drawingGesture.TextBuf != "" {
		c.finalizeTextLabel()
	}
	previous := c.drawingTool
	c.drawingTool = tool
	c.drawingGesture.Reset()
	c.mu.Unlock()
	c.refreshDirty(dirtyFull)
	if isMobile() {
		c.syncMobileFocus() // no text input is pending any more
	}
	if tool != previous {
		c.Events.Emit(interact.DrawingToolEvent{
			Event:    interact.Event{Type: interact.EventDrawingToolChanged},
			Tool:     tool,
			Previous: previous,
		})
	}
}

// DrawingTool returns the currently active drawing tool.
func (c *Chart) DrawingTool() interact.DrawingToolMode {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.drawingTool
}

// AddDrawing programmatically adds a drawing.
func (c *Chart) AddDrawing(d interact.Drawing) {
	c.mu.Lock()
	c.drawingStore.Add(d)
	c.mu.Unlock()
	c.refreshDirty(dirtyFull)
}

// RemoveDrawing removes a drawing by ID.
func (c *Chart) RemoveDrawing(id interact.DrawingID) {
	c.mu.Lock()
	removed := c.drawingStore.Remove(id)
	if c.selectedID == id {
		c.selectedID = ""
	}
	c.mu.Unlock()
	if removed {
		c.Events.Emit(interact.DrawingEvent{
			Event:     interact.Event{Type: interact.EventDrawingDeleted},
			DrawingID: id,
		})
		c.refreshDirty(dirtyFull)
	}
}

// Drawings returns all current drawings.
func (c *Chart) Drawings() []interact.Drawing {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.drawingStore.All()
}

// ClearDrawings removes all drawings.
func (c *Chart) ClearDrawings() {
	c.mu.Lock()
	c.drawingStore.Clear()
	c.selectedID = ""
	c.mu.Unlock()
	c.refreshDirty(dirtyFull)
}

// clearAllDrawings is the drawing tools' Clear All: it removes every drawing
// and goes back to Select. A drawing still in progress is dropped first, or
// going back to Select would finish a label being typed into the emptied chart.
func (c *Chart) clearAllDrawings() {
	c.mu.Lock()
	c.drawingGesture.Reset()
	c.mu.Unlock()
	c.ClearDrawings()
	c.SetDrawingTool(interact.DrawingToolNone)
}

// SelectedDrawing returns the currently selected drawing ID, or "".
func (c *Chart) SelectedDrawing() interact.DrawingID {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.selectedID
}

// SelectDrawing programmatically selects a drawing.
func (c *Chart) SelectDrawing(id interact.DrawingID) {
	c.mu.Lock()
	c.selectedID = id
	c.mu.Unlock()
	c.refreshDirty(dirtyFull)
}

// --- Internal gesture handling ---

// nextDrawingColor returns a color for the next drawing.
func (c *Chart) nextDrawingColor() color.Color {
	n := c.drawingStore.Len()
	return drawingColors[n%len(drawingColors)]
}

// pixelToData converts a pixel position to data coordinates using the cached coordinate system.
// Must be called with c.mu held (at least RLock).
func (c *Chart) pixelToData(pos geo.Position) (float64, float64, bool) {
	snap := c.renderSnap
	if snap == nil || len(snap.coordSystems) == 0 {
		return 0, 0, false
	}
	x, y := snap.coordSystems[0].PixelToData(pos)
	return x, y, true
}

// handleDrawingTap processes a tap in drawing mode. Returns true if consumed.
func (c *Chart) handleDrawingTap(pos geo.Position) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.drawingTool == interact.DrawingToolNone {
		return false
	}

	// Measure tool doesn't use taps.
	if c.drawingTool == interact.DrawingToolMeasure {
		return false
	}

	x, y, ok := c.pixelToData(pos)
	if !ok {
		return false
	}
	dp := interact.DataPoint{X: x, Y: y}

	switch c.drawingTool {
	case interact.DrawingToolHLine:
		id := c.drawingStore.NextID()
		d := &interact.HLineDrawing{DrawingBase: interact.DrawingBase{ID_: id, Color_: c.nextDrawingColor()}, Y: y}
		c.drawingStore.Add(d)
		c.selectedID = id
		c.pendingDirty |= dirtyFull
		c.emitDrawingCreated(id, d)
		return true

	case interact.DrawingToolVLine:
		id := c.drawingStore.NextID()
		d := &interact.VLineDrawing{DrawingBase: interact.DrawingBase{ID_: id, Color_: c.nextDrawingColor()}, X: x}
		c.drawingStore.Add(d)
		c.selectedID = id
		c.pendingDirty |= dirtyFull
		c.emitDrawingCreated(id, d)
		return true

	case interact.DrawingToolTrendLine, interact.DrawingToolRay, interact.DrawingToolFibRetracement:
		return c.handleTwoClickTool(dp)

	case interact.DrawingToolChannel:
		return c.handleChannelTool(dp)

	case interact.DrawingToolRectangle:
		// Rectangle uses click for first corner, then drag/click for second.
		if !c.drawingGesture.Active {
			c.drawingGesture = interact.DrawingGestureState{
				Active: true,
				Tool:   interact.DrawingToolRectangle,
				Step:   1,
				Color:  c.nextDrawingColor(),
			}
			c.drawingGesture.Points[0] = dp
			c.pendingDirty |= dirtyDrawing
			return true
		}
		// Second click finalizes.
		return c.finalizeRectangle(dp)

	case interact.DrawingToolTextLabel:
		return c.handleTextLabelTap(dp)
	}

	return false
}

// handleTwoClickTool handles TrendLine, Ray, and FibRetracement (all 2-click tools).
func (c *Chart) handleTwoClickTool(dp interact.DataPoint) bool {
	if !c.drawingGesture.Active {
		c.drawingGesture = interact.DrawingGestureState{
			Active: true,
			Tool:   c.drawingTool,
			Step:   1,
			Color:  c.nextDrawingColor(),
		}
		c.drawingGesture.Points[0] = dp
		c.pendingDirty |= dirtyDrawing
		return true
	}

	// Second click — finalize.
	c.drawingGesture.Points[1] = dp
	id := c.drawingStore.NextID()
	col := c.drawingGesture.Color

	var d interact.Drawing
	switch c.drawingGesture.Tool {
	case interact.DrawingToolTrendLine:
		d = &interact.TrendLineDrawing{
			DrawingBase: interact.DrawingBase{ID_: id, Color_: col},
			Start:       c.drawingGesture.Points[0],
			End:         dp,
		}
	case interact.DrawingToolRay:
		d = &interact.RayDrawing{
			DrawingBase: interact.DrawingBase{ID_: id, Color_: col},
			Start:       c.drawingGesture.Points[0],
			Direction:   dp,
		}
	case interact.DrawingToolFibRetracement:
		d = &interact.FibRetracementDrawing{
			DrawingBase: interact.DrawingBase{ID_: id, Color_: col},
			HighY:       c.drawingGesture.Points[0].Y,
			LowY:        dp.Y,
			X:           c.drawingGesture.Points[0].X,
		}
	}

	c.drawingStore.Add(d)
	c.selectedID = id
	c.drawingGesture.Reset()
	c.pendingDirty |= dirtyFull
	c.emitDrawingCreated(id, d)
	return true
}

// handleChannelTool handles the 3-click channel tool.
func (c *Chart) handleChannelTool(dp interact.DataPoint) bool {
	if !c.drawingGesture.Active {
		c.drawingGesture = interact.DrawingGestureState{
			Active: true,
			Tool:   interact.DrawingToolChannel,
			Step:   1,
			Color:  c.nextDrawingColor(),
		}
		c.drawingGesture.Points[0] = dp
		c.pendingDirty |= dirtyDrawing
		return true
	}

	step := c.drawingGesture.Step
	if step == 1 {
		c.drawingGesture.Points[1] = dp
		c.drawingGesture.Step = 2
		c.pendingDirty |= dirtyDrawing
		return true
	}

	// Third click — finalize with offset.
	// Compute offset relative to the primary line at the click's X position,
	// not from the start point. This ensures the parallel line appears
	// where the user clicked, even for diagonal lines.
	p0 := c.drawingGesture.Points[0]
	p1 := c.drawingGesture.Points[1]
	lineYAtClick := p0.Y
	if p1.X != p0.X {
		t := (dp.X - p0.X) / (p1.X - p0.X)
		lineYAtClick = p0.Y + t*(p1.Y-p0.Y)
	}
	offsetY := dp.Y - lineYAtClick
	id := c.drawingStore.NextID()
	d := &interact.ChannelDrawing{
		DrawingBase: interact.DrawingBase{ID_: id, Color_: c.drawingGesture.Color},
		Line1Start:  c.drawingGesture.Points[0],
		Line1End:    c.drawingGesture.Points[1],
		OffsetY:     offsetY,
	}
	c.drawingStore.Add(d)
	c.selectedID = id
	c.drawingGesture.Reset()
	c.pendingDirty |= dirtyFull
	c.emitDrawingCreated(id, d)
	return true
}

// finalizeRectangle creates a rectangle from gesture state + second corner.
func (c *Chart) finalizeRectangle(dp interact.DataPoint) bool {
	id := c.drawingStore.NextID()
	d := &interact.RectangleDrawing{
		DrawingBase: interact.DrawingBase{ID_: id, Color_: c.drawingGesture.Color},
		Corner1:     c.drawingGesture.Points[0],
		Corner2:     dp,
	}
	c.drawingStore.Add(d)
	c.selectedID = id
	c.drawingGesture.Reset()
	c.pendingDirty |= dirtyFull
	c.emitDrawingCreated(id, d)
	return true
}

// handleTextLabelTap handles placing and finalizing text labels.
func (c *Chart) handleTextLabelTap(dp interact.DataPoint) bool {
	if !c.drawingGesture.Active {
		// First click: place anchor, enter text input mode.
		c.drawingGesture = interact.DrawingGestureState{
			Active: true,
			Tool:   interact.DrawingToolTextLabel,
			Step:   1,
			Color:  c.nextDrawingColor(),
		}
		c.drawingGesture.Points[0] = dp
		c.pendingDirty |= dirtyDrawing
		return true
	}
	// Already in text input mode — second tap finalizes.
	return c.finalizeTextLabel()
}

// finalizeTextLabel creates the text label drawing from accumulated text.
func (c *Chart) finalizeTextLabel() bool {
	text := c.drawingGesture.TextBuf
	if text == "" {
		c.drawingGesture.Reset()
		c.pendingDirty |= dirtyDrawing
		return true
	}
	id := c.drawingStore.NextID()
	d := &interact.TextLabelDrawing{
		DrawingBase: interact.DrawingBase{ID_: id, Color_: c.drawingGesture.Color},
		Pos:         c.drawingGesture.Points[0],
		Text:        text,
	}
	c.drawingStore.Add(d)
	c.selectedID = id
	c.drawingGesture.Reset()
	c.pendingDirty |= dirtyFull
	c.emitDrawingCreated(id, d)
	return true
}

// handleDrawingSelect tries to select an existing drawing at the click position.
// Returns true if a drawing was hit. Must be called WITHOUT lock held.
func (c *Chart) handleDrawingSelect(pos geo.Position) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	snap := c.renderSnap
	if snap == nil || len(snap.coordSystems) == 0 {
		return false
	}

	drawings := c.drawingStore.All()
	if len(drawings) == 0 {
		return false
	}

	hit := interact.HitTestDrawings(drawings, pos, snap.coordSystems, snap.plotRect, 10)
	if hit != nil {
		c.selectedID = hit.DrawingID
		c.pendingDirty |= dirtyFull
		c.emitDrawingSelected(hit.DrawingID)
		return true
	}

	// Click on empty area deselects.
	if c.selectedID != "" {
		c.selectedID = ""
		c.pendingDirty |= dirtyFull
		return false // don't consume — let other handlers process
	}
	return false
}

// handleDrawingDelete removes the selected drawing. Returns true if consumed.
func (c *Chart) handleDrawingDelete() bool {
	c.mu.Lock()
	id := c.selectedID
	if id == "" {
		c.mu.Unlock()
		return false
	}
	c.drawingStore.Remove(id)
	c.selectedID = ""
	c.pendingDirty |= dirtyFull
	c.mu.Unlock()

	c.Events.Emit(interact.DrawingEvent{
		Event:     interact.Event{Type: interact.EventDrawingDeleted},
		DrawingID: id,
	})
	c.Refresh()
	return true
}

// handleDrawingCancel cancels the active gesture or deselects. Returns true if consumed.
func (c *Chart) handleDrawingCancel() bool {
	c.mu.Lock()
	if c.drawingGesture.Active {
		c.drawingGesture.Reset()
		c.pendingDirty |= dirtyDrawing
		c.mu.Unlock()
		c.Refresh()
		return true
	}
	if c.selectedID != "" {
		c.selectedID = ""
		c.pendingDirty |= dirtyFull
		c.mu.Unlock()
		c.Refresh()
		return true
	}
	c.mu.Unlock()
	return false
}

// handleDrawingDrag processes drag events for Measure tool and drag-to-move.
// Must be called with c.mu held.
func (c *Chart) handleDrawingDrag(e *event.DragEvent) bool {
	// Measure tool.
	if c.drawingTool == interact.DrawingToolMeasure {
		if !c.drawingGesture.MeasureActive {
			c.drawingGesture.MeasureActive = true
			c.drawingGesture.MeasureStart = e.Position
		}
		c.drawingGesture.MeasureEnd = e.Position
		c.pendingDirty |= dirtyDrawing
		return true
	}

	// Drag-to-move: start dragging a selected drawing.
	if c.selectedID != "" && c.drawingGesture.DragDrawingID == "" {
		// Check if drag starts near the selected drawing.
		snap := c.renderSnap
		if snap != nil && len(snap.coordSystems) > 0 {
			d := c.drawingStore.Get(c.selectedID)
			if d != nil {
				hit := interact.HitTestDrawings(
					[]interact.Drawing{d}, e.Position,
					snap.coordSystems, snap.plotRect, 15,
				)
				if hit != nil {
					// Start dragging.
					x, y, ok := c.pixelToData(e.Position)
					if ok {
						origPts := d.DataPoints()
						saved := make([]interact.DataPoint, len(origPts))
						copy(saved, origPts)
						c.drawingGesture.DragDrawingID = c.selectedID
						c.drawingGesture.DragOrigin = interact.DataPoint{X: x, Y: y}
						c.drawingGesture.DragOrigPts = saved
					}
				}
			}
		}
	}

	// Continue dragging: apply delta to all data points.
	if c.drawingGesture.DragDrawingID != "" {
		x, y, ok := c.pixelToData(e.Position)
		if ok {
			d := c.drawingStore.Get(c.drawingGesture.DragDrawingID)
			if d != nil {
				dx := x - c.drawingGesture.DragOrigin.X
				dy := y - c.drawingGesture.DragOrigin.Y
				newPts := make([]interact.DataPoint, len(c.drawingGesture.DragOrigPts))
				for i, pt := range c.drawingGesture.DragOrigPts {
					newPts[i] = interact.DataPoint{X: pt.X + dx, Y: pt.Y + dy}
				}
				d.SetDataPoints(newPts)
				c.pendingDirty |= dirtyFull
			}
		}
		return true
	}

	return false
}

// handleDrawingDragEnd processes drag end for Measure tool and drag-to-move.
// Must be called with c.mu held.
func (c *Chart) handleDrawingDragEnd() bool {
	if c.drawingGesture.MeasureActive {
		c.drawingGesture.MeasureActive = false
		c.pendingDirty |= dirtyDrawing
		return true
	}
	if c.drawingGesture.DragDrawingID != "" {
		c.drawingGesture.DragDrawingID = ""
		c.drawingGesture.DragOrigPts = nil
		c.pendingDirty |= dirtyFull
		return true
	}
	return false
}

// --- Event helpers (must be called without lock held for emitting) ---

func (c *Chart) emitDrawingCreated(id interact.DrawingID, d interact.Drawing) {
	// Schedule on the Fyne main thread so subscribers can safely update UI.
	bellui.Do(func() {
		c.Events.Emit(interact.DrawingEvent{
			Event:     interact.Event{Type: interact.EventDrawingCreated},
			DrawingID: id,
			Drawing:   d,
		})
	})
}

func (c *Chart) emitDrawingSelected(id interact.DrawingID) {
	bellui.Do(func() {
		c.Events.Emit(interact.DrawingEvent{
			Event:     interact.Event{Type: interact.EventDrawingSelected},
			DrawingID: id,
		})
	})
}
