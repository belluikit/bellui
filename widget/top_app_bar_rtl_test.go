package widget_test

import (
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"

	"github.com/stretchr/testify/assert"
)

// barObjects classifies a TopAppBar renderer's objects into the title text and
// the icon buttons (navigation + actions), in object order.
func barObjects(r core.WidgetRenderer) (title *canvas.Text, buttons []*widget.IconButton) {
	for _, o := range r.Objects() {
		switch obj := o.(type) {
		case *canvas.Text:
			title = obj
		case *widget.IconButton:
			buttons = append(buttons, obj)
		}
	}
	return title, buttons
}

func TestTopAppBar_Layout_LTR(t *testing.T) {
	test.NewTempApp(t)
	bellui.SetLayoutDirection(bellui.DirectionLTR)
	t.Cleanup(func() { bellui.SetLayoutDirection(bellui.DirectionAuto) })

	bar := widget.NewTopAppBar("Title", theme.NavigateBackIcon(), nil)
	bar.Actions = []widget.TopAppBarAction{{Icon: theme.SearchIcon(), OnTapped: func() {}}}
	r := test.TempWidgetRenderer(t, bar)
	bar.Resize(geo.NewSize(400, 64))

	title, buttons := barObjects(r)
	assert.NotNil(t, title)
	assert.Len(t, buttons, 2) // nav + 1 action

	nav := buttons[0]
	action := buttons[1]

	// LTR: nav at the leading (left) edge, action at the trailing (right) edge,
	// title to the right of the nav icon with no overlap.
	assert.Less(t, nav.Position().X, float32(200), "nav should be on the left")
	assert.Greater(t, action.Position().X, float32(200), "action should be on the right")
	assert.GreaterOrEqual(t, title.Position().X, nav.Position().X+nav.Size().Width,
		"title must not overlap the nav icon")
}

func TestTopAppBar_Layout_RTL(t *testing.T) {
	test.NewTempApp(t)
	bellui.SetLayoutDirection(bellui.DirectionRTL)
	t.Cleanup(func() { bellui.SetLayoutDirection(bellui.DirectionAuto) })

	bar := widget.NewTopAppBar("Title", theme.NavigateBackIcon(), nil)
	bar.Actions = []widget.TopAppBarAction{{Icon: theme.SearchIcon(), OnTapped: func() {}}}
	r := test.TempWidgetRenderer(t, bar)
	bar.Resize(geo.NewSize(400, 64))

	title, buttons := barObjects(r)
	assert.NotNil(t, title)
	assert.Len(t, buttons, 2)

	nav := buttons[0]
	action := buttons[1]

	// RTL mirror: nav moves to the trailing (right) edge, action to the leading
	// (left) edge, and the title sits to the LEFT of the nav icon without
	// overlapping it (the pre-rebrand tracker #122 bug was the title overlapping the nav icon).
	assert.Greater(t, nav.Position().X, float32(200), "nav should mirror to the right")
	assert.Less(t, action.Position().X, float32(200), "action should mirror to the left")
	assert.LessOrEqual(t, title.Position().X+title.Size().Width, nav.Position().X,
		"title must not overlap the nav icon under RTL")
	assert.Greater(t, title.Position().X, float32(200), "title should be on the right with the nav")
}
