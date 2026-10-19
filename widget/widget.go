// Package widget defines the UI widgets within the Bellui toolkit.
package widget // import "github.com/belluikit/bellui/widget"

import (
	"fmt"
	"log"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	internalWidget "github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/theme"
)

// onMain runs fn on the main goroutine. If already on main, it runs directly.
// Otherwise it dispatches via bellui.DoAndWait to ensure the function completes
// before returning. This makes widget mutating methods safe to call from any goroutine.
func onMain(fn func()) {
	bellui.RunOnMain(fn)
}

// StateSyncer is an optional interface that widgets implement to sync their
// public fields to internal rendering state. If a widget implements this,
// BaseWidget.Refresh() calls SyncState() automatically before the renderer's
// Refresh(), ensuring state is always consistent.
//
// This eliminates the class of bugs where a widget's Refresh() forgets to
// sync internal state (segments, sub-widgets, caches) before triggering
// a renderer redraw.
//
// Example:
//
//	func (e *Entry) SyncState() {
//	    e.syncSegments()
//	    e.text.Refresh()
//	}
type StateSyncer interface {
	SyncState()
}

// DebugRefresh enables logging of every widget Refresh() call.
// Set to true during development to trace the refresh cascade.
//
//	widget.DebugRefresh = true
var DebugRefresh bool

// DebugLayout enables logging of MinSize calculations and Resize calls.
// Set to true during development to understand why widgets get specific sizes.
//
//	widget.DebugLayout = true
var DebugLayout bool

// BaseWidget provides a helper that handles basic widget behaviours.
type BaseWidget struct {
	noCopy noCopy // so `go vet` can complain if a widget is passed by value (copied)

	size     geo.Size
	position geo.Position
	Hidden   bool

	impl core.Widget
	// themeCache is read on the layout path (Theme → MinSize) while Refresh
	// clears it, potentially from a different goroutine (multiple engine
	// instances, or a binding-driven refresh overlapping a layout pass), so it
	// is accessed atomically to stay data-race-free (pre-rebrand tracker #604). A plain interface
	// field would tear; atomic.Value can't be used because distinct themes are
	// distinct concrete types.
	themeCache atomic.Pointer[bellui.Theme]
	refreshing bool // re-entrance guard

	accessibilityLabel string
	toolTipText        string
}

// SetAccessibilityLabel sets an application-supplied accessible label that
// takes precedence over the widget's intrinsic AccessibleLabel. Use it to
// name widgets without visible text, such as icon-only buttons:
//
//	btn := widget.NewIconButton(theme.DeleteIcon(), onDelete)
//	btn.SetAccessibilityLabel(lang.L("Delete"))
//
// It only affects widgets that implement [core.Accessible].
func (w *BaseWidget) SetAccessibilityLabel(label string) {
	w.accessibilityLabel = label
}

// AccessibilityLabel returns the label set via [BaseWidget.SetAccessibilityLabel],
// or an empty string when none is set. It implements [core.AccessibleLabelOverrider].
func (w *BaseWidget) AccessibilityLabel() string {
	return w.accessibilityLabel
}

// SetToolTipText sets the hover tooltip text for this widget. The desktop driver
// shows the text in a small bubble after a short hover delay, and hides it when
// the pointer leaves; an empty string (the default) means no tooltip. This is
// the ergonomic, driver-level replacement for wrapping a widget in
// widget.NewTooltip:
//
//	btn := widget.NewIconButton(theme.ContentAddIcon(), onNew)
//	btn.SetToolTipText(lang.L("New file"))
//
// Since: 1.0
func (w *BaseWidget) SetToolTipText(text string) {
	w.toolTipText = text
}

// ToolTip returns the tooltip text set via [BaseWidget.SetToolTipText], or an
// empty string when none is set. It implements [core.ToolTipable], so every
// widget can carry a hover tooltip.
//
// Since: 1.0
func (w *BaseWidget) ToolTip() string {
	return w.toolTipText
}

