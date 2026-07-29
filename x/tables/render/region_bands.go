package render

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
)

// RegionLayoutAccess is the geometry a horizontally-scrolling region row (the
// column header, the floating-filter row) needs to place its columns in the
// body's three bands: pinned-left fixed, main translated by -ScrollOffsetX and
// clipped, pinned-right overlay. It is the single source of the band anchors —
// the same ones dataGridRenderer.doLayout / DataGrid.hitRegions use for the body
// — so every region row lays columns out where the body draws them.
type RegionLayoutAccess interface {
	HeaderColumnCount() int
	HeaderColumnWidths() []float32
	CellPadding() float32
	HeaderRegions() (mainX, pinnedRightX, pinnedRightW float32)
	ScrollOffsetX() float32
	PinnedLeftCount() int
	PinnedRightCount() int
}

// placeColumnFunc positions the decorations/content of absolute display column
// absCol at X x — widget-space for the fixed pinned bands, clip-local for the
// scrolling main band — sizing content to width w, and appends the produced
// objects to dst, returning the extended slice. It is the only part that differs
// between region rows (pooled canvas text/lines for the header, a widget.Entry
// for the floating filter).
type placeColumnFunc func(absCol int, x, w float32, dst []core.CanvasObject) []core.CanvasObject

// regionBands owns the reusable clip for a region row's scrolling main band so
// the header and floating-filter renderers share one implementation of the
// pinned-left / clipped-main / pinned-right geometry. Embed it in a renderer and
// call layout from that renderer's rebuild.
type regionBands struct {
	mainBand *bellui.Container // clip-local content of the scrolling main band
	mainClip *container.Clip // clips the main band to [mainX, mainRight)
}

// layout places every visible column into its band via place and returns the
// renderer's full object list: prefix objects (drawn behind, e.g. a header
// background) first, then the clipped main band, then the fixed pinned bands on
// top. The main band is wrapped in a real clip so its content can neither render
// nor receive pointer events outside [mainX, mainRight) — mainRight being the
// pinned-right overlay's left edge, or the row's right edge when there is no
// pinned-right band.
func (rb *regionBands) layout(g RegionLayoutAccess, size geo.Size, place placeColumnFunc, prefix ...core.CanvasObject) []core.CanvasObject {
	colCount := g.HeaderColumnCount()
	widths := g.HeaderColumnWidths()
	padding := g.CellPadding()
	mainX, pinnedRightX, pinnedRightW := g.HeaderRegions()
	offsetX := g.ScrollOffsetX()
	plCount := g.PinnedLeftCount()
	prCount := g.PinnedRightCount()
	mainCount := colCount - plCount - prCount
	if mainCount < 0 {
		mainCount = 0
	}

	widthOf := func(i int) float32 {
		if i < len(widths) {
			return widths[i]
		}
		return 100
	}

	// Main band: clipped to [mainX, mainRight), content in clip-local coordinates
	// (column content-x from 0, translated by -offsetX).
	mainRight := size.Width
	if prCount > 0 && pinnedRightW > 0 {
		mainRight = pinnedRightX
	}
	clipW := mainRight - mainX
	if clipW < 0 {
		clipW = 0
	}
	var mainObjs []core.CanvasObject
	x := float32(0)
	for i := plCount; i < plCount+mainCount; i++ {
		mainObjs = place(i, x-offsetX, widthOf(i), mainObjs)
		x += widthOf(i) + padding
	}
	if rb.mainBand == nil {
		rb.mainBand = container.NewWithoutLayout()
	}
	rb.mainBand.Objects = mainObjs
	if rb.mainClip == nil {
		rb.mainClip = container.NewClip(rb.mainBand)
	}
	rb.mainClip.Move(geo.NewPos(mainX, 0))
	rb.mainClip.Resize(geo.NewSize(clipW, size.Height))
	rb.mainClip.Refresh()

	objects := make([]core.CanvasObject, 0, len(prefix)+1+colCount*4)
	objects = append(objects, prefix...)
	objects = append(objects, rb.mainClip)

	// Pinned-left band: fixed at [0, mainX). No occluder needed — the clip keeps
	// the main band out of this region.
	if plCount > 0 {
		x := float32(0)
		for i := 0; i < plCount; i++ {
			objects = place(i, x, widthOf(i), objects)
			x += widthOf(i) + padding
		}
	}

	// Pinned-right band: fixed overlay at [pinnedRightX, size.Width).
	if prCount > 0 && pinnedRightW > 0 {
		x := pinnedRightX
		for i := plCount + mainCount; i < colCount; i++ {
			objects = place(i, x, widthOf(i), objects)
			x += widthOf(i) + padding
		}
	}

	return objects
}
