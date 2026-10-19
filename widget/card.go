package widget

import (
	"image/color"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	internaltheme "github.com/belluikit/bellui/internal/theme"
	"github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/theme"
)

// CardStyle determines the visual variant of a card per MD3 spec.
type CardStyle int

const (
	// CardStyleElevated uses shadow + surface fill (default).
	CardStyleElevated CardStyle = iota
	// CardStyleFilled uses SurfaceContainerHighest fill, no shadow.
	CardStyleFilled
	// CardStyleOutlined uses surface fill with border, no shadow.
	CardStyleOutlined
)

// Card widget groups title, subtitle with content and a header image
//
// Since: 1.0
type Card struct {
	BaseWidget
	Title, Subtitle string
	Image           *canvas.Image
	Content         core.CanvasObject
	// Style sets the MD3 visual variant (Elevated, Filled, Outlined).
	Style CardStyle
}

var _ core.Accessible = (*Card)(nil)

// AccessibleRole returns [core.RoleCard].
func (c *Card) AccessibleRole() core.AccessibleRole { return core.RoleCard }

// AccessibleLabel returns the card title.
func (c *Card) AccessibleLabel() string { return c.Title }

// NewCard creates a new card widget with the specified title, subtitle and content (all optional).
//
// Since: 1.0
func NewCard(title, subtitle string, content core.CanvasObject) *Card {
	card := &Card{
		Title:    title,
		Subtitle: subtitle,
		Content:  content,
	}

	card.ExtendBaseWidget(card)
	return card
}

// CreateRenderer is a private method to Bellui which links this widget to its renderer
func (c *Card) CreateRenderer() core.WidgetRenderer {
	c.ExtendBaseWidget(c)
	th := c.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()

	header := canvas.NewText(c.Title, th.Color(theme.ColorNameForeground, v))
	// MD3 Title Medium: a card's title should not compete with the page's (#494).
	header.TextStyle = theme.TypographyTitleMedium
	subHeader := canvas.NewText(c.Subtitle, header.Color)

	// Always create the background rect; show/hide based on render style.
	bg := canvas.NewRectangle(color.Transparent)
	bg.Hide()

	objects := []core.CanvasObject{bg, header, subHeader}
	if c.Image != nil {
		objects = append(objects, c.Image)
	}
	if c.Content != nil {
		objects = append(objects, c.Content)
	}
	shadowLevel := widget.CardLevel
	if c.Style == CardStyleFilled || c.Style == CardStyleOutlined {
		shadowLevel = widget.BaseLevel
	}
	r := &cardRenderer{
		ShadowingRenderer: widget.NewShadowingRenderer(objects, shadowLevel),
		header:            header,
		subHeader:         subHeader,
		bg:                bg,
		card:              c,
	}
	r.applyTheme()
	return r
}

// MinSize returns the size that this widget should not shrink below
func (c *Card) MinSize() geo.Size {
	c.ExtendBaseWidget(c)
	return c.BaseWidget.MinSize()
}

// SetContent changes the body of this card to have the specified content.
func (c *Card) SetContent(obj core.CanvasObject) {
	c.Content = obj

	c.Refresh()
}

// SetImage changes the image displayed above the title for this card.
func (c *Card) SetImage(img *canvas.Image) {
	c.Image = img

	c.Refresh()
}

// SetSubTitle updates the secondary title for this card.
func (c *Card) SetSubTitle(text string) {
	c.Subtitle = text

	c.Refresh()
}

// SetTitle updates the main title for this card.
func (c *Card) SetTitle(text string) {
	c.Title = text

	c.Refresh()
}

type cardRenderer struct {
	*widget.ShadowingRenderer

	header, subHeader *canvas.Text
	bg                *canvas.Rectangle // non-nil for modern render style

	card *Card
}

// RoundedClipCornerRadius implements the rounded clipping interface.
// Card content is clipped to the card's rounded corners.
func (c *cardRenderer) RoundedClipCornerRadius() float32 {
	if c.bg != nil {
		return c.bg.CornerRadius
	}
	return 0
}

const (
	cardMediaHeight = 128
)

// Layout the components of the card container.
func (c *cardRenderer) Layout(size geo.Size) {
	padding := c.card.Theme().Size(theme.SizeNamePadding)
	pos := geo.NewSquareOffsetPos(padding / 2)
	size = size.Subtract(geo.NewSquareSize(padding))
	c.LayoutShadow(size, pos)

	if c.bg != nil {
		c.bg.Move(pos)
		c.bg.Resize(size)
	}

	if c.card.Image != nil {
		c.card.Image.Move(pos)
		c.card.Image.Resize(geo.NewSize(size.Width, cardMediaHeight))
		pos.Y += cardMediaHeight
	}

	if c.card.Title != "" || c.card.Subtitle != "" {
		titlePad := padding * 2
		size.Width -= titlePad * 2
		pos.X += titlePad
		pos.Y += titlePad

		if c.card.Title != "" {
			height := c.header.MinSize().Height
			c.header.Move(pos)
			c.header.Resize(geo.NewSize(size.Width, height))
			pos.Y += height + padding
		}

		if c.card.Subtitle != "" {
			height := c.subHeader.MinSize().Height
			c.subHeader.Move(pos)
			c.subHeader.Resize(geo.NewSize(size.Width, height))
			pos.Y += height + padding
		}

		size.Width = size.Width + titlePad*2
		pos.X = pos.X - titlePad
		pos.Y += titlePad
	}

	size.Width -= padding * 2
	pos.X += padding
	if c.card.Content != nil {
		height := size.Height - padding*2 - (pos.Y - padding/2) // adjust for content and initial offset
		if c.card.Title != "" || c.card.Subtitle != "" {
			height += padding
			pos.Y -= padding
		}
		c.card.Content.Move(pos.Add(geo.NewPos(0, padding)))
		c.card.Content.Resize(geo.NewSize(size.Width, height))
	}
}

