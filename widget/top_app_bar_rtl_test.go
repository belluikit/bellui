package widget_test

import (
	"fmt"
	"strings"
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

// TestTopAppBar_TitleNeverUnderAnAction pins #1454: whatever the width and
// however many actions, the title ends before the first action (LTR) or
// starts after it (RTL), truncated with an ellipsis when it does not fit, and
// whole again once the bar is wide enough.
func TestTopAppBar_TitleNeverUnderAnAction(t *testing.T) {
	const title = "Communication"
	for _, dir := range []bellui.Direction{bellui.DirectionLTR, bellui.DirectionRTL} {
		for actions := 0; actions <= 5; actions++ {
			test.NewTempApp(t)
			bellui.SetLayoutDirection(dir)
			bar := widget.NewTopAppBar(title, theme.NavigateBackIcon(), nil)
			for range actions {
				bar.Actions = append(bar.Actions, widget.TopAppBarAction{Icon: theme.SearchIcon(), OnTapped: func() {}})
			}
			r := test.TempWidgetRenderer(t, bar)

			for _, width := range []float32{120, 200, 240, 280, 320, 360, 412, 600, 360} {
				bar.Resize(geo.NewSize(width, 64))
				text, buttons := barObjects(r)
				nav, acts := buttons[0], buttons[1:]
				name := func(what string) string {
					return fmt.Sprintf("%s (dir %v, %d actions, width %v, shown %q)", what, dir, actions, width, text.Text)
				}

				// What is drawn is the title, or a prefix of it and an ellipsis.
				if text.Text != title {
					shown := []rune(text.Text)
					if len(shown) > 0 {
						assert.Equal(t, '…', shown[len(shown)-1], name("a cut title ends in an ellipsis"))
						assert.True(t, strings.HasPrefix(title, string(shown[:len(shown)-1])), name("a cut title is a prefix"))
					}
				}
				assert.LessOrEqual(t, text.Size().Width, text.MinSize().Width+0.01, name("box matches the glyphs"))

				if text.Text == "" {
					// Not even the ellipsis fits, so nothing is drawn. The
					// actions may then reach past the nav button: that is
					// the overflow menu's to solve (#1456), not the title's.
					assert.Less(t, width, float32(320), name("only a bar too narrow for an ellipsis draws no title"))
					continue
				}
				left, right := text.Position().X, text.Position().X+text.Size().Width
				if dir == bellui.DirectionLTR {
					assert.GreaterOrEqual(t, left, nav.Position().X+nav.Size().Width, name("title after the nav button"))
					if len(acts) > 0 {
						assert.LessOrEqual(t, right, acts[0].Position().X, name("title before the first action"))
					} else {
						assert.LessOrEqual(t, right, width, name("title inside the bar"))
					}
				} else {
					assert.LessOrEqual(t, right, nav.Position().X, name("title before the nav button"))
					if len(acts) > 0 {
						assert.GreaterOrEqual(t, left, acts[0].Position().X+acts[0].Size().Width, name("title after the first action"))
					} else {
						assert.GreaterOrEqual(t, left, float32(0), name("title inside the bar"))
					}
				}
			}

			// At 600 the whole title fits whatever the action count, and the
			// last resize (back to 360) must have cut or kept it as before.
			bar.Resize(geo.NewSize(600, 64))
			text, _ := barObjects(r)
			assert.Equal(t, title, text.Text, "the whole title once there is room")
			bellui.SetLayoutDirection(bellui.DirectionAuto)
		}
	}
}

// TestTopAppBar_TitleTruncatesAtGalleryWidth pins the case #1454 was filed on:
// a 360 dp bar with a navigation button and five actions has 72 dp before the
// first action, too little for "Communication" at TitleLarge bold.
func TestTopAppBar_TitleTruncatesAtGalleryWidth(t *testing.T) {
	test.NewTempApp(t)
	bellui.SetLayoutDirection(bellui.DirectionLTR)
	t.Cleanup(func() { bellui.SetLayoutDirection(bellui.DirectionAuto) })

	bar := widget.NewTopAppBar("Communication", theme.NavigateBackIcon(), nil)
	for range 5 {
		bar.Actions = append(bar.Actions, widget.TopAppBarAction{Icon: theme.SearchIcon(), OnTapped: func() {}})
	}
	r := test.TempWidgetRenderer(t, bar)
	bar.Resize(geo.NewSize(360, 64))

	text, buttons := barObjects(r)
	assert.NotEqual(t, "Communication", text.Text)
	assert.True(t, strings.HasSuffix(text.Text, "…"), "got %q", text.Text)
	assert.Greater(t, len([]rune(text.Text)), 1, "72 dp holds more than the ellipsis")
	assert.LessOrEqual(t, text.Position().X+text.Size().Width, buttons[1].Position().X)
	assert.Equal(t, "Communication", bar.AccessibleLabel(), "the accessible name keeps the whole title")
}