// ExtendBaseWidget is used by an extending widget to make use of BaseWidget functionality.
func (w *BaseWidget) ExtendBaseWidget(wid core.Widget) {
	if w.impl != nil {
		return
	}

	w.impl = wid
}

// Size gets the current size of this widget.
func (w *BaseWidget) Size() geo.Size {
	return w.size
}

// Resize sets a new size for a widget.
// Note this should not be used if the widget is being managed by a Layout within a Container.
func (w *BaseWidget) Resize(size geo.Size) {
	if size == w.Size() {
		return
	}

	w.size = size

	impl := w.super()
	if impl == nil {
		return
	}

	if DebugLayout {
		log.Printf("[layout] Resize %T → %v (min %v)", impl, size, w.MinSize())
	}

	cache.Renderer(impl).Layout(size)
}

// Position gets the current position of this widget, relative to its parent.
func (w *BaseWidget) Position() geo.Position {
	return w.position
}

// Move the widget to a new position, relative to its parent.
// Note this should not be used if the widget is being managed by a Layout within a Container.
func (w *BaseWidget) Move(pos geo.Position) {
	if w.Position() == pos {
		return
	}

	w.position = pos
	internalWidget.Repaint(w.super())
}

// MinSize for the widget - it should never be resized below this value.
// Enable widget.DebugLayout to trace MinSize calculations.
func (w *BaseWidget) MinSize() geo.Size {
	impl := w.super()

	r := cache.Renderer(impl)
	if r == nil {
		return geo.Size{}
	}

	min := r.MinSize()

	if DebugLayout {
		log.Printf("[layout] MinSize %T = %v", impl, min)
	}

	return min
}

// Visible returns whether or not this widget should be visible.
// Note that this may not mean it is currently visible if a parent has been hidden.
func (w *BaseWidget) Visible() bool {
	return !w.Hidden
}

// Show this widget so it becomes visible
func (w *BaseWidget) Show() {
	if w.Visible() {
		return
	}

	w.Hidden = false

	impl := w.super()
	if impl == nil {
		return
	}
	impl.Refresh()
}

// Hide this widget so it is no longer visible
func (w *BaseWidget) Hide() {
	if !w.Visible() {
		return
	}

	w.Hidden = true

	impl := w.super()
	if impl == nil {
		return
	}
	canvas.Refresh(impl)
}

// Refresh causes this widget to be redrawn in its current state.
//
// The refresh sequence is:
//  1. Re-entrance check — prevents infinite Refresh↔Resize loops
//  2. SyncState — if the widget implements [StateSyncer], its SyncState()
//     method is called to sync public fields to internal rendering state
//  3. Renderer.Refresh — the renderer updates canvas objects from state
//
// Widgets should implement [StateSyncer] instead of overriding Refresh()
// to ensure state is always synced before rendering.
func (w *BaseWidget) Refresh() {
	// 1. Re-entrance guard.
	if w.refreshing {
		return
	}
	w.refreshing = true
	defer func() { w.refreshing = false }()

	impl := w.super()
	if impl == nil {
		return
	}

	if DebugRefresh {
		log.Printf("[refresh] %T", impl)
	}

	w.themeCache.Store(nil)

	// 2. Sync state before renderer refresh.
	if syncer, ok := impl.(StateSyncer); ok {
		syncer.SyncState()
	}

	// 3. Renderer refresh.
	cache.Renderer(impl).Refresh()

	// 4. The renderer may refresh only its parts, so tell the canvas that the
	// widget's own min size may have changed too (#464).
	if m, ok := cache.GetCanvasForObject(impl).(interface{ MarkMinSizeDirty(core.CanvasObject) }); ok {
		m.MarkMinSizeDirty(impl)
	}
}

