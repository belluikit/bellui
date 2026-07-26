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
