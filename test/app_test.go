package test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/lang"
)

func TestTestApp_CloudProvider(t *testing.T) {
	a := NewApp()
	c := &mockCloud{}
	a.(bellui.CloudApp).SetCloudProvider(c)

	assert.Equal(t, c, a.(bellui.CloudApp).CloudProvider())
}

func TestTestApp_SetLayoutDirectionRefreshes(t *testing.T) {
	a := NewApp()
	t.Cleanup(func() { bellui.SetLayoutDirection(bellui.DirectionAuto) })

	bellui.SetLayoutDirection(bellui.DirectionLTR) // known starting point

	applied := 0
	a.Settings().AddListener(func(bellui.Settings) { applied++ })

	bellui.SetLayoutDirection(bellui.DirectionRTL)
	assert.Equal(t, 1, applied, "changing direction should re-apply settings once")
}

func TestTestApp_SetLocaleRefreshes(t *testing.T) {
	a := NewApp()
	t.Cleanup(func() { lang.SetLocale("") })

	lang.SetLocale("en") // known starting point

	applied := 0
	a.Settings().AddListener(func(bellui.Settings) { applied++ })

	lang.SetLocale("fr-FR")
	assert.Equal(t, 1, applied, "changing the locale should re-apply settings once")
}

func TestBelluiApp_transitionCloud(t *testing.T) {
	a := NewApp()
	p := &mockCloud{}
	preferenceChanged := false
	settingsChan := make(chan bellui.Settings, 1)
	a.Preferences().AddChangeListener(func() {
		preferenceChanged = true
	})
	a.Settings().AddListener(func(s bellui.Settings) {
		go func() { settingsChan <- s }()
	})
	a.(bellui.CloudApp).SetCloudProvider(p)

	<-settingsChan // settings were updated
	assert.True(t, preferenceChanged)
}
