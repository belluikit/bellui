package theme_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
)

// An object made after a ThemeOverride was applied carries no override until
// MatchScope copies its parent's (#758).
func TestMatchScope_CopiesTheParentsOverride(t *testing.T) {
	test.NewTempApp(t)
	parent := widget.NewLabel("parent")
	o := container.NewThemeOverride(container.NewStack(parent), test.NewTheme())
	w := test.NewWindow(o)
	defer w.Close()
	want := theme.CurrentForWidget(parent)
	assert.NotEqual(t, theme.Current(), want, "the parent should be overridden")

	check := widget.NewCheck("later", nil)
	text := canvas.NewText("later", nil)
	assert.Equal(t, theme.Current(), theme.CurrentForWidget(check), "made after the override, so not yet in it")

	theme.MatchScope(parent, check, text, nil)
	assert.Equal(t, want, theme.CurrentForWidget(check))
	assert.Equal(t, want, theme.CurrentForWidget(text))
	assert.Equal(t, want, check.Theme(), "the widget's cached theme follows")

	theme.MatchScope(parent, check) // already matching: a no-op
	assert.Equal(t, want, theme.CurrentForWidget(check))
}

func TestMatchScope_ParentOutsideAnOverrideDoesNothing(t *testing.T) {
	test.NewTempApp(t)
	parent := widget.NewLabel("parent")
	check := widget.NewCheck("later", nil)

	theme.MatchScope(parent, check)
	assert.Equal(t, theme.Current(), theme.CurrentForWidget(check))
}
