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

// Removing the selected tab reports the change wherever the tab was: the tab
// that goes is unselected, and the one that takes its place is selected
// (#1204). Only a middle-or-first removal used to be silent, and a last one
// reported the new tab but not the old.
func TestTabs_RemoveSelectedReports(t *testing.T) {
	type tabs interface {
		SelectIndex(int)
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
			name   string
			texts  []string
			remove int
			byItem bool
			now    string // "" for none
			fired  []string
		}{
			{"first", []string{"a", "b", "c"}, 0, false, "b", []string{"unselected a", "selected b"}},
			{"middle", []string{"a", "b", "c"}, 1, false, "c", []string{"unselected b", "selected c"}},
			{"last", []string{"a", "b", "c"}, 2, false, "b", []string{"unselected c", "selected b"}},
			{"middle, by item", []string{"a", "b", "c"}, 1, true, "c", []string{"unselected b", "selected c"}},
			{"only", []string{"a"}, 0, false, "", []string{"unselected a"}},
		} {
			var items []*TabItem
			for _, s := range c.texts {
				items = append(items, &TabItem{Text: s, Content: widget.NewLabel(s)})
			}
			tb, onSelected, onUnselected := newTabs(items...)
			tb.SelectIndex(c.remove)
			var fired []string
			*onSelected = func(ti *TabItem) { fired = append(fired, "selected "+ti.Text) }
			*onUnselected = func(ti *TabItem) { fired = append(fired, "unselected "+ti.Text) }

			if c.byItem {
				tb.Remove(items[c.remove])
			} else {
				tb.RemoveIndex(c.remove)
			}
			msg := name + ": " + c.name
			if c.now == "" {
				assert.Nil(t, tb.Selected(), msg)
			} else if assert.NotNil(t, tb.Selected(), msg) {
				assert.Equal(t, c.now, tb.Selected().Text, msg)
			}
			assert.Equal(t, c.fired, fired, msg)
		}
	}
}

// Closing the selected DocTabs tab reports the selection change before
// OnClosed (#1204).
func TestDocTabs_CloseSelectedReports(t *testing.T) {
	a := &TabItem{Text: "a", Content: widget.NewLabel("a")}
	b := &TabItem{Text: "b", Content: widget.NewLabel("b")}
	d := NewDocTabs(a, b)
	d.SelectIndex(0)
	var fired []string
	d.OnSelected = func(ti *TabItem) { fired = append(fired, "selected "+ti.Text) }
	d.OnUnselected = func(ti *TabItem) { fired = append(fired, "unselected "+ti.Text) }
	d.OnClosed = func(ti *TabItem) { fired = append(fired, "closed "+ti.Text) }

	d.close(a)
	assert.Equal(t, []string{"unselected a", "selected b", "closed a"}, fired)
}
