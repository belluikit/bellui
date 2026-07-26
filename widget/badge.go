package widget

import (
	"fmt"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	internalWidget "github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/theme"
)

const (
	badgePillHeightRatio float32 = 0.4 // pill height as a fraction of icon button size
	badgeTextPadH        float32 = 8   // horizontal padding around badge text
)

// Badge is an MD3 notification indicator (dot or count) attached to a widget.
type Badge struct {
	BaseWidget
	Count   int               // 0 = dot only, >0 = shows number (capped at 999+)
	Content core.CanvasObject // the widget to attach the badge to
}

var _ core.Accessible = (*Badge)(nil)

// AccessibleRole returns [core.RoleBadge].
func (b *Badge) AccessibleRole() core.AccessibleRole { return core.RoleBadge }

// AccessibleLabel returns the badge count as a string, or "new" for dot-only badges.
func (b *Badge) AccessibleLabel() string {
	if b.Count > 0 {
		return fmt.Sprintf("%d", b.Count)
	}
	return "new"
}

// NewBadge creates a dot badge on the given content.
func NewBadge(content core.CanvasObject) *Badge {
	b := &Badge{Content: content}
	b.ExtendBaseWidget(b)
	return b
}

// NewBadgeWithCount creates a numbered badge on the given content.
func NewBadgeWithCount(content core.CanvasObject, count int) *Badge {
	b := &Badge{Content: content, Count: count}
	b.ExtendBaseWidget(b)
	return b
}

func (b *Badge) MinSize() geo.Size {
	b.ExtendBaseWidget(b)
	return b.BaseWidget.MinSize()
}

func (b *Badge) CreateRenderer() core.WidgetRenderer {
	b.ExtendBaseWidget(b)
	dot := &canvas.Circle{}
	pill := canvas.NewRectangle(nil)
	label := canvas.NewText("", nil)
	label.TextStyle.Bold = true

	objects := []core.CanvasObject{b.Content, dot, pill, label}

	r := &badgeRenderer{
		BaseRenderer: internalWidget.NewBaseRenderer(objects),
		dot:          dot,
		pill:         pill,
		label:        label,
		badge:        b,
	}
	r.Refresh()
	return r
}

type badgeRenderer struct {
	internalWidget.BaseRenderer
	dot   *canvas.Circle
	pill  *canvas.Rectangle
	label *canvas.Text
	badge *Badge
}

func (r *badgeRenderer) MinSize() geo.Size {
	if r.badge.Content != nil {
		return r.badge.Content.MinSize()
	}
	return geo.NewSize(0, 0)
}

func (r *badgeRenderer) Layout(size geo.Size) {
	if r.badge.Content != nil {
		r.badge.Content.Resize(size)
		r.badge.Content.Move(geo.NewPos(0, 0))
	}

	th := r.badge.Theme()
	iconBtnSize := th.Size(theme.SizeNameIconButtonSize)
	labelSmall := th.Size(theme.SizeNameLabelSmall)

	// The badge indicator goes at the top-right quadrant of the widget.
	// For a 40x40 IconButton with 24x24 icon centered, the icon's
	// top-right corner is at approximately (32, 8).
	// We want the badge to overlap that corner.

	if r.badge.Count > 0 {
		text := fmt.Sprintf("%d", r.badge.Count)
		if r.badge.Count > 999 {
			text = "999+"
		}
		r.label.Text = text
		r.label.TextSize = labelSmall
		r.label.Show()
		r.dot.Hide()

		pillH := iconBtnSize * badgePillHeightRatio
		textSize := bellui.MeasureText(text, labelSmall, bellui.TextStyle{Bold: true})
		badgeW := geo.Max(pillH, textSize.Width+badgeTextPadH)
		badgeH := pillH

		// Top-right of widget, shifted left so it overlaps the icon.
		bx := size.Width - badgeW + 4
		by := size.Height*0.15 - badgeH/2
		if by < 0 {
			by = 0
		}

		r.pill.CornerRadius = badgeH / 2
		r.pill.Resize(geo.NewSize(badgeW, badgeH))
		r.pill.Move(geo.NewPos(bx, by))
		r.pill.Show()

		r.label.Move(geo.NewPos(
			bx+(badgeW-textSize.Width)/2,
			by+(badgeH-textSize.Height)/2,
		))
	} else {
		r.label.Hide()
		r.pill.Hide()
		r.dot.Show()
		dotSize := iconBtnSize * 0.2
		// Top-right corner, overlapping the icon area.
		bx := size.Width*0.75 - dotSize/2
		by := size.Height*0.15 - dotSize/2
		if by < 0 {
			by = 0
		}
		r.dot.Resize(geo.NewSize(dotSize, dotSize))
		r.dot.Move(geo.NewPos(bx, by))
	}

	// Under RTL the indicator moves to the leading (top-left) corner; the
	// full-width content maps onto itself. The pill and its centred label
	// reflect together so the count stays centred.
	mirrorIfRTL(r.Objects(), size.Width)
}

func (r *badgeRenderer) Refresh() {
	th := r.badge.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()

	errColor := th.Color(theme.ColorNameError, v)
	r.dot.FillColor = errColor
	r.dot.Refresh()

	r.pill.FillColor = errColor
	r.pill.Refresh()

	r.label.Color = th.Color(theme.ColorNameForegroundOnError, v)
	r.label.Refresh()

	if r.badge.Content != nil {
		r.badge.Content.Refresh()
	}

	r.Layout(r.badge.Size())
	canvas.Refresh(r.badge.super())
}
