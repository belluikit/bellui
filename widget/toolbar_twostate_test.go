package widget

import (
	"bytes"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
)

func TestTwoStateToolbarAction(t *testing.T) {
	test.NewTempApp(t)

	var got []bool
	action := NewTwoStateToolbarAction(theme.MediaPlayIcon(), theme.MediaPauseIcon(),
		func(on bool) { got = append(got, on) })

	button, ok := action.ToolbarObject().(*Button)
	require.True(t, ok)
	assert.Equal(t, theme.MediaPlayIcon(), button.Icon)
	assert.False(t, action.On())

	test.Tap(button)
	assert.True(t, action.On())
	assert.Equal(t, theme.MediaPauseIcon(), button.Icon)
	assert.Equal(t, []bool{true}, got)

	test.Tap(button)
	assert.False(t, action.On())
	assert.Equal(t, theme.MediaPlayIcon(), button.Icon)
	assert.Equal(t, []bool{true, false}, got)
}

func TestTwoStateToolbarAction_SetOn(t *testing.T) {
	test.NewTempApp(t)

	var got []bool
	action := NewTwoStateToolbarAction(theme.MediaPlayIcon(), theme.MediaPauseIcon(),
		func(on bool) { got = append(got, on) })
	button := action.ToolbarObject().(*Button)

	action.SetOn(true)
	assert.True(t, action.On())
	assert.Equal(t, theme.MediaPauseIcon(), button.Icon)
	assert.Equal(t, []bool{true}, got)

	action.SetOn(true)
	assert.Equal(t, []bool{true}, got, "no callback when the state did not change")

	action.SetOn(false)
	assert.Equal(t, []bool{true, false}, got)
}

func TestTwoStateToolbarAction_SetIcons(t *testing.T) {
	test.NewTempApp(t)

	action := NewTwoStateToolbarAction(theme.MediaPlayIcon(), theme.MediaPauseIcon(), nil)
	button := action.ToolbarObject().(*Button)

	action.SetOffIcon(theme.MediaRecordIcon())
	assert.Equal(t, theme.MediaRecordIcon(), button.Icon)

	action.SetOn(true)
	action.SetOnIcon(theme.MediaStopIcon())
	assert.Equal(t, theme.MediaStopIcon(), button.Icon)

	// In a Toolbar the item renders via ToolbarObject.
	tb := NewToolbar(action)
	w := test.NewWindow(tb)
	defer w.Close()
}

// assertNoExtendWarning runs f with the global logger captured and fails if
// the "used before ExtendBaseWidget was called" warning fires (#1425).
func assertNoExtendWarning(t *testing.T, f func()) {
	t.Helper()
	ResetExtendWarningsForTests(t)
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)
	f()
	if strings.Contains(buf.String(), "used before ExtendBaseWidget") {
		t.Errorf("ExtendBaseWidget warning logged:\n%s", buf.String())
	}
}

// The toolbar actions hold their button by value; a method called on it
// before it was extended logged the warning (#1425).
func TestToolbarActions_NoExtendBaseWidgetWarning(t *testing.T) {
	test.NewTempApp(t)
	t.Run("NewTwoStateToolbarAction", func(t *testing.T) {
		assertNoExtendWarning(t, func() {
			NewTwoStateToolbarAction(theme.MediaPlayIcon(), theme.MediaPauseIcon(), nil)
		})
	})
	t.Run("TwoStateToolbarAction zero value", func(t *testing.T) {
		assertNoExtendWarning(t, func() {
			(&TwoStateToolbarAction{}).SetOn(true)
		})
	})
	t.Run("ToolbarAction Disable before render", func(t *testing.T) {
		assertNoExtendWarning(t, func() {
			NewToolbarAction(theme.HomeIcon(), nil).Disable()
		})
	})
	t.Run("ToolbarAction SetIcon before render", func(t *testing.T) {
		assertNoExtendWarning(t, func() {
			NewToolbarAction(theme.HomeIcon(), nil).SetIcon(theme.MenuIcon())
		})
	})
	t.Run("shown in a toolbar", func(t *testing.T) {
		assertNoExtendWarning(t, func() {
			act := NewToolbarAction(theme.HomeIcon(), nil)
			two := NewTwoStateToolbarAction(theme.MediaPlayIcon(), theme.MediaPauseIcon(), nil)
			w := test.NewWindow(NewToolbar(act, two))
			defer w.Close()
			two.SetOn(true)
			act.Disable()
		})
	})
}
