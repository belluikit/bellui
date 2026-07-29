package editor

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/theme"
)

// popoverPlacement records where a popover sits relative to its anchor cell.
type popoverPlacement int

const (
	placeBelow popoverPlacement = iota // popover sits below the anchor cell
	placeAbove                         // popover sits above the anchor cell
)

// popover is the internal floating-overlay primitive anchored to a character
// cell in the editor viewport. It is the shared plumbing behind the completion,
// hover, and signature-help surfaces.
//
// Unlike Bellui's [widget.PopUp], a popover renders inside the editor's own widget
// tree (the center Stack) rather than the canvas overlay stack. That keeps the
// editor Entry focused — so the user can keep typing while the popover is open —
// and lets the popover share the Entry's viewport coordinate space, the same
// space [Editor.PositionAt] and the decorator overlay already use.
//
// The editor owns at most one active popover and drives its lifecycle:
// placement and flipping on show, repositioning on scroll and resize, and
// dismissal on Escape, anchor scroll-off, outside click, or focus loss.
type popover struct {
	// obj is the visual object placed in the editor's float layer: a no-layout
	// container holding the elevation shadow behind the body.
	obj *bellui.Container
	// body is the themed, bordered surface (background + padded content); it is
	// the part measured and sized. shadow is the soft elevation behind it.
	body    *bellui.Container
	shadow  *canvas.BoxShadow
	content core.CanvasObject

	// anchorRow, anchorCol is the document cell the popover is pinned to.
	anchorRow, anchorCol int

	// placement records the last computed side (below/above the anchor).
	placement popoverPlacement

	// interactive marks a popover that handles keys and must be the sole
	// interactive surface (completion, hover). A non-interactive popover (the
	// signature card) may be shown alongside one interactive popover.
	interactive bool
	// prefersAbove biases placement above the anchor cell instead of the default
	// below, so the signature card sits clear of a below-anchored completion list.
	prefersAbove bool

	// maxWidth, maxHeight cap the popover size when > 0. The popover is always
	// additionally clamped to the editor viewport.
	maxWidth, maxHeight float32

	// keyHandler, when set, is offered each key event while the popover is
	// visible, before the Entry handles it. It returns true if it consumed the
	// key. This is how completion intercepts Arrow/Tab/Enter without the popover
	// stealing focus from the Entry.
	keyHandler func(*event.KeyEvent) bool

	// onDismiss, when set, is called once when the popover is dismissed.
	onDismiss func()
}

// newPopover wraps content in a themed, padded, bordered background ready to
// float over the editor.
func newPopover(content core.CanvasObject) *popover {
	bg := canvas.NewRectangle(theme.Color(theme.ColorNameSurfaceContainerLow))
	bg.CornerRadius = theme.Size(theme.SizeNameShapeLarge)
	bg.StrokeColor = theme.Color(theme.ColorNameInputBorder)
	bg.StrokeWidth = theme.InputBorderSize()

	// A soft drop shadow gives the surface elevation so it reads as floating
	// above the text. canvas.BoxShadow is GPU-blurred on the GL painter (and
	// falls back to a solid translucent rect on software/test painters), so this
	// is a true soft shadow rather than the earlier flat offset block. See #72.
	shadow := canvas.NewBoxShadow(theme.Color(theme.ColorNameShadow), popoverShadowBlur)
	shadow.CornerRadius = bg.CornerRadius
	shadow.OffsetY = popoverShadowOffsetY

	body := container.NewStack(bg, container.NewPadded(content))
	return &popover{
		content: content,
		body:    body,
		shadow:  shadow,
		// No-layout container: resizeAndPlace positions the shadow and body
		// explicitly so the shadow can sit offset behind the body.
		obj: container.NewWithoutLayout(shadow, body),
	}
}

// Elevation shadow tuning (see #72): a soft Gaussian blur radius and a small
// downward offset, roughly matching a Material "popup"-level elevation.
const (
	popoverShadowBlur    = 10
	popoverShadowOffsetY = 3
)

// contains reports whether the viewport-space point falls within the popover's
// current bounds. Used to distinguish clicks on the popover from outside clicks.
func (p *popover) contains(pos geo.Position) bool {
	o, s := p.obj.Position(), p.obj.Size()
	return pos.X >= o.X && pos.Y >= o.Y &&
		pos.X <= o.X+s.Width && pos.Y <= o.Y+s.Height
}

// overlaps reports whether two placed popovers' bounds intersect. Used to decide
// when an interactive popover must yield to its companion in tight space.
func (p *popover) overlaps(q *popover) bool {
	return p.overlapsRect(geo.Rectangle{Pos: q.obj.Position(), Size: q.obj.Size()})
}

