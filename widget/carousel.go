package widget

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	internalWidget "github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/bellui/theme"
)

var _ event.Draggable = (*Carousel)(nil)

// Carousel is an MD3 horizontally-scrolling showcase of items with snapping.
type Carousel struct {
	BaseWidget
	Items       []core.CanvasObject
	CurrentItem int
	OnChanged   func(int) `json:"-"`
}

// NewCarousel creates a carousel with the given items.
func NewCarousel(items ...core.CanvasObject) *Carousel {
	c := &Carousel{Items: items}
	c.ExtendBaseWidget(c)
	return c
}

// AccessibleRole returns [core.RoleGroup]; the navigation buttons and item
// content inside provide their own semantics.
func (c *Carousel) AccessibleRole() core.AccessibleRole { return core.RoleGroup }

// AccessibleLabel returns an empty string; use SetAccessibilityLabel to
// name the showcase.
func (c *Carousel) AccessibleLabel() string { return "" }

// Next advances to the next item.
func (c *Carousel) Next() {
	if c.CurrentItem < len(c.Items)-1 {
		c.CurrentItem++
		if c.OnChanged != nil {
			c.OnChanged(c.CurrentItem)
		}
		c.Refresh()
	}
}

// Previous goes to the previous item.
func (c *Carousel) Previous() {
	if c.CurrentItem > 0 {
		c.CurrentItem--
		if c.OnChanged != nil {
			c.OnChanged(c.CurrentItem)
		}
		c.Refresh()
	}
}

func (c *Carousel) Dragged(e *event.DragEvent) {
	// Handled on DragEnd for snap behavior.
}

func (c *Carousel) DragEnd() {
	// Snap handled by button taps; drag-to-snap can be added later.
}

func (c *Carousel) MinSize() geo.Size {
	c.ExtendBaseWidget(c)
	return c.BaseWidget.MinSize()
}

func (c *Carousel) CreateRenderer() core.WidgetRenderer {
	c.ExtendBaseWidget(c)
	objects := append([]core.CanvasObject{}, c.Items...)

	// Dot indicators.
	var dots []*canvas.Circle
	for range c.Items {
		dot := &canvas.Circle{}
		dots = append(dots, dot)
		objects = append(objects, dot)
	}

	// Navigation buttons.
	prevBtn := NewIconButton(theme.NavigateBackIcon(), func() { c.Previous() })
	prevBtn.Style = IconButtonStyleFilledTonal
	prevBtn.SetAccessibilityLabel(lang.L("Previous"))
	nextBtn := NewIconButton(theme.NavigateNextIcon(), func() { c.Next() })
	nextBtn.Style = IconButtonStyleFilledTonal
	nextBtn.SetAccessibilityLabel(lang.L("Next"))
	objects = append(objects, prevBtn, nextBtn)

	r := &carouselRenderer{
		BaseRenderer: internalWidget.NewBaseRenderer(objects),
		dots:         dots,
		prevBtn:      prevBtn,
		nextBtn:      nextBtn,
		carousel:     c,
	}
	r.Refresh()
	return r
}

type carouselRenderer struct {
	internalWidget.BaseRenderer
	dots     []*canvas.Circle
	prevBtn  *IconButton
	nextBtn  *IconButton
	carousel *Carousel
}

func (r *carouselRenderer) MinSize() geo.Size {
	if len(r.carousel.Items) == 0 {
		return geo.NewSize(200, 150)
	}
	max := r.carousel.Items[0].MinSize()
	for _, item := range r.carousel.Items[1:] {
		min := item.MinSize()
		if min.Width > max.Width {
			max.Width = min.Width
		}
		if min.Height > max.Height {
			max.Height = min.Height
		}
	}
	return geo.NewSize(max.Width+80, max.Height+32) // nav buttons + dots
}

func (r *carouselRenderer) Layout(size geo.Size) {
	contentH := size.Height - 32 // space for dots
	contentW := size.Width - 80  // space for nav buttons

	// Show only current item, hide others.
	for i, item := range r.carousel.Items {
		if i == r.carousel.CurrentItem {
			item.Show()
			item.Move(geo.NewPos(40, 0))
			item.Resize(geo.NewSize(contentW, contentH))
		} else {
			item.Hide()
		}
	}

	// Nav buttons.
	r.prevBtn.Resize(geo.NewSize(32, 32))
	r.prevBtn.Move(geo.NewPos(4, contentH/2-16))
	r.nextBtn.Resize(geo.NewSize(32, 32))
	r.nextBtn.Move(geo.NewPos(size.Width-36, contentH/2-16))

	// Dots.
	dotSize := float32(8)
	dotGap := float32(8)
	totalDotsW := float32(len(r.dots))*(dotSize+dotGap) - dotGap
	startX := (size.Width - totalDotsW) / 2
	for i, dot := range r.dots {
		dot.Resize(geo.NewSize(dotSize, dotSize))
		dot.Move(geo.NewPos(startX+float32(i)*(dotSize+dotGap), contentH+12))
	}

	// Under RTL reflect the controls: prev/next swap sides and the dots reverse.
	// The centred content page maps onto itself, so it does not shift; hidden
	// (non-current) items are skipped by the reflection.
	mirrorIfRTL(r.Objects(), size.Width)
}

func (r *carouselRenderer) Refresh() {
	th := r.carousel.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()

	for i, dot := range r.dots {
		if i == r.carousel.CurrentItem {
			dot.FillColor = th.Color(theme.ColorNamePrimary, v)
		} else {
			dot.FillColor = th.Color(theme.ColorNameOutlineVariant, v)
		}
		dot.Refresh()
	}

	// Hide prev/next at boundaries.
	if r.carousel.CurrentItem == 0 {
		r.prevBtn.Hide()
	} else {
		r.prevBtn.Show()
	}
	if r.carousel.CurrentItem >= len(r.carousel.Items)-1 {
		r.nextBtn.Hide()
	} else {
		r.nextBtn.Show()
	}

	r.Layout(r.carousel.Size())
	canvas.Refresh(r.carousel.super())
}
