package widget

import (
	"sync"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
)

// Wire the driver-level tooltip seam (pre-rebrand tracker #800). A driver calls core.NotifyToolTip
// as the hovered ToolTipable target changes; this controller owns the show
// delay, the bubble overlay and its teardown. Installed once when the widget
// package initializes, which any app using widgets imports.
func init() {
	core.ShowToolTip = sharedToolTipController.show
	core.HideToolTip = sharedToolTipController.hide
}

var sharedToolTipController = &toolTipController{}

// toolTipController renders driver-level hover tooltips for [core.ToolTipable]
// objects. One tooltip is visible at a time across the app. All methods run on
// the main goroutine (driver hover dispatch, or the delay timer's bellui.Do), and
// the mutex guards against the timer callback racing a target change.
type toolTipController struct {
	mu     sync.Mutex
	active core.CanvasObject // the target the pending/visible tooltip belongs to
	tip    *tooltipOverlay
	canvas core.Canvas
	timer  *time.Timer
}

// show starts the hover-delay for obj's tooltip. It is a no-op when obj is
// already the active target (so it is cheap to call on every pointer move) or
// when obj carries no tooltip text.
func (c *toolTipController) show(obj core.CanvasObject, canvas core.Canvas) {
	tt, ok := obj.(core.ToolTipable)
	if !ok {
		return
	}
	text := tt.ToolTip()

	c.mu.Lock()
	defer c.mu.Unlock()
	if obj == c.active {
		return
	}
	c.stopLocked()
	c.active = obj
	if text == "" || canvas == nil {
		return
	}
	c.canvas = canvas
	c.timer = time.AfterFunc(tooltipDelay, func() {
		bellui.Do(func() { c.present(obj, canvas, text) })
	})
}

// hide cancels any pending tooltip and removes a visible one.
func (c *toolTipController) hide() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.active = nil
	c.stopLocked()
}

// stopLocked cancels a pending timer and removes any visible bubble. The caller
// must hold c.mu.
func (c *toolTipController) stopLocked() {
	if c.timer != nil {
		c.timer.Stop()
		c.timer = nil
	}
	if c.tip != nil && c.canvas != nil {
		c.canvas.Overlays().Remove(c.tip)
	}
	c.tip = nil
	c.canvas = nil
}

// present builds and shows the bubble once the delay elapses, unless the target
// changed while it did (the timer may have fired before hide() stopped it).
func (c *toolTipController) present(obj core.CanvasObject, canvas core.Canvas, text string) {
	c.mu.Lock()
	if c.active != obj {
		c.mu.Unlock()
		return
	}
	c.timer = nil
	th, v := toolTipTheme(obj)
	c.tip = newTooltipOverlay(newToolTipBubble(th, v, text), toolTipPos(obj))
	c.canvas = canvas
	tip := c.tip
	c.mu.Unlock()

	inheritDirection(tip, obj) // the bubble's text follows a scope around obj (#749)
	canvas.Overlays().Add(tip)
}

// toolTipTheme resolves the theme + variant to render a tooltip for obj: the
// widget's own theme when obj is a themed widget, else the app theme.
func toolTipTheme(obj core.CanvasObject) (bellui.Theme, bellui.ThemeVariant) {
	v := bellui.CurrentApp().Settings().ThemeVariant()
	if w, ok := obj.(interface{ Theme() bellui.Theme }); ok {
		return w.Theme(), v
	}
	return bellui.CurrentApp().Settings().Theme(), v
}

// toolTipPos anchors the bubble just below the object.
func toolTipPos(obj core.CanvasObject) geo.Position {
	abs := bellui.CurrentApp().Driver().AbsolutePositionForObject(obj)
	return abs.Add(geo.NewPos(0, obj.Size().Height))
}
