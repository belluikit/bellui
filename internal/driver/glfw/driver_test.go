//go:build !no_glfw && !ci && !mobile

package glfw

import (
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	internalTest "github.com/belluikit/bellui/internal/test"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"

	"github.com/stretchr/testify/assert"
)

func Test_gLDriver_AbsolutePositionForObject(t *testing.T) {
	w := createWindow("Test")
	// Pin the theme: all expected coordinates below depend on it, and other
	// tests in the package may leave a different theme applied.
	test.ApplyTheme(t, internalTest.DarkTheme(theme.DefaultTheme()))

	cr1c1 := widget.NewLabel("row 1 col 1")
	cr1c2 := widget.NewLabel("row 1 col 2")
	cr1c3 := widget.NewLabel("row 1 col 3")
	cr2c1 := widget.NewLabel("row 2 col 1")
	cr2c2 := widget.NewLabel("row 2 col 2")
	cr2c3 := widget.NewLabel("row 2 col 3")
	cr3c1 := widget.NewLabel("row 3 col 1")
	cr3c2 := widget.NewLabel("row 3 col 2")
	cr3c3 := widget.NewLabel("row 3 col 3")
	cr1 := container.NewHBox(cr1c1, cr1c2, cr1c3)
	cr2 := container.NewHBox(cr2c1, cr2c2, cr2c3)
	cr3 := container.NewHBox(cr3c1, cr3c2, cr3c3)
	content := container.NewVBox(cr1, cr2, cr3)
	cr2c2.Hide()

	mm := bellui.NewMainMenu(
		bellui.NewMenu("Menu 1", bellui.NewMenuItem("Menu 1 Item", nil)),
		bellui.NewMenu("Menu 2", bellui.NewMenuItem("Menu 2 Item", nil)),
	)
	// We want to test the handling of the canvas' Bellui menu here.
	// We work around w.SetMainMenu because on MacOS the main menu is a native menu.
	c := w.Canvas()
	movl := buildMenuOverlay(mm, w.window)
	c.setMenuOverlay(movl)
	w.SetContent(content)
	w.Resize(geo.NewSize(300, 200))
	ensureCanvasSize(t, w, geo.NewSize(300, 200))

	ovli1 := widget.NewLabel("Overlay Item 1")
	ovli2 := widget.NewLabel("Overlay Item 2")
	ovli3 := widget.NewLabel("Overlay Item 3")
	ovlContent := container.NewVBox(ovli1, ovli2, ovli3)
	// use the unsafe canvas so we can wrap the whole show in safety without deadlock
	ovl := widget.NewModalPopUp(ovlContent, w.window.Canvas())
	runOnMain(ovl.Show)

	// This helps to detect size changes which might happen when the font size or rendering are changed.
	// It gives also a hint on the expected offset for the overlay components.
	var size geo.Size
	runOnMain(func() {
		size = ovlContent.Size()
	})
	assert.InDelta(t, 109, size.Width, 1)
	assert.InDelta(t, 121, size.Height, 1)

	repaintWindow(w)
	ensureCanvasSize(t, w, geo.NewSize(300, 200))
	// accessing the menu bar's actual CanvasObjects isn't straight forward
	// 0 is the shadow
	// 1 is the menu bar’s underlay
	// 2 is the menu bar's background
	// 3 is the container holding the items
	var m2 core.CanvasObject
	runOnMain(func() {
		mbarCont := cache.Renderer(movl.(core.Widget)).Objects()[3].(*bellui.Container)
		m2 = mbarCont.Objects[1]
	})

	tests := map[string]struct {
		object       core.CanvasObject
		wantX, wantY int
	}{
		"a cell": {
			object: cr1c3,
			wantX:  194,
			wantY:  32,
		},
		"a row": {
			object: cr2,
			wantX:  6,
			wantY:  74,
		},
		"the window content": {
			object: content,
			wantX:  6,
			wantY:  32,
		},
		"a hidden element": {
			object: cr2c2,
			wantX:  0,
			wantY:  0,
		},

		"a menu": {
			object: m2,
			wantX:  82,
			wantY:  0,
		},

		"an overlay item": {
			object: ovli2,
			wantX:  95,
			wantY:  81,
		},
		"the overlay content": {
			object: ovlContent,
			wantX:  95,
			wantY:  39,
		},
		"the overlay": {
			object: ovl,
			wantX:  0,
			wantY:  0,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			runOnMain(func() {
				pos := d.AbsolutePositionForObject(tt.object)
				assert.Equal(t, tt.wantX, int(pos.X))
				assert.Equal(t, tt.wantY, int(pos.Y))
			})
		})
	}
}
