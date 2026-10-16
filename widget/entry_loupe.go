package widget

import (
	"strings"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/theme"
)

// Mobile magnifier/loupe (pre-rebrand tracker #942, a pre-rebrand tracker #758 follow-up): while a finger drags a
// selection handle (or the pre-rebrand tracker #943 caret handle) it hides exactly the text
// being targeted, so a small bubble above the dragged line shows a magnified
// view of the row around the target position. The content is the host's own
// row text re-typeset at zoom × the text size; shaped advances scale
// linearly with the size, so geometry (caret marker, selection highlight)
// derived from the host's own 1× caret measurement — bidi-exact — lands on
// the zoomed glyphs. This is the issue's "re-render" option realized in
// retained mode: no per-frame image render or framebuffer readback, so
// every painter (WebGPU, GL, software) supports it and the only per-frame
// cost is moving canvas primitives.

const (
	// loupeZoom is the magnification factor (native magnifiers use ~1.25–1.8).
	loupeZoom = 1.5
	// loupeWidth is the bubble's preferred width in dp, clamped to the canvas.
	loupeWidth = 160
	// loupeGap separates the bubble from the dragged line, in dp.
	loupeGap = 8
)

// selectionLoupeHost is the optional extension of selectionHandleHost a text
// widget implements to enable the drag magnifier. A host without it simply
// gets no loupe.
type selectionLoupeHost interface {
	selectionHandleHost

	// loupeRowText is the text of one row, as drawn.
	loupeRowText(row int) string
	// loupeRowVisualX is the visual x offset of a column within the drawn row
	// text at 1×, relative to the row text's left edge (bidi-aware).
	loupeRowVisualX(row, col int) float32
	// loupeTextStyle is the style the row is drawn with.
	loupeTextStyle() bellui.TextStyle
}

// selectionLoupeStyledHost is the further extension for hosts whose rows mix
// styles and sizes (the RichText-backed selectable and RichEntry, pre-rebrand tracker #888): the
// magnified row copy renders as multi-styled runs (canvas.StyledText) instead
// of one plain canvas.Text, so bold/heading runs keep their drawn widths and
// the 1×-measured caret x still lands on the zoomed glyphs. loupeRowRuns
// returns the row's runs at their 1× sizes (the loupe applies the zoom); a
// nil result falls back to the plain single-style path.
type selectionLoupeStyledHost interface {
	selectionLoupeHost

	loupeRowRuns(row int) []canvas.TextRun
}

// loupeOwner is the overlay hosting a loupe: the selection-handles overlay
// and the caret-handle overlay (pre-rebrand tracker #943) both qualify. It supplies the drag
// target and the highlighted range in the host's (row, column) space.
type loupeOwner interface {
	core.CanvasObject

	loupeHost() selectionLoupeHost
	// loupeClearance is the height of the grab affordance hanging below the
	// line, cleared when the bubble flips below near the canvas top.
	loupeClearance() float32
	// loupeTarget is the dragged position; ok is false while no drag is live.
	loupeTarget() (row, col int, ok bool)
	// loupeSelection is the selected range to highlight; ok false for none.
	loupeSelection() (rowA, colA, rowB, colB int, ok bool)
}

// entryLoupe is the magnifier bubble, a child of its owner overlay. It takes
// no input (nothing on it is tappable or touchable, so the pass-through
// overlay lets touches fall through it like everywhere else).
type entryLoupe struct {
	BaseWidget

	owner  loupeOwner
	active bool // a handle drag is in progress

	bg        *canvas.Rectangle
	highlight *canvas.Rectangle
	text      *canvas.Text
	styled    *canvas.StyledText // the multi-styled row copy (styled hosts, pre-rebrand tracker #888)
	caret     *canvas.Rectangle

	lastRow  int // row the text/styled object currently shows
	lastText string
}

// newEntryLoupe builds the bubble for an owner overlay, or returns nil when
// the owner's host does not implement selectionLoupeHost.
func newEntryLoupe(owner loupeOwner) *entryLoupe {
	host := owner.loupeHost()
	if host == nil {
		return nil
	}
	l := &entryLoupe{owner: owner, lastRow: -1}
	th := host.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()

	l.bg = canvas.NewRectangle(th.Color(theme.ColorNameInputBackground, v))
	l.bg.StrokeColor = th.Color(theme.ColorNameInputBorder, v)
	l.bg.StrokeWidth = th.Size(theme.SizeNameInputBorder)
	l.bg.CornerRadius = th.Size(theme.SizeNameShapeMedium)

	l.highlight = canvas.NewRectangle(th.Color(theme.ColorNameSelection, v))

	// The magnified row is typeset at a zoomed font size rather than scaled
	// with a Transform: go-text shaping scales advances linearly with the
	// size, so (1× visual x) × zoom still lands on the zoomed glyphs, and
	// untransformed objects compose with the mobile driver's dirty-region
	// culling, which culls by untransformed bounds (pre-rebrand tracker #945 device finding —
	// a transformed text was culled/ghosted mid-drag).
	l.text = canvas.NewText("", th.Color(theme.ColorNameForeground, v))
	l.styled = canvas.NewStyledText(nil, th.Color(theme.ColorNameForeground, v))
	l.styled.Hide()

	l.caret = canvas.NewRectangle(th.Color(theme.ColorNamePrimary, v))

	l.ExtendBaseWidget(l)
	l.Hide()
	return l
}

