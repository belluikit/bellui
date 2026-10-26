package container_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/driver"
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

// inRTLScopeWindow shows content inside a right-to-left DirectionScope in a
// left-to-right app (#749).
func inRTLScopeWindow(t *testing.T, content core.CanvasObject) bellui.Window {
	t.Helper()
	test.NewTempApp(t)
	lang.SetLocaleForTests(t, "en-US")
	require.False(t, lang.IsRTLDirection(), "the app itself is left to right")
	w := test.NewTempWindow(t, container.NewDirectionScope(content, bellui.DirectionRTL))
	w.SetPadded(false)
	w.Resize(geo.NewSize(300, 400))
	return w
}

// The issue's probe: an HBox laid out mirrored stays mirrored after its own
// Refresh, which re-lays it out outside any layout walk (#749).
func TestDirectionScope_HBoxStaysMirroredAfterRefresh(t *testing.T) {
	a, b := minRect(50, 50), minRect(50, 50)
	box := container.NewHBox(a, b)
	inRTLScopeWindow(t, container.NewVBox(box))
	require.Greater(t, a.Position().X, b.Position().X, "laid out mirrored")
	mirrored := a.Position().X

	box.Refresh()
	assert.Equal(t, mirrored, a.Position().X, "still mirrored after Refresh")
}

// Add and Remove re-lay the container out at once, outside a layout walk;
// inside a scope they mirror (#749).
func TestDirectionScope_ContainerAddRemoveInScope(t *testing.T) {
	a, b := minRect(50, 50), minRect(50, 50)
	box := container.NewHBox(a)
	inRTLScopeWindow(t, container.NewVBox(box))

	box.Add(b)
	assert.Greater(t, a.Position().X, b.Position().X, "the added item lands left of the first under RTL")

	c := minRect(50, 50)
	box.Add(c)
	box.Remove(b)
	assert.Greater(t, a.Position().X, c.Position().X, "still mirrored after Remove")
}

// An object moved out of a scope gets the app's direction from the lookup
// at once: Remove invalidates the direction cache (#749).
func TestDirectionOf_ObjectMovedOutOfScope(t *testing.T) {
	moved := minRect(10, 10)
	scoped := container.NewVBox(moved)
	outside := container.NewVBox()
	test.NewTempApp(t)
	lang.SetLocaleForTests(t, "en-US")
	w := test.NewTempWindow(t, container.NewVBox(container.NewDirectionScope(scoped, bellui.DirectionRTL), outside))
	w.Resize(geo.NewSize(200, 200))
	require.Equal(t, bellui.DirectionRTL, container.DirectionOf(moved))

	scoped.Remove(moved)
	outside.Add(moved)
	assert.Equal(t, bellui.DirectionLTR, container.DirectionOf(moved))
}

// Tab order inside an RTL scope in an LTR window: the scope's focusables are
// ordered right to left, as one block in the window's order (#749).
func TestDirectionScope_TabOrder(t *testing.T) {
	top := widget.NewEntry()
	left, right := widget.NewEntry(), widget.NewEntry()
	for i, e := range []*widget.Entry{left, right} {
		e.Resize(geo.NewSize(100, 40))
		e.Move(geo.NewPos(float32(i)*150, 0))
	}
	row := container.NewDirectionScope(container.NewWithoutLayout(left, right), bellui.DirectionRTL)
	after := widget.NewEntry()
	w := inLTRWindow(t, container.NewVBox(top, row, after))

	c := w.Canvas()
	var order []*widget.Entry
	for range 4 {
		c.FocusNext()
		order = append(order, c.Focused().(*widget.Entry))
	}
	assert.Equal(t, []*widget.Entry{top, right, left, after}, order,
		"the scope's block is right to left, in its place in the window's order")
}

// inLTRWindow shows content in an LTR app and window, unscoped.
func inLTRWindow(t *testing.T, content core.CanvasObject) bellui.Window {
	t.Helper()
	test.NewTempApp(t)
	lang.SetLocaleForTests(t, "en-US")
	w := test.NewTempWindow(t, content)
	w.Resize(geo.NewSize(400, 400))
	return w
}

// The issue's second probe: two hand-placed entries inside an RTL scope in an
// LTR app; Tab goes to the right one first (#749).
func TestDirectionScope_TabGoesRightFirst(t *testing.T) {
	left, right := widget.NewEntry(), widget.NewEntry()
	for i, e := range []*widget.Entry{left, right} {
		e.Resize(geo.NewSize(100, 40))
		e.Move(geo.NewPos(float32(i)*150, 0))
	}
	w := inRTLScopeWindow(t, container.NewWithoutLayout(left, right))

	w.Canvas().FocusNext()
	assert.Equal(t, right, w.Canvas().Focused(), "the first in the scope's reading order is the rightmost")
}

