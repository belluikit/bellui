package container_test

import (
	"net/url"
	"testing"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
)

// content is what an app puts in a container: a link and a label, which have
// no disabled look of their own, beside a Button and an Entry, which do.
func content() core.CanvasObject {
	u, _ := url.Parse("https://example.com")
	e := widget.NewEntry()
	e.SetText("entry text")
	return container.NewVBox(widget.NewHyperlink("A link", u), widget.NewLabel("A label"), widget.NewButton("A button", nil), e)
}

// Inside a disabled AppTabs and InnerWindow, a link and a label are drawn
// dimmed, as the Button and Entry beside them draw their own disabled look,
// and are not dimmed twice. The link and label stayed at full colour while
// taking no input (#1393).
func TestDisabledContainer_DimsContent(t *testing.T) {
	for _, v := range []struct {
		name    string
		variant bellui.ThemeVariant
	}{{"light", theme.VariantLight}, {"dark", theme.VariantDark}} {
		t.Run(v.name, func(t *testing.T) {
			a := test.NewTempApp(t)
			test.ApplyTheme(t, theme.DefaultTheme())
			a.Settings().(interface{ SetThemeVariant(bellui.ThemeVariant) }).SetThemeVariant(v.variant)

			tabs := container.NewAppTabs(container.NewTabItem("Tab", content()))
			inner := container.NewInnerWindow("Window", content())
			w := test.NewTempWindow(t, container.NewGridWithColumns(2, tabs, inner))
			w.Resize(geo.NewSize(560, 300))
			tabs.Disable()
			inner.Disable()
			test.AssertRendersToImage(t, "disable/dims_content_"+v.name+".png", w.Canvas())
		})
	}
}

// Widgets that draw their own disabled look, including the parts they build
// (a segment, a date cell, a tree-view row and its node), are not dimmed a
// second time by the rule that dims inert content (#1393).
func TestDisabledContainer_OwnLooksNotDimmedTwice(t *testing.T) {
	a := test.NewTempApp(t)
	test.ApplyTheme(t, theme.DefaultTheme())
	a.Settings().(interface{ SetThemeVariant(bellui.ThemeVariant) }).SetThemeVariant(theme.VariantLight)

	tv := widget.NewTreeView()
	_ = tv.AddNode("", widget.TreeViewNode{ID: "n", Text: "Tree view node"})
	picker := widget.NewDatePicker(time.Date(2026, 3, 9, 0, 0, 0, 0, time.UTC), nil)
	tabs := container.NewAppTabs(container.NewTabItem("Tab", container.NewVBox(
		widget.NewSegmentedButton([]string{"Day", "Week"}, nil),
		container.NewGridWrap(geo.NewSize(260, 60), tv),
		picker,
	)))
	w := test.NewTempWindow(t, tabs)
	w.Resize(geo.NewSize(360, 560))
	tabs.Disable()
	test.AssertRendersToImage(t, "disable/own_looks_not_dimmed_twice.png", w.Canvas())
}
