package app

import (
	"testing"

	"github.com/belluikit/bellui/internal/driver/mobile/event/key"
)

// A native key acts on its DOWN and its UP is the release. The two used to
// be swapped, so every key acted when released (#793).
func TestNativeKeyDirection(t *testing.T) {
	for _, c := range []struct {
		name   string
		action int32
		repeat int32
		code   key.Code
		dir    key.Direction
		ok     bool
	}{
		{"down", keyActionDown, 0, key.CodeA, key.DirPress, true},
		{"up", keyActionUp, 0, key.CodeA, key.DirRelease, true},
		{"held key repeats", keyActionDown, 3, key.CodeA, key.DirPress, true},
		{"held Backspace repeats", keyActionDown, 1, key.CodeDeleteBackspace, key.DirPress, true},
		{"Escape press", keyActionDown, 0, key.CodeEscape, key.DirPress, true},
		{"held Escape does not repeat", keyActionDown, 1, key.CodeEscape, key.DirNone, false},
		{"Escape release", keyActionUp, 0, key.CodeEscape, key.DirRelease, true},
		{"multiple", keyActionMultiple, 0, key.CodeUnknown, key.DirNone, true},
	} {
		dir, ok := nativeKeyDirection(c.action, c.repeat, c.code)
		if dir != c.dir || ok != c.ok {
			t.Errorf("%s: got (%v, %v), want (%v, %v)", c.name, dir, ok, c.dir, c.ok)
		}
	}
}