// An object moved into a scope gets the scope's direction from the lookup at
// once: Add invalidates the direction cache, as Remove does (#749).
func TestDirectionOf_ObjectMovedIntoScope(t *testing.T) {
	moved := minRect(10, 10)
	scoped := container.NewVBox()
	outside := container.NewVBox(moved)
	test.NewTempApp(t)
	lang.SetLocaleForTests(t, "en-US")
	w := test.NewTempWindow(t, container.NewVBox(container.NewDirectionScope(scoped, bellui.DirectionRTL), outside))
	w.Resize(geo.NewSize(200, 200))
	require.Equal(t, bellui.DirectionLTR, container.DirectionOf(moved))

	outside.Remove(moved)
	scoped.Add(moved)
	assert.Equal(t, bellui.DirectionRTL, container.DirectionOf(moved))
}

// A container added to one in a scope keeps the mirroring Add gave it through
// its own Refresh before the next frame (#749).
func TestDirectionScope_AddedHBoxStaysMirroredAfterRefresh(t *testing.T) {
	outer := container.NewVBox()
	inRTLScopeWindow(t, outer)
	require.Equal(t, bellui.DirectionRTL, container.DirectionOf(outer), "the cache is built before the Add")

	a, b := minRect(50, 50), minRect(50, 50)
	box := container.NewHBox(a, b)
	outer.Add(box)
	require.Greater(t, a.Position().X, b.Position().X, "Add laid it out mirrored")

	box.Refresh()
	assert.Greater(t, a.Position().X, b.Position().X, "still mirrored after its own Refresh")
}

// Setting the Direction field and calling Refresh, as its doc says, reaches
// the direction lookup at once, as SetDirection does (#749).
func TestDirectionScope_DirectionFieldThenRefresh(t *testing.T) {
	inner := minRect(10, 10)
	test.NewTempApp(t)
	lang.SetLocaleForTests(t, "en-US")
	scope := container.NewDirectionScope(container.NewVBox(inner), bellui.DirectionRTL)
	w := test.NewTempWindow(t, scope)
	w.Resize(geo.NewSize(200, 200))
	require.Equal(t, bellui.DirectionRTL, container.DirectionOf(inner))

	scope.Direction = bellui.DirectionLTR
	scope.Refresh()
	assert.Equal(t, bellui.DirectionLTR, container.DirectionOf(inner))
}

// Add and Remove take the object's subtree into or out of the direction cache
// where it stands, so a loop of them does not rebuild the cache each time; a
// nested scope moved with them keeps its own direction inside (#749).
func TestDirectionOf_AddRemoveUpdateTheCacheInPlace(t *testing.T) {
	scoped, outside := container.NewVBox(), container.NewVBox()
	test.NewTempApp(t)
	lang.SetLocaleForTests(t, "en-US")
	w := test.NewTempWindow(t, container.NewVBox(container.NewDirectionScope(scoped, bellui.DirectionRTL), outside))
	w.Resize(geo.NewSize(200, 200))
	inner, innerRTL := minRect(10, 10), minRect(10, 10)
	nested := container.NewDirectionScope(container.NewVBox(inner), bellui.DirectionLTR)
	nestedRTL := container.NewDirectionScope(container.NewVBox(innerRTL), bellui.DirectionRTL)
	outside.Add(nested)
	scoped.Add(nestedRTL)
	w.Resize(geo.NewSize(200, 300)) // a frame: the renderers exist and the cache is rebuilt
	require.Equal(t, bellui.DirectionLTR, container.DirectionOf(nested))
	require.Equal(t, bellui.DirectionRTL, container.DirectionOf(innerRTL))
	builds, lookups := driver.DirectionIndexStats()

	rows := make([]*canvas.Rectangle, 20)
	for i := range rows {
		rows[i] = minRect(10, 10)
		scoped.Add(rows[i])
	}
	assert.Equal(t, bellui.DirectionRTL, container.DirectionOf(rows[len(rows)-1]), "an added row is in the scope")
	outside.Remove(nested)
	scoped.Add(nested)
	assert.Equal(t, bellui.DirectionRTL, container.DirectionOf(nested), "the nested scope sits in the RTL scope now")
	assert.Equal(t, bellui.DirectionLTR, container.DirectionOf(inner), "its content keeps its own direction")
	for _, r := range rows {
		scoped.Remove(r)
	}
	assert.Equal(t, bellui.DirectionLTR, container.DirectionOf(rows[0]), "a removed row is out of the scope")
	scoped.Remove(nestedRTL)
	assert.Equal(t, bellui.DirectionRTL, container.DirectionOf(innerRTL), "a scope removed keeps its content's direction")
	scoped.RemoveAll()
	assert.Equal(t, bellui.DirectionLTR, container.DirectionOf(nested), "RemoveAll takes the nested scope out")
	assert.Equal(t, bellui.DirectionLTR, container.DirectionOf(inner))

	after, afterLookups := driver.DirectionIndexStats()
	assert.Greater(t, afterLookups, lookups, "the lookups reached the cache")
	assert.Equal(t, builds, after, "no Add or Remove rebuilt it")
}

