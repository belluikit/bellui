package container

import (
	"testing"

	"github.com/belluikit/bellui/geo"
	internalTest "github.com/belluikit/bellui/internal/test"
	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
)

func TestTabButton_Icon_Change(t *testing.T) {
	b := &tabButton{icon: theme.CancelIcon()}
	r := cache.Renderer(b)
	icon := r.Objects()[4].(*canvas.Image)
	oldResource := icon.Resource

	b.icon = theme.ConfirmIcon()
	b.Refresh()
	assert.NotEqual(t, oldResource, icon.Resource)
}

func TestTab_ThemeChange(t *testing.T) {
	a := test.NewTempApp(t)
	a.Settings().SetTheme(internalTest.LightTheme(theme.DefaultTheme()))

	tabs := NewAppTabs(
		NewTabItem("a", widget.NewLabel("a")),
		NewTabItem("b", widget.NewLabel("b")),
	)
	w := test.NewTempWindow(t, tabs)
	w.Resize(geo.NewSize(180, 120))

	initial := w.Canvas().Capture()

	a.Settings().SetTheme(internalTest.DarkTheme(theme.DefaultTheme()))
	tabs.SelectIndex(1)
	second := w.Canvas().Capture()
	assert.NotEqual(t, initial, second)

	a.Settings().SetTheme(internalTest.LightTheme(theme.DefaultTheme()))
	tabs.SelectIndex(0)
	assert.Equal(t, initial, w.Canvas().Capture())
}

// Removing a tab before the selected one keeps the same tab selected and
// reports nothing, including when the selected tab is the last (#1201). It
// used to shift the selection twice and report the tab it passed through.
func TestTabs_RemoveBeforeSelected(t *testing.T) {
	type tabs interface {
		SelectIndex(int)
		SelectedIndex() int
		Selected() *TabItem
		RemoveIndex(int)
		Remove(*TabItem)
	}
	kinds := map[string]func(items ...*TabItem) (tabs, *func(*TabItem), *func(*TabItem)){
		"AppTabs": func(items ...*TabItem) (tabs, *func(*TabItem), *func(*TabItem)) {
			a := NewAppTabs(items...)
			return a, &a.OnSelected, &a.OnUnselected
		},
		"DocTabs": func(items ...*TabItem) (tabs, *func(*TabItem), *func(*TabItem)) {
			d := NewDocTabs(items...)
			return d, &d.OnSelected, &d.OnUnselected
		},
	}
	for name, newTabs := range kinds {
		for _, c := range []struct {
			name             string
			selected, remove int
			byItem           bool
		}{
			{"last selected", 2, 0, false},
			{"middle selected", 1, 0, false},
			{"last selected, by item", 2, 1, true},
		} {
			a := &TabItem{Text: "a", Content: widget.NewLabel("a")}
			b := &TabItem{Text: "b", Content: widget.NewLabel("b")}
			c3 := &TabItem{Text: "c", Content: widget.NewLabel("c")}
			items := []*TabItem{a, b, c3}
			tb, onSelected, onUnselected := newTabs(items...)
			tb.SelectIndex(c.selected)
			want := items[c.selected]
			var fired []string
			*onSelected = func(ti *TabItem) { fired = append(fired, "selected "+ti.Text) }
			*onUnselected = func(ti *TabItem) { fired = append(fired, "unselected "+ti.Text) }

			if c.byItem {
				tb.Remove(items[c.remove])
			} else {
				tb.RemoveIndex(c.remove)
			}
			msg := name + ": " + c.name
			assert.Same(t, want, tb.Selected(), msg)
			assert.Equal(t, c.selected-1, tb.SelectedIndex(), msg)
			assert.Empty(t, fired, msg)
		}
	}
}
