package widget

import (
	"bytes"
	_ "embed"
	"log"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	internalWidget "github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
)

//go:embed testdata/bellui.png
var iconData []byte

func TestMenuItem_Disabled(t *testing.T) {
	t.Run("can render disabled item", func(t *testing.T) {
		i := bellui.NewMenuItem("Disabled", func() {})
		m := bellui.NewMenu("top", []*bellui.MenuItem{i}...)
		i.Disabled = true
		w := newMenuItem(i, NewMenu(m))
		r := cache.Renderer(w)

		assert.Equal(t, theme.Color(theme.ColorNameDisabled), r.(*menuItemRenderer).text.Color)
	})

	t.Run("should apply themeing to disabled item SVG icon", func(t *testing.T) {
		i := bellui.NewMenuItem("Disabled", func() {})
		m := bellui.NewMenu("top", i)
		i.Disabled = true
		i.Icon = theme.AccountIcon()
		w := newMenuItem(i, NewMenu(m))
		r := cache.Renderer(w)

		assert.Equal(t, theme.NewDisabledResource(theme.AccountIcon()), r.(*menuItemRenderer).icon.Resource)
	})

	t.Run("should not apply themeing to disabled item with PNG icon", func(t *testing.T) {
		i := bellui.NewMenuItem("Disabled", func() {})
		m := bellui.NewMenu("top", i)
		i.Disabled = true
		i.Icon = bellui.NewStaticResource("bellui.png", iconData)
		w := newMenuItem(i, NewMenu(m))
		r := cache.Renderer(w)

		assert.Equal(t, i.Icon, r.(*menuItemRenderer).icon.Resource)
	})

	t.Run("should not log error when rendering disabled menu item with PNG icon", func(t *testing.T) {
		var buf bytes.Buffer
		log.SetOutput(&buf)
		defer func() {
			log.SetOutput(os.Stderr)
		}()

		test.NewTempApp(t)

		w := bellui.CurrentApp().NewWindow("")
		defer w.Close()
		w.SetPadded(false)
		c := w.Canvas()

		item := bellui.NewMenuItem("Foo", func() {})
		item.Icon = bellui.NewStaticResource("bellui.png", iconData)
		item.Disabled = true
		m := NewMenu(bellui.NewMenu("", item))
		size := m.MinSize()
		w.Resize(size.Add(geo.NewSize(10, 10)))
		m.Resize(size)
		o := internalWidget.NewOverlayContainer(m, c, func() {})
		w.SetContent(o)

		assert.Equal(t, "", buf.String()) // should not log an error
	})

	t.Run("should not log error when rendering disabled menu item with SVG icon", func(t *testing.T) {
		var buf bytes.Buffer
		log.SetOutput(&buf)
		defer func() {
			log.SetOutput(os.Stderr)
		}()

		test.NewTempApp(t)

		w := bellui.CurrentApp().NewWindow("")
		defer w.Close()
		w.SetPadded(false)
		c := w.Canvas()

		item := bellui.NewMenuItem("Foo", func() {})
		item.Icon = theme.AccountIcon()
		item.Disabled = true
		m := NewMenu(bellui.NewMenu("", item))
		size := m.MinSize()
		w.Resize(size.Add(geo.NewSize(10, 10)))
		m.Resize(size)
		o := internalWidget.NewOverlayContainer(m, c, func() {})
		w.SetContent(o)

		assert.Equal(t, "", buf.String()) // should not log an error
	})
}
