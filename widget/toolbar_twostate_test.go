package widget

import (
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
