package widget_test

import (
	"testing"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
)

// A drag across a SwipeArea reports its direction when it goes far enough
// and mostly one way; a short or diagonal drag reports nothing (#970).
func TestSwipeArea(t *testing.T) {
	test.NewTempApp(t)
	var got []widget.SwipeDirection
	area := widget.NewSwipeArea(canvas.NewRectangle(nil), func(d widget.SwipeDirection) { got = append(got, d) })
	w := test.NewTempWindow(t, area)
	w.Resize(geo.NewSize(300, 300))
	area.Resize(geo.NewSize(300, 300))
	mid := geo.NewPos(150, 150)
	for _, c := range []struct {
		name   string
		dx, dy float32
		want   widget.SwipeDirection // "" for none
	}{
		{"right", 120, 10, widget.SwipeRight},
		{"left", -120, -10, widget.SwipeLeft},
		{"up", 5, -80, widget.SwipeUp},
		{"down", 0, 60, widget.SwipeDown},
		{"too short", 10, 0, ""},
		{"diagonal", 80, 70, ""},
	} {
		got = nil
		test.Drag(w.Canvas(), mid, c.dx, c.dy)
		switch {
		case c.want == "" && len(got) != 0:
			t.Errorf("%s: reported %v", c.name, got)
		case c.want != "" && (len(got) != 1 || got[0] != c.want):
			t.Errorf("%s: reported %v, want %s", c.name, got, c.want)
		}
	}
	// a swipe made of many small moves adds them up
	got = nil
	test.DragSteps(w.Canvas(), mid, -90, 0, 9, nil)
	if len(got) != 1 || got[0] != widget.SwipeLeft {
		t.Errorf("in steps: reported %v, want left", got)
	}
}

// A tap on a SwipeArea reaches its content.
func TestSwipeAreaTapsReachTheContent(t *testing.T) {
	test.NewTempApp(t)
	taps := 0
	btn := widget.NewButton("x", func() { taps++ })
	area := widget.NewSwipeArea(btn, nil)
	test.Tap(area)
	if taps != 1 {
		t.Errorf("%d taps reached the content", taps)
	}
}
