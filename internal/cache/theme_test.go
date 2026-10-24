package cache_test

import (
	"image/color"
	"testing"
	"time"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/test"
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
