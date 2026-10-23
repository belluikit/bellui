package editor

import (
	"testing"

	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// The editor's own shortcuts — save, run, find, find next/previous — use the
// platform's primary modifier: Cmd on macOS, Ctrl elsewhere (#339). They were
// bound to Super, which on Linux and Windows is the Windows key, so Ctrl+S
// did nothing there although OnSave's doc promised it.
func TestShortcutsUsePrimaryModifier(t *testing.T) {
	test.NewTempApp(t)
	e := NewEditor()
	w := test.NewWindow(e)
	defer w.Close()
	w.Resize(geo.NewSize(400, 300))
	e.SetText("alpha beta\nalpha gamma\ndelta")

	saved, ran := 0, 0
	e.OnSave = func() { saved++ }
	e.OnRun = func() { ran++ }

	primary := func(key event.KeyName, extra event.KeyModifier) *desktop.CustomShortcut {
		return &desktop.CustomShortcut{KeyName: key, Modifier: event.KeyModifierShortcutDefault | extra}
	}

	e.entry.TypedShortcut(primary(event.KeyS, 0))
	if saved != 1 {
		t.Errorf("primary+S fired OnSave %d times, want 1", saved)
	}
	e.entry.TypedShortcut(primary(event.KeyR, 0))
	if ran != 1 {
		t.Errorf("primary+R fired OnRun %d times, want 1", ran)
	}

	e.entry.TypedShortcut(primary(event.KeyF, 0))
	if !e.findVisible {
		t.Fatal("primary+F did not open the find bar")
	}
	e.find.entry.Text = "alpha"
	e.find.updateMatches()
	if e.searchSpans.Active != 0 {
		t.Fatalf("active match = %d after search, want 0", e.searchSpans.Active)
	}
	e.entry.TypedShortcut(primary(event.KeyG, 0))
	if e.searchSpans.Active != 1 {
		t.Errorf("primary+G on the editor: active match = %d, want 1", e.searchSpans.Active)
	}
	e.find.entry.TypedShortcut(primary(event.KeyG, event.KeyModifierShift))
	if e.searchSpans.Active != 0 {
		t.Errorf("primary+Shift+G in the find bar: active match = %d, want 0", e.searchSpans.Active)
	}
	e.find.entry.TypedShortcut(primary(event.KeyG, 0))
	if e.searchSpans.Active != 1 {
		t.Errorf("primary+G in the find bar: active match = %d, want 1", e.searchSpans.Active)
	}

	// Where the primary modifier is Ctrl, Super is the Windows key and must
	// not save.
	if event.KeyModifierShortcutDefault != event.KeyModifierSuper {
		e.entry.TypedShortcut(&desktop.CustomShortcut{KeyName: event.KeyS, Modifier: event.KeyModifierSuper})
		if saved != 1 {
			t.Errorf("Super+S fired OnSave on a platform whose primary modifier is Ctrl")
		}
	}
}
