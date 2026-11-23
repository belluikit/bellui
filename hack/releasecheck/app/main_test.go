package main

import (
	"slices"
	"strings"
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
)

// Every tab builds and lays out in both variants, so the checklist never
// points a tester at a screen that cannot open.
func TestTabsBuild(t *testing.T) {
	for _, v := range []bellui.ThemeVariant{theme.VariantLight, theme.VariantDark} {
		a := test.NewTempApp(t)
		a.Settings().(interface{ SetThemeVariant(bellui.ThemeVariant) }).SetThemeVariant(v)
		w := test.NewTempWindow(t, nil)
		for name, tab := range map[string]core.CanvasObject{
			"Locale": localeTab(w), "Direction": directionTab(w), "Theme override": overrideTab(),
			"Menus": menusTab(w), "Touch & keys": touchTab(),
			"Disabled": disabledTab(), "Drawer": drawerTab(w), "Swipes": swipesTab(), "Text": textTab(w), "Chips & layout": chipsTab(),
		} {
			w.SetContent(tab)
			w.Resize(geo.NewSize(700, 560))
			if tab.Size().Width == 0 {
				t.Errorf("variant %d: the %s tab has no size", v, name)
			}
		}
	}
}

// The greeting the Locale tab shows for Persian is the English text, not the
// app's Arabic: the release checklist's I18N-03 expects it (#342).
func TestPersianGetsEnglishGreeting(t *testing.T) {
	test.NewTempApp(t)
	if err := lang.AddTranslationsForLocale([]byte(greetingEN), "en"); err != nil {
		t.Fatal(err)
	}
	if err := lang.AddTranslationsForLocale([]byte(greetingAR), "ar"); err != nil {
		t.Fatal(err)
	}
	for loc, want := range map[string]string{"fa-IR": "Hello Ada", "ar-EG": "مرحبا Ada"} {
		lang.SetLocaleForTests(t, loc)
		if got := lang.X("release.greeting", "Hello {Name}", map[string]any{"Name": "Ada"}); got != want {
			t.Errorf("%s: greeting %q, want %q", loc, got, want)
		}
	}
}

// The Chips tab's group cycles a chip through its three states, and refuses
// to exclude the last style, as the checklist's WG-07 expects (#1383).
func TestExcludableCardRefusesTheLastStyle(t *testing.T) {
	test.NewTempApp(t)
	card := excludableCard().(*widget.Card)
	box := card.Content.(*bellui.Container)
	g := box.Objects[0].(*widget.ChipGroup)
	tapChip := func(opt string) {
		for _, o := range test.TempWidgetRenderer(t, g).Objects() {
			if c := o.(*widget.Chip); c.Text == opt {
				test.Tap(c)
				return
			}
		}
		t.Fatalf("no chip %q", opt)
	}
	for _, opt := range g.Options[:len(g.Options)-1] {
		tapChip(opt)
		tapChip(opt)
	}
	if want := g.Options[:len(g.Options)-1]; !slices.Equal(g.Excluded, want) {
		t.Fatalf("excluded %v, want %v", g.Excluded, want)
	}
	last := g.Options[len(g.Options)-1]
	tapChip(last)
	tapChip(last)
	if !slices.Equal(g.Selected, []string{last}) || slices.Contains(g.Excluded, last) {
		t.Errorf("the last style was excluded: selected %v, excluded %v", g.Selected, g.Excluded)
	}
	status := box.Objects[1].(*widget.Label).Text
	if !strings.Contains(status, "cannot be excluded") {
		t.Errorf("status does not say the change was refused: %q", status)
	}
}
