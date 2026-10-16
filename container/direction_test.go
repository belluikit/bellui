package container_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
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
