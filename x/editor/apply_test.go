package editor

import (
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// TestApplyGutter_AfterRender exercises the #281 re-apply path: installing,
// swapping and removing a gutter on an editor that has already rendered.
func TestApplyGutter_AfterRender(t *testing.T) {
	test.NewTempApp(t)
	e := NewEditor()
	e.SetText("one\ntwo\nthree")
	win := test.NewWindow(e)
	defer win.Close()
	win.Resize(geo.NewSize(300, 200))

	// NewEditor defaults to a line-number gutter, applied by CreateRenderer.
	if e.gutterW == nil {
		t.Fatal("default line-number gutter was not applied at first render")
	}
	if got := len(e.hbox.Objects); got != 2 {
		t.Fatalf("hbox holds %d objects, want 2 (gutter + center)", got)
	}

	// Remove after first render.
	e.Gutter = nil
	e.ApplyGutter()
	if e.gutterW != nil {
		t.Error("ApplyGutter(nil) left a gutter widget installed")
	}
	if e.entry.SuppressGutter {
		t.Error("entry's built-in gutter still suppressed after removing the gutter")
	}
	if got := len(e.hbox.Objects); got != 1 {
		t.Errorf("hbox holds %d objects after removal, want 1 (center)", got)
	}

	// Install after first render — the player use case: a reconfigured
	// composite replacing whatever the editor was built with.
	comp := NewCompositeGutter(NewLineNumberGutter(), NewBreakpointGutter(nil))
	e.Gutter = comp
	e.ApplyGutter()
	if e.gutterW == nil {
		t.Fatal("ApplyGutter did not build the gutter widget")
	}
	if e.gutterW.provider != GutterProvider(comp) {
		t.Error("gutter widget does not capture the newly assigned provider")
	}
	if !e.entry.SuppressGutter {
		t.Error("entry's built-in gutter not suppressed after installing a gutter")
	}
	if got := len(e.hbox.Objects); got != 2 {
		t.Fatalf("hbox holds %d objects after install, want 2 (gutter + center)", got)
	}

	// Swap the provider once more.
	ln := NewLineNumberGutter()
	e.Gutter = ln
	e.ApplyGutter()
	if e.gutterW == nil || e.gutterW.provider != GutterProvider(ln) {
		t.Error("ApplyGutter did not re-read the swapped provider")
	}
}

// TestApplyDecorators_AfterRender re-reads Decorators and SpanDecorators on a
// rendered editor and rebuilds the overlay, keeping the internal decorators.
func TestApplyDecorators_AfterRender(t *testing.T) {
	test.NewTempApp(t)
	e := NewEditor()
	e.SetText("one\ntwo\nthree")
	win := test.NewWindow(e)
	defer win.Close()
	win.Resize(geo.NewSize(300, 200))

	if e.decOverlay == nil {
		t.Fatal("expected the decorator overlay to exist after first render")
	}
	if got := len(e.decOverlay.decorators); got != 0 {
		t.Fatalf("bare editor overlay holds %d row decorators, want 0", got)
	}
	internalSpans := len(e.decOverlay.spanDecorators)
	if internalSpans == 0 {
		t.Fatal("expected internal span decorators (search matches etc.) on a bare editor")
	}

	cur := NewCurrentLineDecorator(1)
	spans := &StaticSpanDecorator{}
	e.Decorators = []RowDecorator{cur}
	e.SpanDecorators = []SpanDecorator{spans}
	prev := e.decOverlay

	// A plain repaint does not re-read the fields — the overlay holds the
	// slices it was built with. This is what RefreshDecorators' doc promises
	// and the reason ApplyDecorators exists.
	e.Refresh()
	e.RefreshDecorators()
	if got := len(e.decOverlay.decorators); got != 0 {
		t.Errorf("repaint-only refresh picked up %d row decorators, want 0", got)
	}

	e.ApplyDecorators()

	if e.decOverlay == prev {
		t.Fatal("ApplyDecorators did not rebuild the overlay")
	}
	if got := len(e.decOverlay.decorators); got != 1 {
		t.Errorf("overlay holds %d row decorators, want 1", got)
	}
	if got := len(e.decOverlay.spanDecorators); got != internalSpans+1 {
		t.Errorf("overlay holds %d span decorators, want host's 1 + the %d internal ones",
			got, internalSpans)
	}
	if e.decOverlay.spanDecorators[0] != SpanDecorator(spans) {
		t.Error("host span decorators should come first, before the internal set")
	}

	// The rebuilt overlay must actually be in the layout — the center stack the
	// hbox holds — or it would never paint.
	center, ok := e.hbox.Objects[len(e.hbox.Objects)-1].(*bellui.Container)
	if !ok {
		t.Fatal("expected the hbox's last object to be the center stack container")
	}
	found := false
	for _, o := range center.Objects {
		if o == core.CanvasObject(e.decOverlay) {
			found = true
			break
		}
	}
	if !found {
		t.Error("rebuilt overlay is not part of the editor's center stack")
	}
}

// TestApplyGutter_BeforeRender is the degenerate order: a host may configure
// and apply before the first show; CreateRenderer then applies the same state
// again and everything still works.
func TestApplyGutter_BeforeRender(t *testing.T) {
	test.NewTempApp(t)
	e := NewEditor()
	e.SetText("one\ntwo")
	ln := NewLineNumberGutter()
	e.Gutter = ln
	e.ApplyGutter()
	e.ApplyDecorators()

	win := test.NewWindow(e)
	defer win.Close()
	win.Resize(geo.NewSize(300, 200))

	if e.gutterW == nil || e.gutterW.provider != GutterProvider(ln) {
		t.Error("gutter not installed after pre-render ApplyGutter + show")
	}
	if got := len(e.hbox.Objects); got != 2 {
		t.Errorf("hbox holds %d objects, want 2", got)
	}
	// The pre-render ApplyDecorators is equally harmless: CreateRenderer built
	// the overlay again, internal decorators and all.
	if e.decOverlay == nil {
		t.Fatal("no decorator overlay after pre-render ApplyDecorators + show")
	}
	if len(e.decOverlay.spanDecorators) == 0 {
		t.Error("internal span decorators lost across pre-render ApplyDecorators + show")
	}
}

// TestApply_PreservesScrollOffset pins the offset invariant of a rebuild: the
// gutter widget and decorator overlay are new objects, and a scrolled editor
// must hand them its current offset — OnScrolled only fires on a move, so a
// widget left at 0 would draw (and hit-test) a viewport off. It drives the
// unexported helpers on purpose: the exported wrappers' refresh chain happens
// to re-notify OnScrolled today, which would mask the invariant rather than
// test it.
func TestApply_PreservesScrollOffset(t *testing.T) {
	test.NewTempApp(t)
	e := NewEditor()
	e.SetText("one\ntwo\nthree\nfour\nfive\nsix\nseven\neight")
	win := test.NewWindow(e)
	defer win.Close()
	win.Resize(geo.NewSize(300, 100))

	const off = 42
	scrollTo(e, off)

	e.Gutter = NewLineNumberGutter()
	e.applyGutter()
	if e.gutterW.scrollY != off {
		t.Errorf("re-applied gutter starts at scrollY %v, want the editor's %v",
			e.gutterW.scrollY, float32(off))
	}

	e.Decorators = []RowDecorator{NewCurrentLineDecorator(1)}
	e.applyDecorators()
	if e.decOverlay.scrollY != off {
		t.Errorf("re-applied overlay starts at scrollY %v, want the editor's %v",
			e.decOverlay.scrollY, float32(off))
	}
}
