package widget

import (
	"image/color"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	iwidget "github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/theme"
)

var _ core.Widget = (*ScrollIndicator)(nil)

// ScrollIndicator is a transparent overlay that shows the current scroll
// position within a Scroll container. It is designed to be placed on top of
// the scroll area (e.g. via a Stack/max layout) and renders a semi-transparent
// minimap in the corner showing the viewport position within the total content.
//
// Attach it to a Scroll container using NewScrollIndicator, which automatically
// registers a scroll listener (via Scroll.AddScrollListener) to stay in sync.
//
// Since: 1.0
type ScrollIndicator struct {
	BaseWidget

	Scroll *iwidget.Scroll

	// Placement controls which corner the minimap appears in.
	// Defaults to BottomRight.
	Placement ScrollIndicatorPlacement

	removeListener func()

	// Cached state
	contentSize geo.Size
	viewSize    geo.Size
	offset      geo.Position
}

// AccessibleRole returns [core.RoleScrollBar].
func (si *ScrollIndicator) AccessibleRole() core.AccessibleRole { return core.RoleScrollBar }

// AccessibleLabel returns an empty string.
func (si *ScrollIndicator) AccessibleLabel() string { return "" }

// ScrollIndicatorPlacement controls the corner of the minimap overlay.
type ScrollIndicatorPlacement int

const (
	// ScrollIndicatorBottomRight places the indicator in the bottom-right corner.
	ScrollIndicatorBottomRight ScrollIndicatorPlacement = iota
	// ScrollIndicatorBottomLeft places the indicator in the bottom-left corner.
	ScrollIndicatorBottomLeft
	// ScrollIndicatorTopRight places the indicator in the top-right corner.
	ScrollIndicatorTopRight
	// ScrollIndicatorTopLeft places the indicator in the top-left corner.
	ScrollIndicatorTopLeft
)

const (
	indicatorWidth  = float32(48)
	indicatorHeight = float32(36)
	indicatorMargin = float32(6)
)

// NewScrollIndicator creates a scroll indicator overlay attached to the given Scroll.
// Place it on top of the scroll using a Stack layout:
//
//	container.NewStack(scroll, widget.NewScrollIndicator(scroll))
//
// Since: 1.0
func NewScrollIndicator(scroll *iwidget.Scroll) *ScrollIndicator {
	si := &ScrollIndicator{
		Scroll:    scroll,
		Placement: ScrollIndicatorBottomRight,
	}
	si.ExtendBaseWidget(si)
	si.attach()
	return si
}

// MinSize returns zero — the overlay stretches to fill its parent stack.
func (si *ScrollIndicator) MinSize() geo.Size {
	return geo.NewSize(0, 0)
}

// Refresh updates the indicator to reflect the current scroll state.
func (si *ScrollIndicator) Refresh() {
	si.updateState()
	si.BaseWidget.Refresh()
}

// CreateRenderer returns the renderer for the scroll indicator overlay.
func (si *ScrollIndicator) CreateRenderer() core.WidgetRenderer {
	si.ExtendBaseWidget(si)
	si.attach() // re-attach after a previous Destroy (e.g. hide/show cycle)
	si.updateState()

	bg := canvas.NewRectangle(color.Transparent)
	bg.CornerRadius = 4

	viewport := canvas.NewRectangle(color.Transparent)
	viewport.CornerRadius = 3

	return &scrollIndicatorRenderer{
		si:       si,
		bg:       bg,
		viewport: viewport,
	}
}

func (si *ScrollIndicator) attach() {
	if si.Scroll == nil || si.removeListener != nil {
		return
	}
	si.removeListener = si.Scroll.AddScrollListener(func(geo.Position) {
		si.Refresh()
	})
}

// detach removes the indicator's scroll listener, releasing the reference the
// Scroll holds to it. Without it the listener keeps a detached indicator alive —
// and refreshing on every scroll — for the whole lifetime of the (longer-lived)
// Scroll. AddScrollListener removals are order-independent, so this is safe even
// when other consumers observe the same Scroll.
func (si *ScrollIndicator) detach() {
	if si.removeListener == nil {
		return
	}
	si.removeListener()
	si.removeListener = nil
}

