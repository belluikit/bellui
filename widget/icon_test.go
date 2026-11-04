package widget_test

import (
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/layout"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
)

func TestIcon_Layout(t *testing.T) {
	test.NewTempApp(t)

	for name, tt := range map[string]struct {
		resource bellui.Resource
	}{
		"empty": {},
		"resource": {
			resource: theme.CancelIcon(),
		},
	} {
		t.Run(name, func(t *testing.T) {
			icon := &widget.Icon{
				Resource: tt.resource,
			}

			window := test.NewTempWindow(t, &bellui.Container{Layout: layout.NewCenterLayout(), Objects: []core.CanvasObject{icon}})
			window.Resize(icon.MinSize().Max(geo.NewSize(150, 200)))

			test.AssertRendersToMarkup(t, "icon/layout_"+name+".xml", window.Canvas())
		})
	}
}

// TestIcon_IconSize pins #937: IconSize sets the icon's min size and the
// size its image is drawn at, inside a layout that keeps children at their
// min size; changing it and refreshing re-lays the icon out; zero keeps the
// theme's inline icon size.
func TestIcon_IconSize(t *testing.T) {
	test.NewTempApp(t)

	icon := widget.NewIcon(theme.CancelIcon())
	themed := icon.MinSize()
	if want := test.Theme().Size(theme.SizeNameInlineIcon); themed != geo.NewSquareSize(want) {
		t.Fatalf("zero IconSize: MinSize = %v, want the theme's %v", themed, want)
	}

	box := &bellui.Container{Layout: layout.NewCenterLayout(), Objects: []core.CanvasObject{icon}}
	window := test.NewTempWindow(t, box)
	window.Resize(geo.NewSize(200, 200))

	icon.IconSize = 96
	icon.Refresh()
	box.Refresh()
	if got := icon.MinSize(); got != geo.NewSquareSize(96) {
		t.Errorf("IconSize 96: MinSize = %v, want 96x96", got)
	}
	if got := icon.Size(); got != geo.NewSquareSize(96) {
		t.Errorf("IconSize 96: laid out at %v, want 96x96", got)
	}
	img := test.WidgetRenderer(icon).Objects()[0]
	if got := img.Size(); got != geo.NewSquareSize(96) {
		t.Errorf("IconSize 96: image drawn at %v, want 96x96", got)
	}

	icon.IconSize = 0
	icon.Refresh()
	box.Refresh()
	if got := icon.Size(); got != themed {
		t.Errorf("IconSize back to 0: laid out at %v, want the theme's %v", got, themed)
	}
}