func (l *entryLoupe) CreateRenderer() core.WidgetRenderer {
	return &entryLoupeRenderer{l: l}
}

// show activates the magnifier for the duration of a drag and places it
// immediately; while active the owner's per-frame sync keeps it anchored
// through auto-scrolls and parent movement.
func (l *entryLoupe) show() {
	if l == nil {
		return
	}
	l.active = true
	l.sync()
}

// dismiss deactivates the magnifier when the drag ends or is cancelled.
func (l *entryLoupe) dismiss() {
	if l == nil || !l.active {
		return
	}
	l.active = false
	l.Hide()
}

// sync places the magnifier over the owner's current drag target. Called
// from the owner's per-frame geometry sync so it re-anchors every frame
// while shown.
func (l *entryLoupe) sync() {
	if l == nil || !l.active {
		return
	}
	host := l.owner.loupeHost()
	row, col, ok := l.owner.loupeTarget()
	if !ok {
		l.Hide()
		return
	}

	rect := host.selectionHandleCaretRect(row, col)
	anchor := rect.Pos.Subtract(host.selectionHandleScroll()) // host-relative caret top
	hostObj := host.selectionHandleObject()
	hostSize := hostObj.Size()
	if anchor.Y+rect.Size.Height <= 0 || anchor.Y >= hostSize.Height ||
		anchor.X < 0 || anchor.X > hostSize.Width {
		l.Hide() // target scrolled out of the viewport
		return
	}

	hostOrigin := hostAbsolutePosition(hostObj).Subtract(l.owner.Position())
	l.place(row, col, rect.Size.Height, hostOrigin.Add(anchor))
}

// place positions the bubble above (or, near the canvas top, below) the
// dragged line and lays out the magnified row copy inside it so the caret
// marker sits over the drag target. caretTop is the caret's top corner in
// overlay coordinates; lineHeight is the 1× row height.
func (l *entryLoupe) place(row, col int, lineHeight float32, caretTop geo.Position) {
	host := l.owner.loupeHost()
	th := host.Theme()
	pad := th.Size(theme.SizeNameInnerPadding)
	margin := th.Size(theme.SizeNamePadding)
	overlaySize := l.owner.Size()

	w := geo.Min(loupeWidth, overlaySize.Width-2*margin)
	hgt := lineHeight*loupeZoom + 2*pad

	// Horizontal: centered on the caret, clamped to the canvas. The caret
	// marker is placed against the caret's true x afterwards, so clamping
	// near an edge keeps the marker truthful instead of centered.
	x := caretTop.X - w/2
	x = geo.Max(x, margin)
	x = geo.Min(x, overlaySize.Width-margin-w)

	// Vertical: above the line; when that leaves the canvas, flip below the
	// grab affordance (the finger) instead.
	y := caretTop.Y - loupeGap - hgt
	if y < 0 {
		y = caretTop.Y + lineHeight + l.owner.loupeClearance() + loupeGap
	}
	y = geo.Min(y, overlaySize.Height-hgt)

	pos, size := geo.NewPos(x, y), geo.NewSize(w, hgt)
	if l.Position() != pos {
		l.Move(pos)
	}
	if l.Size() != size {
		l.Resize(size)
	}

	// Magnified row copy: typeset at zoom × the host's text size(s). Shaped
	// advances scale linearly with the size, so every in-loupe x is
	// (1× visual x) × zoom against the zoomed glyphs.
	markerX := caretTop.X - x
	caretInText := host.loupeRowVisualX(row, col)
	textX := markerX - caretInText*loupeZoom
	rowText := host.loupeRowText(row)
	if runs := l.zoomedRowRuns(row); runs != nil {
		// A styled host's row copy renders as multi-styled runs, each at its
		// own zoomed size, so mixed bold/heading rows keep their drawn widths.
		if rowText != l.lastText || row != l.lastRow || !l.styled.Visible() {
			l.lastText, l.lastRow = rowText, row
			l.styled.Runs = runs
			l.styled.Refresh()
		}
		l.text.Hide()
		l.styled.Resize(l.styled.MinSize())
		l.styled.Move(geo.NewPos(textX, pad))
		l.styled.Show()
	} else {
		style := host.loupeTextStyle()
		zoomedSize := th.Size(theme.SizeNameText) * loupeZoom
		if rowText != l.lastText || row != l.lastRow ||
			l.text.TextSize != zoomedSize || l.text.TextStyle != style {
			l.lastText, l.lastRow = rowText, row
			l.text.Text = rowText
			l.text.TextSize = zoomedSize
			l.text.TextStyle = style
			l.text.Refresh()
		}
		l.styled.Hide()
		l.text.Resize(bellui.MeasureText(rowText, zoomedSize, style))
		l.text.Move(geo.NewPos(textX, pad))
		l.text.Show()
	}

	// Selection highlight for the stretch of this row inside the selection
	// (for mixed-bidi rows this is the min/max span, an approximation the
	// dragged edge's caret marker keeps honest).
	l.placeHighlight(row, textX, pad, lineHeight*loupeZoom)

	// The caret marker rides over the drag target.
	caretW := float32(2) * loupeZoom
	l.caret.Move(geo.NewPos(markerX-caretW/2, pad))
	l.caret.Resize(geo.NewSize(caretW, lineHeight*loupeZoom))

	if !l.Visible() {
		l.Show()
	}
}

