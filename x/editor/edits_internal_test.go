package editor

import (
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/test"
)

// TestApplyEdit_UserEditSemantics pins the pre-rebrand tracker #160 contract for a single splice:
// the host OnChanged fires once with the full new text, the caret lands after
// the replacement, and ONE undo restores the original (SetText would have
// cleared the history instead).
func TestApplyEdit_UserEditSemantics(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "hello world")

	changed := 0
	var last string
	e.OnChanged = func(s string) { changed++; last = s }

	if !e.ApplyEdit(TextRange{StartRow: 0, StartCol: 6, EndRow: 0, EndCol: 11}, "editor") {
		t.Fatal("ApplyEdit should apply")
	}
	if e.Text() != "hello editor" || changed != 1 || last != "hello editor" {
		t.Fatalf("text=%q changed=%d last=%q, want one OnChanged with %q", e.Text(), changed, last, "hello editor")
	}
	if r, c := e.CursorPos(); r != 0 || c != 12 {
		t.Errorf("caret should sit after the replacement (0,12), got (%d,%d)", r, c)
	}

	e.entry.TypedShortcut(&bellui.ShortcutUndo{})
	if e.Text() != "hello world" {
		t.Errorf("one undo should restore the original, got %q", e.Text())
	}
}

// TestApplyEdits_BatchIsAtomicSingleUndo: a multi-splice batch lands as one
// edit — one OnChanged, one undo step.
func TestApplyEdits_BatchIsAtomicSingleUndo(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "aaa\nbbb\nccc")

	changed := 0
	e.OnChanged = func(string) { changed++ }

	ok := e.ApplyEdits([]TextEdit{
		{Range: TextRange{StartRow: 0, StartCol: 0, EndRow: 0, EndCol: 3}, Text: "AAA"},
		{Range: TextRange{StartRow: 2, StartCol: 0, EndRow: 2, EndCol: 3}, Text: "CCC"},
	})
	if !ok || e.Text() != "AAA\nbbb\nCCC" || changed != 1 {
		t.Fatalf("ok=%v text=%q changed=%d, want one atomic batch", ok, e.Text(), changed)
	}

	e.entry.TypedShortcut(&bellui.ShortcutUndo{})
	if e.Text() != "aaa\nbbb\nccc" {
		t.Errorf("one undo should revert the whole batch, got %q", e.Text())
	}
}

// TestApplyEdits_ProtectedRangeVetoesWholeBatch: one splice touching a
// protected range rejects the batch all-or-nothing.
func TestApplyEdits_ProtectedRangeVetoesWholeBatch(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "aaa\nSECRET\nccc")
	e.SetProtectedRanges([]TextRange{{StartRow: 1, StartCol: 0, EndRow: 1, EndCol: 6}})

	ok := e.ApplyEdits([]TextEdit{
		{Range: TextRange{StartRow: 0, StartCol: 0, EndRow: 0, EndCol: 3}, Text: "AAA"},
		{Range: TextRange{StartRow: 1, StartCol: 2, EndRow: 1, EndCol: 4}, Text: "xx"},
	})
	if ok || e.Text() != "aaa\nSECRET\nccc" {
		t.Errorf("a protected splice must veto the whole batch, ok=%v text=%q", ok, e.Text())
	}
}

// TestApplyEdit_AnchorTracksRanges: an insert above a protected range shifts
// it (pre-rebrand tracker #130) — no host re-publish, unlike SetText's document-swap contract.
func TestApplyEdit_AnchorTracksRanges(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "aaa\nSECRET\nccc")
	e.SetProtectedRanges([]TextRange{{StartRow: 1, StartCol: 0, EndRow: 1, EndCol: 6}})

	if !e.ApplyEdit(TextRange{StartRow: 0, StartCol: 0, EndRow: 0, EndCol: 0}, "new\n") {
		t.Fatal("edit above the range should apply")
	}
	want := TextRange{StartRow: 2, StartCol: 0, EndRow: 2, EndCol: 6}
	if got := e.ProtectedRanges(); len(got) != 1 || got[0] != want {
		t.Fatalf("ProtectedRanges after insert-above = %v, want [%v]", got, want)
	}
}

// TestApplyEdits_EmptyAndNoop: an empty batch reports false and fires nothing.
func TestApplyEdits_EmptyAndNoop(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "abc")
	changed := 0
	e.OnChanged = func(string) { changed++ }

	if e.ApplyEdits(nil) {
		t.Error("an empty batch must report false")
	}
	if e.ApplyEdits([]TextEdit{{Range: TextRange{}, Text: ""}}) {
		t.Error("an all-no-op batch must report false")
	}
	if changed != 0 {
		t.Errorf("no-op batches must not fire OnChanged, got %d", changed)
	}
}

// TestApplyEdit_NoCompletionRetrigger: a programmatic edit does not open the
// completion popup, matching completion acceptance.
func TestApplyEdit_NoCompletionRetrigger(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "")
	e.SetCompletionProvider(&WordListCompletionProvider{Words: []string{"alpha"}})

	if !e.ApplyEdit(TextRange{}, "al") {
		t.Fatal("insert should apply")
	}
	if e.completion.visible() {
		t.Error("a programmatic edit must not trigger completion")
	}
}

// TestApplyEdit_DeferredModeSyncs: ApplyEdits is a sync point under
// DeferTextSync — Entry.Text and OnChanged observe the result immediately.
func TestApplyEdit_DeferredModeSyncs(t *testing.T) {
	test.NewTempApp(t)
	e := newDeferredEditor(t, "hello")

	changed := 0
	e.OnChanged = func(string) { changed++ }

	if !e.ApplyEdit(TextRange{StartRow: 0, StartCol: 0, EndRow: 0, EndCol: 5}, "bye") {
		t.Fatal("edit should apply")
	}
	if e.entry.Text != "bye" || changed != 1 {
		t.Errorf("ApplyEdits should be a deferred-mode sync point: text=%q changed=%d", e.entry.Text, changed)
	}
}
