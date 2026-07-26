package editor

import (
	"testing"

	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// renderedWordEditor builds an editor with a word-list completion provider,
// applying configure (which may set e.KeyMap) before the window forces
// CreateRenderer to install the keymap shortcuts.
func renderedWordEditor(t *testing.T, configure func(*Editor), words ...string) *Editor {
	t.Helper()
	test.NewTempApp(t)
	e := NewEditor()
	if configure != nil {
		configure(e)
	}
	e.SetCompletionProvider(&WordListCompletionProvider{Words: words})
	w := test.NewWindow(e)
	t.Cleanup(w.Close)
	w.Resize(geo.NewSize(400, 300))
	return e
}

func TestKeyMap_Defaults(t *testing.T) {
	km := DefaultKeyMap()

	if km.Completion.KeyName != event.KeySpace || km.Completion.Modifier != event.KeyModifierControl {
		t.Errorf("Completion default = %+v, want Ctrl+Space", km.Completion)
	}
	if want := event.KeyModifierShortcutDefault | event.KeyModifierShift; km.SignatureHelp.KeyName != event.KeySpace || km.SignatureHelp.Modifier != want {
		t.Errorf("SignatureHelp default = %+v, want ShortcutDefault+Shift+Space", km.SignatureHelp)
	}
	if km.CycleOverloadNext.KeyName != event.KeyDown || km.CycleOverloadNext.Modifier != event.KeyModifierAlt {
		t.Errorf("CycleOverloadNext default = %+v, want Alt+Down", km.CycleOverloadNext)
	}
	if km.CycleOverloadPrev.KeyName != event.KeyUp || km.CycleOverloadPrev.Modifier != event.KeyModifierAlt {
		t.Errorf("CycleOverloadPrev default = %+v, want Alt+Up", km.CycleOverloadPrev)
	}
	if km.GoToDefinition.KeyName != event.KeyF12 || km.GoToDefinition.Modifier != 0 {
		t.Errorf("GoToDefinition default = %+v, want bare F12", km.GoToDefinition)
	}
}

func TestKeyMap_NewEditorInstallsDefaults(t *testing.T) {
	e := NewEditor()
	if e.KeyMap != DefaultKeyMap() {
		t.Errorf("NewEditor should default KeyMap to DefaultKeyMap(), got %+v", e.KeyMap)
	}
}

// TestKeyMap_DefaultCompletionShortcutInvokes drives the default completion
// binding through the entry's real shortcut dispatch and asserts the popup opens.
func TestKeyMap_DefaultCompletionShortcutInvokes(t *testing.T) {
	e := renderedWordEditor(t, nil, "alpha", "beta", "gamma")

	e.entry.TypedShortcut(&desktop.CustomShortcut{
		KeyName:  event.KeySpace,
		Modifier: event.KeyModifierControl,
	})

	if !e.completion.visible() {
		t.Fatal("default Ctrl+Space should invoke completion")
	}
	if len(e.completion.shown) != 3 {
		t.Errorf("invoke should show all 3 words, got %d", len(e.completion.shown))
	}
}

// TestKeyMap_CustomCompletionBinding verifies a host can rebind completion: the
// custom shortcut opens it and the former default no longer does.
func TestKeyMap_CustomCompletionBinding(t *testing.T) {
	custom := desktop.CustomShortcut{KeyName: event.KeyE, Modifier: event.KeyModifierControl}
	e := renderedWordEditor(t, func(e *Editor) {
		e.KeyMap.Completion = custom
	}, "alpha", "beta")

	// The old default is no longer bound.
	e.entry.TypedShortcut(&desktop.CustomShortcut{
		KeyName:  event.KeySpace,
		Modifier: event.KeyModifierControl,
	})
	if e.completion.visible() {
		t.Fatal("rebinding completion should unbind the former Ctrl+Space default")
	}

	// The custom binding opens it.
	e.entry.TypedShortcut(&custom)
	if !e.completion.visible() {
		t.Error("the custom completion shortcut should invoke completion")
	}
}

// TestKeyMap_UnboundCompletionDisabled verifies clearing a binding disables the
// shortcut (auto-trigger-on-typing is a separate path and unaffected).
func TestKeyMap_UnboundCompletionDisabled(t *testing.T) {
	e := renderedWordEditor(t, func(e *Editor) {
		e.KeyMap.Completion = desktop.CustomShortcut{} // unbound
	}, "alpha", "beta")

	e.entry.TypedShortcut(&desktop.CustomShortcut{
		KeyName:  event.KeySpace,
		Modifier: event.KeyModifierControl,
	})
	if e.completion.visible() {
		t.Error("an unbound completion shortcut must not invoke completion")
	}
}
