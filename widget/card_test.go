package widget_test

import (
	"image/color"
	"testing"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"

	"github.com/stretchr/testify/assert"
)

func TestCard_SetImage(t *testing.T) {
	c := widget.NewCard("Title", "sub", widget.NewLabel("Content"))
	r := test.TempWidgetRenderer(t, c)
	assert.Len(t, r.Objects(), 5) // shadow + bg + header + subHeader + content

	c.SetImage(canvas.NewImageFromResource(theme.ComputerIcon()))
	assert.Len(t, r.Objects(), 6)
}

func TestCard_SetContent(t *testing.T) {
	c := widget.NewCard("Title", "sub", widget.NewLabel("Content"))
	r := test.TempWidgetRenderer(t, c)
	assert.Len(t, r.Objects(), 5) // shadow + bg + header + subHeader + content

	newContent := widget.NewLabel("New")
	c.SetContent(newContent)
	assert.Len(t, r.Objects(), 5)
	assert.Equal(t, newContent, r.Objects()[4])
}

func TestCard_Layout(t *testing.T) {
	test.NewApp()

	for name, tt := range map[string]struct {
		title, subtitle string
		icon            *canvas.Image
		content         core.CanvasObject
	}{
		"title": {
			title:    "Title",
			subtitle: "",
			icon:     nil,
			content:  nil,
		},
		"subtitle": {
			title:    "",
			subtitle: "Subtitle",
			icon:     nil,
			content:  nil,
		},
		"titles": {
			title:    "Title",
			subtitle: "Subtitle",
			icon:     nil,
			content:  nil,
		},
		"titles_image": {
			title:    "Title",
			subtitle: "Subtitle",
			icon:     canvas.NewImageFromResource(theme.ComputerIcon()),
			content:  nil,
		},
		"just_image": {
			title:    "",
			subtitle: "",
			icon:     canvas.NewImageFromResource(theme.ComputerIcon()),
			content:  nil,
		},
		"just_content": {
			title:    "",
			subtitle: "",
			icon:     nil,
			content:  newContentRect(),
		},
		"title_content": {
			title:    "Hello",
			subtitle: "",
			icon:     nil,
			content:  newContentRect(),
		},
		"image_content": {
			title:    "",
			subtitle: "",
			icon:     canvas.NewImageFromResource(theme.ComputerIcon()),
			content:  newContentRect(),
		},
		"all_items": {
			title:    "Longer title",
			subtitle: "subtitle with length",
			icon:     canvas.NewImageFromResource(theme.ComputerIcon()),
			content:  newContentRect(),
		},
	} {
		t.Run(name, func(t *testing.T) {
			card := &widget.Card{
				Title:    tt.title,
				Subtitle: tt.subtitle,
				Image:    tt.icon,
				Content:  tt.content,
			}

			window := test.NewTempWindow(t, card)
			size := card.MinSize().Max(geo.NewSize(80, 0)) // give a little width for image only tests
			window.Resize(size.Add(geo.NewSize(theme.InnerPadding(), theme.InnerPadding())))
			if tt.content != nil {
				assert.Equal(t, float32(10), tt.content.Size().Height)
			}
			test.AssertRendersToMarkup(t, "card/layout_"+name+".xml", window.Canvas())
		})
	}
}

func TestCard_MinSize(t *testing.T) {
	content := widget.NewLabel("simple")
	card := &widget.Card{Content: content}

	// shadow + content pad; across, the content is inset to the title's edge (#494)
	inner := card.MinSize().Subtract(geo.NewSize(theme.InnerPadding()*2+theme.Padding(), theme.InnerPadding()+theme.Padding()))
	assert.Equal(t, content.MinSize(), inner)
}

func TestCard_Refresh(t *testing.T) {
	text := widget.NewLabel("Test")
	card := widget.NewCard("", "", text)
	w := test.NewTempWindow(t, card)
	test.AssertRendersToMarkup(t, "card/content_label.xml", w.Canvas())

	text.Text = "Changed"
	card.Refresh()
	test.AssertRendersToMarkup(t, "card/content_label_changed.xml", w.Canvas())
}

func newContentRect() *canvas.Rectangle {
	rect := canvas.NewRectangle(color.Gray{0x66})
	rect.StrokeColor = color.Black
	rect.StrokeWidth = 2
	rect.SetMinSize(geo.NewSize(10, 10))

	return rect
}
