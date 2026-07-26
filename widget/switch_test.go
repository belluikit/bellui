package widget_test

import (
	"testing"

	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"

	"github.com/stretchr/testify/assert"
)

func TestSwitch_New(t *testing.T) {
	test.NewTempApp(t)
	s := widget.NewSwitch(nil)
	assert.NotNil(t, s)
	assert.False(t, s.On)
}

func TestSwitch_MinSize(t *testing.T) {
	test.NewTempApp(t)
	s := widget.NewSwitch(nil)
	min := s.MinSize()
	assert.Greater(t, min.Width, float32(0))
	assert.Greater(t, min.Height, float32(0))
}

func TestSwitch_Tapped(t *testing.T) {
	test.NewTempApp(t)
	changed := false
	s := widget.NewSwitch(func(on bool) { changed = on })

	test.Tap(s)
	assert.True(t, s.On)
	assert.True(t, changed)

	test.Tap(s)
	assert.False(t, s.On)
	assert.False(t, changed)
}

func TestSwitch_Disabled(t *testing.T) {
	test.NewTempApp(t)
	s := widget.NewSwitch(nil)
	s.Disable()

	s.Tapped(&event.PointEvent{})
	assert.False(t, s.On, "Disabled switch should not toggle")
}