func (si *ScrollIndicator) updateState() {
	if si.Scroll == nil {
		return
	}
	si.offset = si.Scroll.Offset
	si.viewSize = si.Scroll.Size()
	if si.Scroll.Content != nil {
		si.contentSize = si.Scroll.Content.MinSize()
	}
}

// --- scrollIndicatorRenderer ---

type scrollIndicatorRenderer struct {
	si       *ScrollIndicator
	bg       *canvas.Rectangle
	viewport *canvas.Rectangle
}

// Destroy detaches from the Scroll's OnScrolled chain so the indicator can be
// released once it is removed from the canvas (see ScrollIndicator.detach).
func (r *scrollIndicatorRenderer) Destroy() {
	r.si.detach()
}

func (r *scrollIndicatorRenderer) Layout(size geo.Size) {
	r.positionIndicator(size)
}

func (r *scrollIndicatorRenderer) MinSize() geo.Size {
	return geo.NewSize(0, 0)
}

func (r *scrollIndicatorRenderer) Objects() []core.CanvasObject {
	return []core.CanvasObject{r.bg, r.viewport}
}

func (r *scrollIndicatorRenderer) Refresh() {
	th := r.si.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()

	bgColor := th.Color(theme.ColorNameOverlayBackground, v)
	r.bg.FillColor = withAlpha(bgColor, 160)
	r.bg.StrokeColor = withAlpha(th.Color(theme.ColorNameSeparator, v), 100)
	r.bg.StrokeWidth = 1
	r.bg.Refresh()

	vpColor := th.Color(theme.ColorNamePrimary, v)
	r.viewport.FillColor = withAlpha(vpColor, 140)
	r.viewport.Refresh()

	r.positionIndicator(r.si.Size())
}

func (r *scrollIndicatorRenderer) positionIndicator(parentSize geo.Size) {
	si := r.si
	w := indicatorWidth
	h := indicatorHeight
	m := indicatorMargin

	// Don't show if scroll content fits entirely
	if si.contentSize.Width <= si.viewSize.Width && si.contentSize.Height <= si.viewSize.Height {
		r.bg.Hide()
		r.viewport.Hide()
		return
	}
	r.bg.Show()
	r.viewport.Show()

	// Position the minimap box in the chosen corner
	var x, y float32
	switch si.Placement {
	case ScrollIndicatorTopLeft:
		x, y = m, m
	case ScrollIndicatorTopRight:
		x, y = parentSize.Width-w-m, m
	case ScrollIndicatorBottomLeft:
		x, y = m, parentSize.Height-h-m
	default: // BottomRight
		x, y = parentSize.Width-w-m, parentSize.Height-h-m
	}

	r.bg.Move(geo.NewPos(x, y))
	r.bg.Resize(geo.NewSize(w, h))

	// Compute viewport rect within the minimap
	cw := si.contentSize.Width
	ch := si.contentSize.Height
	if cw <= 0 {
		cw = 1
	}
	if ch <= 0 {
		ch = 1
	}

	vw := si.viewSize.Width / cw
	vh := si.viewSize.Height / ch
	if vw > 1 {
		vw = 1
	}
	if vh > 1 {
		vh = 1
	}

	ox := si.offset.X / cw
	oy := si.offset.Y / ch
	if ox < 0 {
		ox = 0
	}
	if oy < 0 {
		oy = 0
	}

	pad := float32(2)
	innerW := w - pad*2
	innerH := h - pad*2

	vpX := x + pad + ox*innerW
	vpY := y + pad + oy*innerH
	vpW := vw * innerW
	vpH := vh * innerH

	// Clamp
	if vpX+vpW > x+w-pad {
		vpX = x + w - pad - vpW
	}
	if vpY+vpH > y+h-pad {
		vpY = y + h - pad - vpH
	}

	r.viewport.Move(geo.NewPos(vpX, vpY))
	r.viewport.Resize(geo.NewSize(vpW, vpH))
}

// withAlpha returns a copy of the color with the given alpha value.
func withAlpha(c color.Color, a uint8) color.Color {
	r, g, b, _ := c.RGBA()
	return color.NRGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: a}
}
