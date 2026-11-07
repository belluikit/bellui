package cache_test

import (
	"image/color"
	"runtime"
	"testing"
	"time"
	"weak"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
)

func TestDroppedOverrideScopesReleaseTheirObjects(t *testing.T) {
	test.NewApp()
	before := cache.OverriddenObjects()
	for i := 0; i < 100; i++ {
		label := widget.NewLabel("item")
		cache.OverrideTheme(label, test.NewTheme())
		test.DestroyRenderers(t, label)
	}
	if left := cache.OverriddenObjects() - before; left != 0 {
		t.Fatalf("dropping 100 overridden subtrees left %d override entries behind", left)
	}
}

func TestAnInnerWidgetKeepsItsScopeUntilItsOwnRendererGoes(t *testing.T) {
	test.NewApp()
	before := cache.OverriddenObjects()
	oldDuration := cache.ValidDuration
	cache.ValidDuration = -time.Nanosecond // entries are stamped already-expired against the current sample
	defer func() { cache.ValidDuration = oldDuration }()

	label := widget.NewLabel("kept")
	cache.OverrideTheme(label, test.NewTheme())
	cache.DestroyRenderer(label) // the label's own part of the walk is released
	if got := cache.OverriddenObjects() - before; got == 0 {
		t.Fatal("the label's inner widgets should still hold their renderers, and their entries with them")
	}
	cache.Clean(true) // expires the inner widgets' renderers
	if left := cache.OverriddenObjects() - before; left != 0 {
		t.Fatalf("every renderer in the subtree is gone but %d override entries are still held", left)
	}
}

func TestADroppedContainerSubtreeReleasesEverythingButTheContainer(t *testing.T) {
	test.NewApp()
	before := cache.OverriddenObjects()
	box := container.NewVBox(widget.NewLabel("item"), canvas.NewRectangle(color.White))
	cache.OverrideTheme(box, test.NewTheme())
	if got := cache.OverriddenObjects() - before; got == 0 {
		t.Fatal("the overridden container stored no entries")
	}
	test.DestroyRenderers(t, box)
	// A container has no renderer to destroy; its own entry remains until
	// CleanCanvas releases it when the window it belongs to closes.
	if left := cache.OverriddenObjects() - before; left != 1 {
		t.Fatalf("the dropped container subtree left %d override entries behind, want only the container itself", left)
	}
}

// A container has no renderer, so its part of a ThemeOverride's scope was
// released only when its window closed: a phone app, with one window, kept
// every screen it rebuilt under an override (#1065). The ThemeOverride owns
// the scope now, and a dropped one frees its content with the window open.
// The plain VBox is the control: it shows the test sees an object freed.
func TestADroppedThemeOverrideFreesItsContainer(t *testing.T) {
	for _, c := range []struct {
		name            string
		override, shown bool
	}{
		{"no override", false, false},
		{"never shown", true, false},
		{"shown then replaced", true, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			test.NewApp()
			oldDuration := cache.ValidDuration
			cache.ValidDuration = -time.Nanosecond // entries are stamped already-expired against the current sample
			defer func() { cache.ValidDuration = oldDuration }()
			w := test.NewWindow(widget.NewLabel("first"))
			defer w.Close()

			freed := buildAndDrop(w, c.override, c.shown)
			cache.CleanCanvas(test.NewCanvas()) // the expiry sweep, without Clean's 10 s throttle; w stays open
			for range 5 {
				runtime.GC()
			}
			if freed.Value() != nil {
				t.Fatal("a dropped VBox is still held with its window open")
			}
		})
	}
}

// buildAndDrop makes a VBox, puts it under a ThemeOverride when override,
// shows that in w and replaces it when shown, and returns a weak pointer to
// the VBox.
func buildAndDrop(w bellui.Window, override, shown bool) weak.Pointer[bellui.Container] {
	box := container.NewVBox(widget.NewLabel("item"), canvas.NewRectangle(color.White))
	var content core.CanvasObject = box
	if override {
		content = container.NewThemeOverride(box, theme.DefaultTheme())
	}
	if shown {
		w.SetContent(content)
		w.Content().Refresh()
		w.SetContent(widget.NewLabel("next"))
	}
	return weak.Make(box)
}
