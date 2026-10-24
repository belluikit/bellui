package container_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
)

// minRect returns a rectangle with a fixed minimum size, so an HBox lays it out
// deterministically without depending on font metrics.
func minRect(w, h float32) *canvas.Rectangle {
	rect := canvas.NewRectangle(nil)
	rect.SetMinSize(geo.NewSize(w, h))
	return rect
}

// layoutScope resizes a DirectionScope wrapping content to the content's natural
// size, forcing the subtree to lay out under the scope's direction.
func layoutScope(scope *container.DirectionScope) {
	scope.Resize(scope.MinSize())
}

func TestDirectionScope_MirrorsSubtreeUnderLTRGlobal(t *testing.T) {
	test.NewTempApp(t)
	bellui.SetLayoutDirection(bellui.DirectionLTR)
	t.Cleanup(func() { bellui.SetLayoutDirection(bellui.DirectionAuto) })

	a := minRect(50, 50)
	b := minRect(50, 50)
	c := minRect(50, 50)
	box := container.NewHBox(a, b, c)

	scope := container.NewDirectionScope(box, bellui.DirectionRTL)
	layoutScope(scope)

	w := box.Size().Width
	// The RTL scope mirrors the row even though the global setting is LTR:
	// the first child sits at the right edge, the last at the left.
	assert.Equal(t, w-50, a.Position().X, "first child mirrored to the right edge")
	assert.Equal(t, float32(0), c.Position().X, "last child mirrored to the left edge")
	assert.Equal(t, geo.NewSize(50, 50), a.Size(), "sizes unchanged by mirroring")
}

func TestDirectionScope_LTRDialogInsideRTLApp(t *testing.T) {
	test.NewTempApp(t)
	bellui.SetLayoutDirection(bellui.DirectionRTL)
	t.Cleanup(func() { bellui.SetLayoutDirection(bellui.DirectionAuto) })

	a := minRect(50, 50)
	b := minRect(50, 50)
	c := minRect(50, 50)
	box := container.NewHBox(a, b, c)

	// The headline use case: an LTR subtree inside an otherwise RTL app keeps
	// the natural left-to-right order.
	scope := container.NewDirectionScope(box, bellui.DirectionLTR)
	layoutScope(scope)

	assert.Equal(t, float32(0), a.Position().X, "first child stays at the left edge under the LTR scope")
	assert.Equal(t, box.Size().Width-50, c.Position().X, "last child stays at the right edge")
}

func TestDirectionScope_AutoInheritsGlobal(t *testing.T) {
	test.NewTempApp(t)
	bellui.SetLayoutDirection(bellui.DirectionRTL)
	t.Cleanup(func() { bellui.SetLayoutDirection(bellui.DirectionAuto) })

	a := minRect(50, 50)
	b := minRect(50, 50)
	box := container.NewHBox(a, b)

	// DirectionAuto is a no-op scope, so it follows the RTL global.
	scope := container.NewDirectionScope(box, bellui.DirectionAuto)
	layoutScope(scope)

	assert.Equal(t, box.Size().Width-50, a.Position().X, "auto scope follows the RTL global (mirrored)")
}

func TestDirectionScope_NestedInnermostWins(t *testing.T) {
	test.NewTempApp(t)
	bellui.SetLayoutDirection(bellui.DirectionRTL)
	t.Cleanup(func() { bellui.SetLayoutDirection(bellui.DirectionAuto) })

	a := minRect(50, 50)
	b := minRect(50, 50)
	inner := container.NewHBox(a, b)

	// Outer LTR scope, inner RTL scope: the inner row mirrors per the innermost
	// direction, not the outer one.
	innerScope := container.NewDirectionScope(inner, bellui.DirectionRTL)
	outerScope := container.NewDirectionScope(innerScope, bellui.DirectionLTR)
	outerScope.Resize(outerScope.MinSize())

	assert.Equal(t, inner.Size().Width-50, a.Position().X, "inner RTL scope mirrors regardless of outer LTR")
}

func TestDirectionScope_SetDirectionHonoredOnRelayout(t *testing.T) {
	test.NewTempApp(t)
	bellui.SetLayoutDirection(bellui.DirectionLTR)
	t.Cleanup(func() { bellui.SetLayoutDirection(bellui.DirectionAuto) })

	a := minRect(50, 50)
	b := minRect(50, 50)
	box := container.NewHBox(a, b)

	scope := container.NewDirectionScope(box, bellui.DirectionLTR)
	layoutScope(scope)
	assert.Equal(t, float32(0), a.Position().X, "starts LTR: first child at the left")

	// SetDirection updates the field; the new direction is applied on the next
	// real relayout (here forced by a size change). In-place live toggling
	// without a relayout is the runtime direction-change refresh tracked by pre-rebrand tracker #65,
	// the same limitation the global SetLayoutDirection has.
	scope.SetDirection(bellui.DirectionRTL)
	scope.Resize(scope.Size().Add(geo.NewSize(40, 0)))
	assert.Equal(t, box.Size().Width-50, a.Position().X, "after SetDirection(RTL) + relayout the row mirrors")
}

// A theme change re-lays-out every window's content (internal/app.ApplyThemeTo)
// outside any layout walk; the scoped subtree must keep its direction (#342).
func TestDirectionScope_SurvivesThemeVariantChange(t *testing.T) {
	a := test.NewTempApp(t)
	bellui.SetLayoutDirection(bellui.DirectionRTL)
	t.Cleanup(func() { bellui.SetLayoutDirection(bellui.DirectionAuto) })

	first := minRect(50, 50)
	second := minRect(50, 50)
	box := container.NewHBox(first, second)
	scope := container.NewDirectionScope(box, bellui.DirectionLTR)

	w := test.NewTempWindow(t, container.NewHBox(scope))
	w.Resize(geo.NewSize(300, 100))
	assert.Less(t, first.Position().X, second.Position().X, "LTR scope lays out left to right")

	a.Settings().SetThemeVariant(theme.VariantDark)
	assert.Less(t, first.Position().X, second.Position().X, "LTR scope still left to right after a theme change")
}