// Theme returns a cached Theme instance for this widget (or its extending widget).
// This will be the app theme in most cases, or a widget specific theme if it is inside a ThemeOverride container.
//
// Since: 1.0
func (w *BaseWidget) Theme() bellui.Theme {
	if cached := w.themeCache.Load(); cached != nil {
		return *cached
	}

	th := cache.WidgetTheme(w.super())
	// don't cache the default as it may change
	if th == nil {
		return theme.Current()
	}

	// Only this branch takes an address, so only it allocates: storing &th
	// made th escape on every call, the default theme's included (#492).
	cached := th
	w.themeCache.Store(&cached)
	return cached
}

var warnedWidgets sync.Map

// ResetExtendWarningsForTests clears the process-global de-dup set that limits the
// "used before ExtendBaseWidget was called" warning (see super) to once per call
// site for the whole process. That de-dup is the right default in production (a
// misconfigured widget would otherwise log every frame), but it makes a regression
// test that asserts a widget does NOT warn unreliable in a shared package run: an
// earlier test that tripped the same call site suppresses the log the later test is
// watching for. Call this at the start of such a test so it observes the warning
// fresh; the registered cleanup clears the set again so a following test starts
// clean too. Mirrors lang.SetLocaleForTests (takes the minimal Cleanup interface to
// avoid importing testing into production code).
func ResetExtendWarningsForTests(t interface{ Cleanup(func()) }) {
	warnedWidgets.Clear()
	t.Cleanup(func() { warnedWidgets.Clear() })
}

// super will return the actual object that this represents.
// If extended then this is the extending widget, otherwise it is nil.
// Logs a warning on first use if ExtendBaseWidget was not called.
func (w *BaseWidget) super() core.Widget {
	if w.impl != nil {
		return w.impl
	}

	// Log a clear warning with the call site so the offending widget can be found.
	// reflect.TypeOf(w) is always *BaseWidget here, so the stack trace is more
	// useful than the type name for identifying the actual widget.
	_, file, line, ok := runtime.Caller(2) // caller of the method that called super()
	loc := ""
	if ok {
		loc = fmt.Sprintf(" (called from %s:%d)", file, line)
	}
	if _, warned := warnedWidgets.LoadOrStore(loc, true); !warned {
		log.Printf("bellui: widget BaseWidget used before ExtendBaseWidget was called. "+
			"Add w.ExtendBaseWidget(w) to your constructor. "+
			"This widget will not render correctly.%s", loc)
	}
	return nil
}

// DisableableWidget describes an extension to BaseWidget which can be disabled.
// Disabled widgets should have a visually distinct style when disabled, normally using theme.DisabledButtonColor.
type DisableableWidget struct {
	BaseWidget

	disabled bool
}

// Enable this widget, updating any style or features appropriately.
func (w *DisableableWidget) Enable() {
	onMain(func() {
		if !w.Disabled() {
			return
		}
		w.disabled = false
		if impl := w.super(); impl != nil {
			impl.Refresh()
		}
	})
}

// Disable this widget so that it cannot be interacted with, updating any style appropriately.
func (w *DisableableWidget) Disable() {
	onMain(func() {
		if w.Disabled() {
			return
		}
		w.disabled = true
		if impl := w.super(); impl != nil {
			impl.Refresh()
		}
	})
}

// Disabled returns true if this widget is currently disabled or false if it can currently be interacted with.
func (w *DisableableWidget) Disabled() bool {
	return w.disabled
}

// NewSimpleRenderer creates a new SimpleRenderer to render a widget using a
// single core.CanvasObject.
//
// Since: 1.0
func NewSimpleRenderer(object core.CanvasObject) core.WidgetRenderer {
	return internalWidget.NewSimpleRenderer(object)
}

// Orientation controls the horizontal/vertical layout of a widget
type Orientation int

// Orientation constants to control widget layout
const (
	Horizontal Orientation = 0
	Vertical   Orientation = 1

	// Adaptive will switch between horizontal and vertical layouts according to device orientation.
	// This orientation is not always supported and interpretation can vary per-widget.
	//
	// Since: 1.0
	Adaptive Orientation = 2
)

type noCopy struct{}

func (*noCopy) Lock() {}

func (*noCopy) Unlock() {}
