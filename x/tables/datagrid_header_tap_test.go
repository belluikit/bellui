package tables

import (
	"fmt"
	"testing"

	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
)

// TestHeaderTapSortsOnTouchAndMouse: a header sorts on the tap. A touch
// screen sends only a tap, so a header that sorted on the mouse press never
// sorted on a phone (seen on a Pixel 9a, 2026-11-16). A mouse click is a
// press then a tap, and sorts once, with the press's Shift; a tap on a
// column border sorts nothing.
func TestHeaderTapSortsOnTouchAndMouse(t *testing.T) {
	g := headerHitGrid(t)
	h := g.headerWidget
	y := g.HeaderHeight() / 2
	at := func(x float32) *event.PointEvent { return &event.PointEvent{Position: geo.NewPos(x, y)} }
	sort := func() string { return fmt.Sprint(g.API().GetSort()) }
	checks := 0
	want := func(what, w string) {
		t.Helper()
		checks++
		if got := sort(); got != w {
			t.Errorf("%s: sort %s, want %s", what, got, w)
		}
	}

	// a finger: a tap alone
	h.Tapped(at(10))
	want("a touch tap on a", fmt.Sprint([]SortItem{{ColID: "a", Direction: SortAscending}}))
	h.Tapped(at(10))
	want("a second touch tap on a", fmt.Sprint([]SortItem{{ColID: "a", Direction: SortDescending}}))
	g.API().ClearSort()

	// a mouse: a press, then a tap, sorting once
	h.MouseDown(&desktop.MouseEvent{PointEvent: *at(10)})
	want("the press alone", "[]")
	h.Tapped(at(10))
	want("a click on a", fmt.Sprint([]SortItem{{ColID: "a", Direction: SortAscending}}))
	// Shift adds a column
	h.MouseDown(&desktop.MouseEvent{PointEvent: *at(100), Modifier: event.KeyModifierShift})
	h.Tapped(at(100))
	want("a shift-click on b", fmt.Sprint([]SortItem{{ColID: "a", Direction: SortAscending}, {ColID: "b", Direction: SortAscending}}))
	g.API().ClearSort()

	// a tap on a column's border is for resizing
	bx := float32(-1)
	for x := float32(1); x < 200; x++ {
		if g.HeaderBorderCol(x) >= 0 {
			bx = x
			break
		}
	}
	if bx < 0 {
		t.Fatal("no column border in the header")
	}
	h.Tapped(at(bx))
	want("a tap on a border", "[]")
	if checks != 6 {
		t.Fatalf("ran %d checks, want 6", checks)
	}
}