func TestDirectionOf(t *testing.T) {
	test.NewTempApp(t)
	lang.SetLocaleForTests(t, "en-US")

	loose := widget.NewLabel("not on a canvas")
	assert.Equal(t, bellui.DirectionLTR, container.DirectionOf(loose), "off-canvas: the global direction")

	plain := widget.NewLabel("plain")
	inRTL := widget.NewLabel("rtl")
	inLTR := widget.NewLabel("ltr in rtl")
	inAuto := widget.NewLabel("auto in rtl")
	ltr := container.NewDirectionScope(inLTR, bellui.DirectionLTR)
	rtl := container.NewDirectionScope(container.NewVBox(inRTL, ltr,
		container.NewDirectionScope(inAuto, bellui.DirectionAuto)), bellui.DirectionRTL)
	w := test.NewTempWindow(t, container.NewVBox(plain, rtl))

	assert.Equal(t, bellui.DirectionLTR, container.DirectionOf(plain))
	assert.Equal(t, bellui.DirectionRTL, container.DirectionOf(inRTL))
	assert.Equal(t, bellui.DirectionLTR, container.DirectionOf(inLTR), "innermost scope wins")
	assert.Equal(t, bellui.DirectionRTL, container.DirectionOf(inAuto), "an automatic scope inherits")
	assert.Equal(t, bellui.DirectionRTL, container.DirectionOf(ltr), "a scope sits in its parent's direction")
	assert.Equal(t, bellui.DirectionLTR, container.DirectionOf(rtl))

	w.Canvas().(interface{ SetDirection(bellui.Direction) }).SetDirection(bellui.DirectionRTL)
	assert.Equal(t, bellui.DirectionRTL, container.DirectionOf(plain), "the canvas direction applies outside scopes")
	assert.Equal(t, bellui.DirectionLTR, container.DirectionOf(inLTR), "a scope still wins over the canvas")
}

// A wheel scroll is an input event, dispatched outside any layout walk; inside
// an RTL scope it must flip as it does in an RTL app (#696).
func TestDirectionScope_ScrollWheelFlipsInRTLScope(t *testing.T) {
	test.NewTempApp(t)
	lang.SetLocaleForTests(t, "en-US")
	scroll := container.NewHScroll(minRect(1000, 50))
	w := test.NewTempWindow(t, container.NewDirectionScope(scroll, bellui.DirectionRTL))
	w.SetPadded(false)
	w.Resize(geo.NewSize(100, 100))
	require.Equal(t, float32(900), scroll.Offset.X, "pinned to the RTL start (right edge)")

	scroll.Scrolled(&event.ScrollEvent{Scrolled: geo.NewDelta(-10, 0)})
	assert.Equal(t, float32(890), scroll.Offset.X, "the wheel moves toward the content's far (left) end")
}

// Tab order follows a window's own direction (Canvas.SetDirection): focus
// traversal runs outside a layout walk, so it used to read the app's. The
// entries are placed by hand, so only the focus reordering sees the direction.
func TestCanvasDirection_FocusOrder(t *testing.T) {
	test.NewTempApp(t)
	lang.SetLocaleForTests(t, "en-US")
	left, right := widget.NewEntry(), widget.NewEntry()
	for i, e := range []*widget.Entry{left, right} {
		e.Resize(geo.NewSize(100, 40))
		e.Move(geo.NewPos(float32(i)*150, 0))
	}
	w := test.NewTempWindow(t, container.NewWithoutLayout(left, right))
	w.Canvas().(interface{ SetDirection(bellui.Direction) }).SetDirection(bellui.DirectionRTL)
	w.Resize(geo.NewSize(300, 100))

	w.Canvas().FocusNext()
	assert.Equal(t, right, w.Canvas().Focused(), "under RTL the first in reading order is the rightmost")
}

// Canvas.SetDirection on a window already laid out re-mirrors its content even
// though nothing is resized.
func TestCanvasDirection_SetAfterLayoutMirrors(t *testing.T) {
	test.NewTempApp(t)
	lang.SetLocaleForTests(t, "en-US")
	first, second := minRect(50, 50), minRect(50, 50)
	w := test.NewTempWindow(t, container.NewHBox(first, second))
	w.Resize(geo.NewSize(300, 100))
	require.Less(t, first.Position().X, second.Position().X)

	w.Canvas().(interface{ SetDirection(bellui.Direction) }).SetDirection(bellui.DirectionRTL)
	assert.Greater(t, first.Position().X, second.Position().X, "mirrored after SetDirection")
}

// panicOnRefresh is a rectangle whose Refresh panics.
type panicOnRefresh struct{ *canvas.Rectangle }

func (panicOnRefresh) Refresh() { panic("refresh") }

// A Refresh that panics under NewDirectionScope's push must not leave the
// scope's direction ambient once the panic is recovered.
func TestNewDirectionScope_PanickingRefreshPopsDirection(t *testing.T) {
	test.NewTempApp(t)
	lang.SetLocaleForTests(t, "en-US")
	assert.Panics(t, func() {
		container.NewDirectionScope(panicOnRefresh{canvas.NewRectangle(nil)}, bellui.DirectionRTL)
	})
	assert.False(t, lang.IsRTLDirection(), "the scope's RTL does not outlive the panic")
}