// MinSize calculates the minimum size of a card.
// This is based on the contained text, image and content.
func (c *cardRenderer) MinSize() geo.Size {
	hasHeader := c.card.Title != ""
	hasSubHeader := c.card.Subtitle != ""
	hasImage := c.card.Image != nil
	hasContent := c.card.Content != nil

	padding := c.card.Theme().Size(theme.SizeNamePadding)
	if !hasHeader && !hasSubHeader && !hasContent { // just image, or nothing
		if c.card.Image == nil {
			return geo.NewSize(padding, padding) // empty, just space for border
		}
		return geo.NewSize(c.card.Image.MinSize().Width+padding, cardMediaHeight+padding)
	}

	min := geo.NewSize(padding, padding)
	if hasImage {
		min = geo.NewSize(min.Width, min.Height+cardMediaHeight)
	}

	if hasHeader || hasSubHeader {
		titlePad := padding * 2
		min = min.Add(geo.NewSize(0, titlePad*2))
		if hasHeader {
			headerMin := c.header.MinSize()
			min = geo.NewSize(geo.Max(min.Width, headerMin.Width+titlePad*2+padding),
				min.Height+headerMin.Height)
			if hasSubHeader {
				min.Height += padding
			}
		}
		if hasSubHeader {
			subHeaderMin := c.subHeader.MinSize()
			min = geo.NewSize(geo.Max(min.Width, subHeaderMin.Width+titlePad*2+padding),
				min.Height+subHeaderMin.Height)
		}
	}

	if hasContent {
		contentMin := c.card.Content.MinSize()
		min = geo.NewSize(geo.Max(min.Width, contentMin.Width+padding*3),
			min.Height+contentMin.Height+padding*2)
	}

	return min
}

func (c *cardRenderer) Refresh() {
	c.header.Text = c.card.Title
	c.header.Refresh()
	c.subHeader.Text = c.card.Subtitle
	c.subHeader.Refresh()

	objects := []core.CanvasObject{c.bg, c.header, c.subHeader}
	if c.card.Image != nil {
		objects = append(objects, c.card.Image)
	}
	if c.card.Content != nil {
		objects = append(objects, c.card.Content)
	}
	c.ShadowingRenderer.SetObjects(objects)

	c.applyTheme()
	c.Layout(c.card.Size())
	c.ShadowingRenderer.RefreshShadow()
	canvas.Refresh(c.card.super())
}

// applyTheme updates this card to match the current theme
func (c *cardRenderer) applyTheme() {
	th := c.card.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()
	style := internaltheme.RenderStyleForWidget(c.card)

	if c.bg != nil {
		radius := th.Size(theme.SizeNameCardRadius)
		c.bg.CornerRadius = radius

		switch c.card.Style {
		case CardStyleFilled:
			c.bg.FillColor = th.Color(theme.ColorNameSurfaceContainerHighest, v)
			c.bg.StrokeWidth = 0
			c.bg.StrokeColor = color.Transparent
			c.bg.Show()
			c.bg.Refresh()
		case CardStyleOutlined:
			c.bg.FillColor = th.Color(theme.ColorNameSurface, v)
			c.bg.StrokeWidth = 1
			c.bg.StrokeColor = th.Color(theme.ColorNameOutlineVariant, v)
			c.bg.Show()
			c.bg.Refresh()
		default: // CardStyleElevated
			if style == internaltheme.RenderStyleModern {
				c.bg.FillColor = th.Color(theme.ColorNameCardBackground, v)
				c.bg.StrokeWidth = 1
				c.bg.StrokeColor = th.Color(theme.ColorNameSeparator, v)
				c.bg.Show()
				c.bg.Refresh()
			} else {
				c.bg.Hide()
			}
		}
	}
	if c.header != nil {
		c.header.TextSize = th.Size(theme.SizeNameTitleMedium)
		c.header.Color = th.Color(theme.ColorNameForeground, v)
	}
	if c.subHeader != nil {
		c.subHeader.TextSize = th.Size(theme.SizeNameText)
		c.subHeader.Color = th.Color(theme.ColorNameForeground, v)
	}
	if c.card.Content != nil {
		c.card.Content.Refresh()
	}
}
