package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
)

func TestSetSingleInstance_RequiresID(t *testing.T) {
	a := &belluiApp{} // no uniqueID, and default metadata has an empty ID
	primary, err := a.SetSingleInstance(nil)
	assert.Error(t, err)
	assert.False(t, primary)
	assert.False(t, a.siEnabled)
}

func TestSetSingleInstance_PrimaryAndIdempotent(t *testing.T) {
	a := NewWithID("io.bellui.test.single-instance").(*belluiApp)
	t.Cleanup(func() {
		if a.siCloser != nil {
			a.siCloser()
		}
	})

	si, ok := bellui.App(a).(bellui.SingleInstanceApp)
	require.True(t, ok, "app should implement bellui.SingleInstanceApp")

	primary, err := si.SetSingleInstance(nil)
	require.NoError(t, err)
	assert.True(t, primary, "sole running instance is the primary")
	assert.True(t, a.siEnabled)

	// A second call is a no-op that reports the same role without rebinding.
	primaryAgain, err := si.SetSingleInstance(func([]string) {})
	require.NoError(t, err)
	assert.Equal(t, primary, primaryAgain)
}