// zoomedRowRuns returns the magnified row copy as styled runs when the host
// is a selectionLoupeStyledHost, with each run's 1× size scaled by the zoom
// (shaped advances scale linearly with the size, the same premise as the
// plain path). nil selects the plain single-style path.
func (l *entryLoupe) zoomedRowRuns(row int) []canvas.TextRun {
	sh, ok := l.owner.loupeHost().(selectionLoupeStyledHost)
	if !ok {
		return nil
	}
	runs := sh.loupeRowRuns(row)
	for i := range runs {
		runs[i].TextSize *= loupeZoom
	}
	return runs
}

// placeHighlight sizes the selection-highlight bar for the part of row that
// is selected; textX/textY are the magnified row copy's origin.
func (l *entryLoupe) placeHighlight(row int, textX, textY, zoomedHeight float32) {
	host := l.owner.loupeHost()
	rowA, colA, rowB, colB, ok := l.owner.loupeSelection()
	if !ok || row < rowA || row > rowB || (rowA == rowB && colA == colB) {
		l.highlight.Hide()
		return
	}

	x1 := float32(0)
	if rowA == row {
		x1 = host.loupeRowVisualX(row, colA)
	}
	x2 := host.loupeRowVisualX(row, len([]rune(host.loupeRowText(row))))
	if rowB == row {
		x2 = host.loupeRowVisualX(row, colB)
	}
	if x2 < x1 {
		x1, x2 = x2, x1
	}

	l.highlight.Move(geo.NewPos(textX+x1*loupeZoom, textY))
	l.highlight.Resize(geo.NewSize((x2-x1)*loupeZoom, zoomedHeight))
	if !l.highlight.Visible() {
		l.highlight.Show()
	}
}

type entryLoupeRenderer struct {
	l *entryLoupe
}

func (r *entryLoupeRenderer) Destroy() {}

// IsClip clips the magnified row copy to the bubble's bounds on every driver
// (the rectangular scissor seam the tree walker honors).
func (r *entryLoupeRenderer) IsClip() {}

// RoundedClipCornerRadius rounds that clip to the bubble's corners where the
// painter supports stencil clipping (the Card/Entry seam).
func (r *entryLoupeRenderer) RoundedClipCornerRadius() float32 {
	return r.l.bg.CornerRadius
}

func (r *entryLoupeRenderer) Layout(size geo.Size) {
	r.l.bg.Resize(size)
	// text/highlight/caret geometry is driven by place(), not by layout
}

func (r *entryLoupeRenderer) MinSize() geo.Size {
	return geo.Size{}
}

func (r *entryLoupeRenderer) Objects() []core.CanvasObject {
	return []core.CanvasObject{r.l.bg, r.l.highlight, r.l.text, r.l.styled, r.l.caret}
}