// overlapsRect reports whether the popover intersects r, an editor-local
// rectangle — used against the overlay completion popup's reported placement
// (#143), which lives outside the float layer's popover set.
func (p *popover) overlapsRect(r geo.Rectangle) bool {
	if r.IsZero() {
		return false
	}
	ap, as := p.obj.Position(), p.obj.Size()
	return ap.X < r.Pos.X+r.Size.Width && r.Pos.X < ap.X+as.Width &&
		ap.Y < r.Pos.Y+r.Size.Height && r.Pos.Y < ap.Y+as.Height
}

// sizeWithin returns the popover size clamped to its maximums and to the usable
// bounds extent. The body (not obj) is measured, since obj also holds the
// offset shadow.
func (p *popover) sizeWithin(bounds geo.Rectangle) geo.Size {
	size := p.body.MinSize()
	if p.maxWidth > 0 && size.Width > p.maxWidth {
		size.Width = p.maxWidth
	}
	if p.maxHeight > 0 && size.Height > p.maxHeight {
		size.Height = p.maxHeight
	}
	if bounds.Size.Width > 0 && size.Width > bounds.Size.Width {
		size.Width = bounds.Size.Width
	}
	if bounds.Size.Height > 0 && size.Height > bounds.Size.Height {
		size.Height = bounds.Size.Height
	}
	return size
}

// resizeAndPlace sizes the popover and moves it to its anchored, flipped, and
// clamped position within bounds (the usable region in editor-local
// coordinates; its min corner may be negative when the popover may overflow the
// editor into the surrounding canvas — see #19).
func (p *popover) resizeAndPlace(anchor, bounds geo.Rectangle) {
	size := p.sizeWithin(bounds)
	pos, placement := popoverPosition(anchor, size, bounds, p.prefersAbove)
	p.placement = placement
	p.obj.Resize(size)
	p.obj.Move(pos)
	// Lay out the children of the no-layout obj: the body fills obj, and the
	// shadow sits directly behind it at the same bounds — BoxShadow blurs and
	// offsets outward on its own (nothing clips it, and it is non-interactive).
	p.body.Resize(size)
	p.body.Move(geo.NewPos(0, 0))
	p.shadow.Resize(size)
	p.shadow.Move(geo.NewPos(0, 0))
}

// popoverPosition computes where a popover of the given size should sit relative
// to an anchor cell within bounds. It sits on its preferred side of the anchor
// (below by default, or above when prefersAbove is set) and flips to the other
// side only when it does not fit on the preferred side and the opposite side has
// more room; it then clamps so the popover stays within bounds. The anchor and
// the result are in editor-local coordinates; bounds.Pos is the (possibly
// negative) min corner and bounds.Pos+bounds.Size the max corner.
func popoverPosition(anchor geo.Rectangle, size geo.Size, bounds geo.Rectangle, prefersAbove bool) (geo.Position, popoverPlacement) {
	minX, minY := bounds.Pos.X, bounds.Pos.Y
	maxX := minX + bounds.Size.Width
	maxY := minY + bounds.Size.Height

	// Horizontal: left-align with the anchor, then nudge left to stay in bounds.
	x := anchor.Pos.X
	if bounds.Size.Width > 0 && x+size.Width > maxX {
		x = maxX - size.Width
	}
	if x < minX {
		x = minX
	}

	// Vertical: sit on the preferred side, flipping to the other only when the
	// popover does not fit on the preferred side and the opposite side has more
	// room. Keeping the signature card above and completion below this way leaves
	// the anchor line between them, so the two don't overlap (#79).
	below := anchor.Pos.Y + anchor.Size.Height
	roomBelow := maxY - below
	roomAbove := anchor.Pos.Y - minY

	placement := placeBelow
	if prefersAbove {
		placement = placeAbove
		if size.Height > roomAbove && roomBelow > roomAbove {
			placement = placeBelow
		}
	} else if size.Height > roomBelow && roomAbove > roomBelow {
		placement = placeAbove
	}

	y := below
	if placement == placeAbove {
		y = anchor.Pos.Y - size.Height
	}
	if y < minY {
		y = minY
	}
	return geo.NewPos(x, y), placement
}

// popoverLayer is the layout used by the editor's float layer. It never
// repositions its children — the editor places popovers explicitly — but its
// Layout hook fires on resize so the editor can re-anchor the active popover.
type popoverLayer struct {
	onLayout func(geo.Size)
}

func (l *popoverLayer) Layout(_ []core.CanvasObject, size geo.Size) {
	if l.onLayout != nil {
		l.onLayout(size)
	}
}

func (l *popoverLayer) MinSize(_ []core.CanvasObject) geo.Size {
	return geo.Size{}
}
