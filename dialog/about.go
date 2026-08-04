package dialog

// The About dialog follows the API shape of the bellui-x project's dialog.NewAbout
// (fyne.io/x/fyne, dialog/about.go, Copyright (C) 2020 Fyne.io developers and
// community, BSD-3-Clause license), reimplemented natively for this fork's
// MD3 theming: surface-neutral composition, typography tokens for the app
// name and version, and RichText markdown content (remote content stays
// disabled, the parsed content is app-supplied but treated as untrusted).

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
)

// NewAbout creates an about dialog for the given app: its metadata icon,
// name and version above the markdown content, with the links in a row along
// the bottom. You should call Show on the returned dialog to display it.
//
// Since: 1.0
func NewAbout(content string, links []*widget.Hyperlink, a bellui.App, w bellui.Window) Dialog {
	d := NewCustom(aboutTitle(a.Metadata()), lang.L("OK"),
		aboutContent(content, links, a.Metadata()), w)
	d.Resize(geo.NewSize(420, 400))

	return d
}

// NewAboutWindow creates an about window for the given app: its metadata
// icon, name and version above the markdown content, with the links in a row
// along the bottom. You should call Show on the returned window to display it.
//
// Since: 1.0
func NewAboutWindow(content string, links []*widget.Hyperlink, a bellui.App) bellui.Window {
	w := a.NewWindow(aboutTitle(a.Metadata()))
	w.SetContent(container.NewPadded(aboutContent(content, links, a.Metadata())))
	w.Resize(geo.NewSize(420, 440))

	return w
}

// ShowAbout opens an about dialog built from the app metadata along with the
// markdown content and links passed into this method.
//
// Since: 1.0
func ShowAbout(content string, links []*widget.Hyperlink, a bellui.App, w bellui.Window) {
	NewAbout(content, links, a, w).Show()
}

// ShowAboutWindow opens an about window built from the app metadata along
// with the markdown content and links passed into this method.
//
// Since: 1.0
func ShowAboutWindow(content string, links []*widget.Hyperlink, a bellui.App) {
	NewAboutWindow(content, links, a).Show()
}

func aboutTitle(meta bellui.AppMetadata) string {
	if meta.Name == "" {
		return lang.L("About")
	}
	return lang.L("About") + " " + meta.Name
}

// aboutContent composes the shared body: icon + name (HeadlineSmall) +
// version (BodyMedium) header, scrollable markdown content, links footer.
func aboutContent(content string, links []*widget.Hyperlink, meta bellui.AppMetadata) core.CanvasObject {
	header := container.NewVBox()
	if meta.Icon != nil && len(meta.Icon.Content()) > 0 {
		logo := canvas.NewImageFromResource(meta.Icon)
		logo.FillMode = canvas.ImageFillContain
		logo.SetMinSize(geo.NewSquareSize(96))
		header.Add(logo)
	}
	if meta.Name != "" {
		name := widget.NewLabelWithStyle(meta.Name, bellui.TextAlignCenter, theme.TypographyHeadlineSmall)
		name.SizeName = theme.SizeNameHeadlineSmall
		header.Add(name)
	}
	if meta.Version != "" {
		version := widget.NewLabelWithStyle(lang.L("Version")+" "+meta.Version,
			bellui.TextAlignCenter, theme.TypographyBodyMedium)
		version.SizeName = theme.SizeNameBodyMedium
		header.Add(version)
	}

	rich := widget.NewRichTextFromMarkdown(content)
	rich.Wrapping = bellui.TextWrapWord
	scroll := container.NewScroll(rich)

	var footer core.CanvasObject
	if len(links) > 0 {
		row := container.NewHBox()
		for i, link := range links {
			if i > 0 {
				row.Add(widget.NewLabel("·"))
			}
			row.Add(link)
		}
		footer = container.NewCenter(row)
	}

	body := container.NewBorder(header, footer, nil, nil, scroll)
	if underlay := aboutUnderlay(meta, scroll); underlay != nil {
		return container.NewStack(underlay, body)
	}
	return body
}

// aboutParallaxFactor divides the content scroll offset into the underlay
// movement, so the decoration drifts slower than the text above it.
const aboutParallaxFactor = 3

// aboutUnderlay builds the decorative parallax layer (#453): the app icon,
// oversized and almost transparent in the manner of an MD3 surface tint,
// drifting upward at a fraction of the content scroll speed. Nil without an
// icon. The layer is clipped by a non-scrolling Scroll so the oversized icon
// never paints outside the dialog body.
func aboutUnderlay(meta bellui.AppMetadata, scroll *container.Scroll) core.CanvasObject {
	if meta.Icon == nil || len(meta.Icon.Content()) == 0 {
		return nil
	}

	img := canvas.NewImageFromResource(meta.Icon)
	img.FillMode = canvas.ImageFillContain
	img.Translucency = 0.94
	slide := &parallaxLayout{}
	layer := &bellui.Container{Layout: slide, Objects: []core.CanvasObject{img}}

	scroll.AddScrollListener(func(offset geo.Position) {
		slide.offset = -offset.Y / aboutParallaxFactor
		slide.Layout(layer.Objects, layer.Size())
	})

	clip := container.NewScroll(layer)
	clip.Direction = container.ScrollNone
	return clip
}

// parallaxLayout centers its (single) object horizontally, oversized, and
// shifts it vertically by the current parallax offset.
type parallaxLayout struct {
	offset float32
}

func (p *parallaxLayout) Layout(objects []core.CanvasObject, size geo.Size) {
	if len(objects) == 0 {
		return
	}
	side := geo.Max(size.Width*0.8, 220)
	under := objects[0]
	under.Resize(geo.NewSquareSize(side))
	under.Move(geo.NewPos((size.Width-side)/2, p.offset))
}

func (p *parallaxLayout) MinSize([]core.CanvasObject) geo.Size {
	return geo.Size{}
}