func (r *entryLoupeRenderer) Refresh() {
	l := r.l
	th := l.owner.loupeHost().Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()
	l.bg.FillColor = th.Color(theme.ColorNameInputBackground, v)
	l.bg.StrokeColor = th.Color(theme.ColorNameInputBorder, v)
	l.bg.StrokeWidth = th.Size(theme.SizeNameInputBorder)
	l.bg.CornerRadius = th.Size(theme.SizeNameShapeMedium)
	l.highlight.FillColor = th.Color(theme.ColorNameSelection, v)
	l.text.Color = th.Color(theme.ColorNameForeground, v)
	l.styled.Color = th.Color(theme.ColorNameForeground, v)
	l.caret.FillColor = th.Color(theme.ColorNamePrimary, v)
	l.bg.Refresh()
	l.highlight.Refresh()
	l.text.Refresh()
	l.styled.Refresh()
	l.caret.Refresh()
}

// Entry's selectionLoupeHost implementation.

var _ selectionLoupeHost = (*Entry)(nil)

func (e *Entry) loupeRowText(row int) string {
	return string(e.textProvider().row(row))
}

// loupeRowVisualX maps a column to its visual x within the drawn row text at
// 1×. lineSizeToColumn reports innerPad-inset plus the in-row visual x (its
// bidi path maps logical columns through the shared visual layout), so
// stripping that constant yields coordinates directly usable inside the
// loupe's text copy.
func (e *Entry) loupeRowVisualX(row, col int) float32 {
	prov := e.textProvider()
	th := e.Theme()
	innerPad := th.Size(theme.SizeNameInnerPadding)
	textSize := th.Size(theme.SizeNameText)
	return prov.lineSizeToColumn(col, row, textSize, innerPad, true).Width - innerPad + prov.inset.Width
}

func (e *Entry) loupeTextStyle() bellui.TextStyle {
	return e.TextStyle
}

// Shared provider-backed loupe geometry for the RichText-hosted surfaces (the
// standalone selectable and RichEntry, pre-rebrand tracker #888).

// loupeProviderVisualX maps a column to its visual x within a provider row's
// drawn text at 1×, stripping the innerPad-inset baseline lineSizeToColumn
// and inlineColumnX both report against (the same convention as Entry's
// loupeRowVisualX). Rows flowing inline image atoms use the atom-aware walk,
// so the mapping matches the caret/selection geometry.
func loupeProviderVisualX(prov *RichText, row, col int, th bellui.Theme, sizeName bellui.ThemeSizeName) float32 {
	innerPad := th.Size(theme.SizeNameInnerPadding)
	textSize := th.Size(sizeName)
	if x, ok := prov.inlineColumnX(row, col, textSize, innerPad); ok {
		return x - innerPad + prov.inset.Width
	}
	return prov.lineSizeToColumn(col, row, textSize, innerPad, true).Width - innerPad + prov.inset.Width
}

// loupeRunsForRow renders one provider row as styled 1× runs for the loupe's
// magnified copy, each text segment in its own style, size and color —
// measured the same way lineSizeToColumn measures the row, so the 1× visual
// x × zoom lands on the zoomed glyphs. Non-text atoms (inline images) are
// not part of the copy; the caret marker stays truthful to the real edge.
func loupeRunsForRow(prov *RichText, row int) []canvas.TextRun {
	bound := prov.rowBoundary(row)
	if bound == nil {
		return nil
	}

	if len(bound.segments) == 1 {
		ts, ok := bound.segments[0].(*TextSegment)
		if !ok {
			return nil
		}
		// The lazy-aware row read: never decode the whole document for one row.
		text := string(prov.row(row))
		if concealed(ts) {
			text = strings.Repeat(passwordChar, len([]rune(text)))
		}
		if text == "" {
			return nil
		}
		return []canvas.TextRun{{Text: text, Color: ts.color(), TextSize: ts.size(), TextStyle: ts.Style.TextStyle}}
	}

	// Multi-segment rows are never lazy, so per-segment slicing is safe (the
	// same begin/end trimming as RichText.row).
	runs := make([]canvas.TextRun, 0, len(bound.segments))
	nSegs := len(bound.segments)
	for i, seg := range bound.segments {
		ts, ok := seg.(*TextSegment)
		if !ok {
			continue // atoms are not part of the text copy
		}
		runes := []rune(ts.Text)
		start, end := 0, len(runes)
		if i == 0 {
			start = bound.begin
		}
		if i == nSegs-1 && bound.end <= len(runes) && (bound.end > 0 || bound.begin >= bound.end) {
			end = bound.end
		}
		if start > end {
			start = end
		}
		text := string(runes[start:end])
		if concealed(seg) {
			text = strings.Repeat(passwordChar, end-start)
		}
		if text == "" {
			continue
		}
		runs = append(runs, canvas.TextRun{Text: text, Color: ts.color(), TextSize: ts.size(), TextStyle: ts.Style.TextStyle})
	}
	if len(runs) == 0 {
		return nil
	}
	return runs
}
