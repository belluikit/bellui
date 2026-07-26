package editor

import (
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
)

// TestNaturalRichSize_RecoversWidthUnderWordWrap guards the hover/signature card
// sizing: a word-wrapping RichText reports MinSize().Width as ~0 (fyne computes
// the content width only with wrapping off), so naturalRichSize must measure the
// wider unwrapped width and restore the widget's Wrapping. Without this the cards
// collapse to ~1 char wide.
func TestNaturalRichSize_RecoversWidthUnderWordWrap(t *testing.T) {
	test.NewTempApp(t)
	rt := widget.NewRichTextFromMarkdown("the quick brown fox jumps over the lazy dog")
	rt.Wrapping = bellui.TextWrapWord

	natural := naturalRichSize(rt)
	if natural.Width <= 0 {
		t.Fatalf("naturalRichSize width = %v, want > 0", natural.Width)
	}
	// The word-wrapping MinSize collapses width to ~padding; the helper recovers
	// the real unwrapped content width.
	if wrapped := rt.MinSize().Width; natural.Width <= wrapped {
		t.Errorf("naturalRichSize width %v should exceed wrap-word MinSize width %v", natural.Width, wrapped)
	}
	// Wrapping must be left as the caller set it.
	if rt.Wrapping != bellui.TextWrapWord {
		t.Errorf("Wrapping = %v, want TextWrapWord (restored)", rt.Wrapping)
	}
}

// TestNaturalRichSize_RenderedAndWrapped is the regression for the live hover
// bug: on an already-rendered widget that has wrapped to a narrow width, flipping
// Wrapping without a refresh reads stale bounds and still reports ~0 width.
// naturalRichSize must recover the natural width regardless.
func TestNaturalRichSize_RenderedAndWrapped(t *testing.T) {
	test.NewTempApp(t)
	const content = "the quick brown fox jumps over the lazy dog"

	ref := widget.NewRichTextFromMarkdown(content)
	ref.Wrapping = bellui.TextWrapOff
	wantWidth := ref.MinSize().Width

	rt := widget.NewRichTextFromMarkdown(content)
	rt.Wrapping = bellui.TextWrapWord
	test.TempWidgetRenderer(t, rt)  // realize the renderer (rendered state)
	rt.Resize(geo.NewSize(40, 200)) // force wrapping into many narrow rows

	got := naturalRichSize(rt)
	if got.Width < wantWidth {
		t.Errorf("naturalRichSize width %v should recover natural width %v even when rendered+wrapped", got.Width, wantWidth)
	}
	if rt.Wrapping != bellui.TextWrapWord {
		t.Errorf("Wrapping = %v, want TextWrapWord (restored)", rt.Wrapping)
	}
}
