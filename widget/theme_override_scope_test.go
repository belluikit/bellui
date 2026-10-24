package widget_test

import (
	"fmt"
	"image/color"
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// overrideColor is the separator color of the theme the tests below apply, so
// a separator drawn with the app theme is told apart from one drawn with it.
var overrideColor = color.NRGBA{R: 0xff, G: 0x00, B: 0xff, A: 0xff}

type separatorTheme struct{ bellui.Theme }

func (s separatorTheme) Color(n bellui.ThemeColorName, v bellui.ThemeVariant) color.Color {
	if n == theme.ColorNameOutlineVariant {
		return overrideColor
	}
	return s.Theme.Color(n, v)
}

// unscopedWidgets walks o through the renderers its widgets already have (it
// never creates one, so it sees the tree the widget built itself) and names
// every widget whose theme is not th.
func unscopedWidgets(o core.CanvasObject, th bellui.Theme) (missing []string, widgets int) {
	var walk func(core.CanvasObject)
	walk = func(o core.CanvasObject) {
		switch c := o.(type) {
		case core.Widget:
			widgets++
			if cache.WidgetTheme(c) != th {
				missing = append(missing, fmt.Sprintf("%T", c))
			}
			if !cache.IsRendered(c) {
				return
			}
			for _, child := range cache.Renderer(c).Objects() {
				walk(child)
			}
		case *bellui.Container:
			for _, child := range c.Objects {
				walk(child)
			}
		}
	}
	walk(o)
	return missing, widgets
}

// wrongColorSeparators counts the rendered separators under o that do not
// draw with the override's color: what the screen shows, whatever the scope
// map holds.
func wrongColorSeparators(o core.CanvasObject) (wrong, total int) {
	var walk func(core.CanvasObject)
	walk = func(o core.CanvasObject) {
		switch c := o.(type) {
		case *widget.Separator:
			if !cache.IsRendered(c) {
				return
			}
			total++
			bar := cache.Renderer(c).Objects()[0].(*canvas.Rectangle)
			if bar.FillColor != color.Color(overrideColor) {
				wrong++
			}
		case core.Widget:
			if !cache.IsRendered(c) {
				return
			}
			for _, child := range cache.Renderer(c).Objects() {
				walk(child)
			}
		case *bellui.Container:
			for _, child := range c.Objects {
				walk(child)
			}
		}
	}
	walk(o)
	return wrong, total
}

// collectionsUnderTest builds each collection widget that creates child
// objects as it grows: dividers, separators, headers, row wrappers.
func collectionsUnderTest() map[string]func() core.CanvasObject {
	label := func() core.CanvasObject { return widget.NewLabel("cell") }
	return map[string]func() core.CanvasObject{
		"Table": func() core.CanvasObject {
			t := widget.NewTableWithHeaders(
				func() (int, int) { return 50, 10 },
				label,
				func(widget.TableCellID, core.CanvasObject) {},
			)
			return t
		},
		"Tree": func() core.CanvasObject {
			t := widget.NewTreeWithStrings(map[string][]string{"": treeIDs(60)})
			return t
		},
		"TreeView": func() core.CanvasObject {
			nodes := make([]widget.TreeViewNode, 60)
			for i := range nodes {
				nodes[i] = widget.TreeViewNode{ID: widget.TreeViewNodeID(fmt.Sprint(i)), Text: "node"}
			}
			return widget.NewTreeViewWithNodes(nodes)
		},
		// Each open branch that has more children to load shows an Activity.
		"TreeView/loadMore": func() core.CanvasObject {
			tv := widget.NewTreeView()
			for i := 0; i < 30; i++ {
				id := widget.TreeViewNodeID(fmt.Sprint(i))
				_ = tv.AddNode("", widget.TreeViewNode{ID: id, Text: "branch"})
				_ = tv.AddNode(id, widget.TreeViewNode{ID: id + "/leaf", Text: "leaf"})
				tv.SetHasMore(id, true)
			}
			tv.OpenAllBranches()
			return tv
		},
		"List": func() core.CanvasObject {
			return widget.NewList(func() int { return 60 }, label, func(widget.ListItemID, core.CanvasObject) {})
		},
		"GridWrap": func() core.CanvasObject {
			return widget.NewGridWrap(func() int { return 200 }, label, func(widget.GridWrapItemID, core.CanvasObject) {})
		},
	}
}

func treeIDs(n int) []string {
	ids := make([]string, n)
	for i := range ids {
		ids[i] = fmt.Sprint("n", i)
	}
	return ids
}

// TestThemeOverride_ReachesObjectsMadeAfterIt grows each collection after a
// theme override was applied, so it creates new child objects, and checks
// that every widget it renders carries the override (#755).
func TestThemeOverride_ReachesObjectsMadeAfterIt(t *testing.T) {
	for name, build := range collectionsUnderTest() {
		t.Run(name, func(t *testing.T) {
			overrideThenGrow(t, build, false)
		})
		// Shrinking first parks rows, dividers and separators made under no
		// override in the widget's pools, where the override's walk does not
		// reach them; growing again reuses them.
		t.Run(name+"/pooled", func(t *testing.T) {
			overrideThenGrow(t, build, true)
		})
	}
}

func overrideThenGrow(t *testing.T, build func() core.CanvasObject, shrinkFirst bool) {
	test.NewTempApp(t)
	th := separatorTheme{test.Theme()}

	o := build()
	w := test.NewTempWindow(t, o)
	if shrinkFirst {
		w.Resize(geo.NewSize(600, 600))
	}
	w.Resize(geo.NewSize(100, 100))
	cache.OverrideTheme(o, th)
	missing, before := unscopedWidgets(o, th)
	require.Empty(t, missing, "the override did not reach the widgets that existed when it was applied")

	w.Resize(geo.NewSize(600, 600))
	o.Refresh()

	missing, after := unscopedWidgets(o, th)
	require.Greater(t, after, before, "growing made no new widgets, so this measures nothing")
	assert.Empty(t, missing, "%d widgets shown after the override do not carry it", len(missing))
}

// TestThemeOverrideContainer_ReachesObjectsMadeAfterIt asks the same of
// container.NewThemeOverride, the API a real app uses. It re-applies its scope
// only in its own CreateRenderer and Refresh, which a collection growing
// inside it does not call, so that does not hide the bug (#755).
func TestThemeOverrideContainer_ReachesObjectsMadeAfterIt(t *testing.T) {
	for name, build := range collectionsUnderTest() {
		t.Run(name, func(t *testing.T) {
			test.NewTempApp(t)

			o := build()
			over := container.NewThemeOverride(o, separatorTheme{test.Theme()})
			w := test.NewTempWindow(t, over)
			w.Resize(geo.NewSize(100, 100))
			th := cache.WidgetTheme(o)
			require.NotNil(t, th)
			_, before := unscopedWidgets(o, th)

			w.Resize(geo.NewSize(600, 600))
			missing, after := unscopedWidgets(o, th)
			require.Greater(t, after, before, "growing made no new widgets, so this measures nothing")
			assert.Empty(t, missing, "%d widgets shown after the override do not carry it", len(missing))

			// A separator made during the override's layout picks up its
			// color from the theme pushed for that layout, so the bug hides
			// until something refreshes it outside one, as this does.
			o.Refresh()
			wrong, total := wrongColorSeparators(over)
			assert.Zero(t, wrong, "%d of %d separators draw with the app theme's color", wrong, total)
		})
	}
}

type bigTextTheme struct{ bellui.Theme }

func (b bigTextTheme) Size(n bellui.ThemeSizeName) float32 {
	if n == theme.SizeNameText {
		return 40
	}
	return b.Theme.Size(n)
}

// TestThemeOverride_TableHeadersSizedByIt checks that a Table measures its
// header template under its override, not the app theme: a header laid out
// smaller than it asks for is clipped (#755).
func TestThemeOverride_TableHeadersSizedByIt(t *testing.T) {
	test.NewTempApp(t)
	var made []*widget.Label
	table := widget.NewTableWithHeaders(
		func() (int, int) { return 5, 5 },
		func() core.CanvasObject { return widget.NewLabel("x") },
		func(widget.TableCellID, core.CanvasObject) {},
	)
	table.CreateHeader = func() core.CanvasObject {
		l := widget.NewLabel("Header")
		made = append(made, l)
		return l
	}
	table.UpdateHeader = func(widget.TableCellID, core.CanvasObject) {}
	w := test.NewTempWindow(t, container.NewThemeOverride(table, bigTextTheme{test.Theme()}))
	w.Resize(geo.NewSize(800, 800))
	table.Refresh()

	shown := 0
	for _, l := range made {
		if !l.Visible() || l.Size().IsZero() {
			continue // a template, or a header in the pool
		}
		shown++
		assert.GreaterOrEqual(t, l.Size().Height, l.MinSize().Height, "a header is laid out shorter than it asks for")
		assert.GreaterOrEqual(t, l.Size().Width, l.MinSize().Width, "a header is laid out narrower than it asks for")
	}
	require.NotZero(t, shown, "no header was shown, so this measures nothing")
}
