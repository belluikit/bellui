//go:build !tamago && !noos

package test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/theme"
)

// TestApplyTheme_RestoresPrevious confirms the happy path: ApplyTheme installs
// the theme and the registered cleanup restores the previous one.
func TestApplyTheme_RestoresPrevious(t *testing.T) {
	// These tests install a fresh app and mutate its theme; restore the prior
	// current app so later same-package tests that read the global theme are
	// unaffected.
	defer bellui.SetCurrentApp(bellui.CurrentApp())
	a := NewApp().(*app)

	previous := a.Settings().Theme()
	th := NewTheme()
	require.NotEqual(t, previous, th)

	// Run ApplyTheme inside a sub-test so its t.Cleanup fires when the sub-test
	// ends, letting us observe the restore.
	t.Run("apply", func(t *testing.T) {
		ApplyTheme(t, th)
		assert.Equal(t, th, a.lastAppliedTheme(), "theme should be applied")
	})
	assert.Equal(t, previous, a.lastAppliedTheme(), "previous theme should be restored after cleanup")
}

// TestThemeApplied_Bounded verifies the wait is bounded: it returns true
// promptly for an already-applied theme and returns false (rather than
// hanging) once the timeout elapses for a theme that never applies (pre-rebrand tracker #37).
func TestThemeApplied_Bounded(t *testing.T) {
	defer bellui.SetCurrentApp(bellui.CurrentApp())
	a := NewApp().(*app)

	applied := NewTheme()
	a.Settings().SetTheme(applied)
	require.True(t, themeApplied(a, applied, time.Second),
		"the already-applied theme should be observed immediately")

	never := theme.DefaultTheme() // a distinct theme that is never set, so the wait must time out
	require.NotEqual(t, applied, never)

	start := time.Now()
	ok := themeApplied(a, never, 30*time.Millisecond)
	elapsed := time.Since(start)

	assert.False(t, ok, "an unapplied theme must time out rather than match")
	assert.Less(t, elapsed, time.Second, "the wait must be bounded by the timeout, not hang")
}