// The cache of objects on windows with a direction of their own follows an
// object a Container takes in or gives up between frames: added to a box on
// a right-to-left window it is right to left, and moved from there to a box on
// a window in the global direction it is not (#772).
func TestDirectionOf_DirectedWindowCacheFollowsAddRemove(t *testing.T) {
	test.NewTempApp(t)
	lang.SetLocaleForTests(t, "en-US")
	rtlBox, ltrBox := container.NewVBox(minRect(10, 10)), container.NewVBox(minRect(10, 10))
	ltr := test.NewTempWindow(t, ltrBox)
	ltr.Resize(geo.NewSize(200, 200))
	rtl := test.NewTempWindow(t, rtlBox)
	rtl.Canvas().(interface{ SetDirection(bellui.Direction) }).SetDirection(bellui.DirectionRTL)
	rtl.Resize(geo.NewSize(200, 200))
	require.Equal(t, bellui.DirectionRTL, container.DirectionOf(rtlBox.Objects[0]), "the lookup builds the cache")

	moved := minRect(10, 10)
	rtlBox.Add(moved)
	assert.Equal(t, bellui.DirectionRTL, container.DirectionOf(moved))

	rtlBox.Remove(moved)
	ltrBox.Add(moved)
	assert.Equal(t, bellui.DirectionLTR, container.DirectionOf(moved))
}

// swapper is a widget whose renderer replaces its objects with next on each
// Refresh, without a Container: the move no dirhook.Added reports (#773).
type swapper struct {
	widget.BaseWidget
	next []core.CanvasObject
}

func newSwapper() *swapper {
	s := &swapper{}
	s.ExtendBaseWidget(s)
	return s
}

func (s *swapper) CreateRenderer() core.WidgetRenderer { return &swapperRenderer{s: s} }

type swapperRenderer struct {
	s    *swapper
	objs []core.CanvasObject
}

func (r *swapperRenderer) Destroy()                     {}
func (r *swapperRenderer) Layout(geo.Size)              {}
func (r *swapperRenderer) MinSize() geo.Size            { return geo.NewSize(10, 10) }
func (r *swapperRenderer) Objects() []core.CanvasObject { return r.objs }
func (r *swapperRenderer) Refresh()                     { r.objs = r.s.next }

// An object a widget's renderer takes into a scope on its own, or one placed
// by assigning a Container's Objects, is in the scope's direction as soon as
// its new parent refreshes, before any frame, and without a rebuild of the
// cache (#773).
func TestDirectionOf_RefreshTakesInNewChildren(t *testing.T) {
	test.NewTempApp(t)
	lang.SetLocaleForTests(t, "en-US")
	sw, box := newSwapper(), container.NewVBox()
	w := test.NewTempWindow(t, container.NewDirectionScope(container.NewVBox(sw, box), bellui.DirectionRTL))
	w.Resize(geo.NewSize(200, 200))
	require.Equal(t, bellui.DirectionRTL, container.DirectionOf(sw), "the lookup builds the cache")
	builds, lookups := driver.DirectionIndexStats()

	label := widget.NewLabel("taken in by a renderer")
	sw.next = []core.CanvasObject{label}
	sw.Refresh()
	assert.Equal(t, bellui.DirectionRTL, container.DirectionOf(label), "a renderer's new child")

	rect := minRect(10, 10)
	box.Objects = []core.CanvasObject{rect}
	box.Refresh()
	assert.Equal(t, bellui.DirectionRTL, container.DirectionOf(rect), "a child placed by assigning Objects")

	after, afterLookups := driver.DirectionIndexStats()
	assert.Greater(t, afterLookups, lookups, "the lookups reached the cache")
	assert.Equal(t, builds, after, "no refresh rebuilt it")
}

// What a refresh does not undo, as DirectionOf documents: an object a
// renderer dropped keeps its old scope's direction until the next frame
// rebuilds the cache (#773). This pins that gap; it fails if it closes.
func TestDirectionOf_RendererDropIsSeenAtTheNextFrame(t *testing.T) {
	test.NewTempApp(t)
	lang.SetLocaleForTests(t, "en-US")
	label := widget.NewLabel("dropped")
	sw := newSwapper()
	sw.next = []core.CanvasObject{label}
	w := test.NewTempWindow(t, container.NewDirectionScope(container.NewVBox(sw), bellui.DirectionRTL))
	w.Resize(geo.NewSize(200, 200))
	require.Equal(t, bellui.DirectionRTL, container.DirectionOf(label))

	sw.next = nil
	sw.Refresh()
	assert.Equal(t, bellui.DirectionRTL, container.DirectionOf(label), "before a frame, the old scope's direction")
	w.Resize(geo.NewSize(200, 300)) // a frame
	assert.Equal(t, bellui.DirectionLTR, container.DirectionOf(label), "after it, the global direction")
}
