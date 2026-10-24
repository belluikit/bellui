package app

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/internal/uidir"
)

// ApplyThemeTo ensures that the specified canvasobject and all widgets and themeable objects will
// be updated for the current theme.
//
// The re-layout runs outside the canvas' layout walk, so it pushes the
// directions that walk would: the canvas' own (Canvas.SetDirection) around the
// whole content, and each container.DirectionScope's around its subtree.
// Without them a theme change lays scoped content out in the global direction
// (#342).
func ApplyThemeTo(content core.CanvasObject, canv core.Canvas) {
	if dc, ok := canv.(interface{ Direction() bellui.Direction }); ok {
		d := dc.Direction()
		defer uidir.Restore(uidir.Mark())
		uidir.Push(d)
		defer uidir.Pop(d)
	}
	applyThemeTo(content, canv)
}

// directionScoped is implemented by container.DirectionScope, detected
// structurally because container imports this package.
type directionScoped interface {
	ScopedDirection() bellui.Direction
}

func applyThemeTo(content core.CanvasObject, canv core.Canvas) {
	if content == nil {
		return
	}
	if ds, ok := content.(directionScoped); ok {
		d := ds.ScopedDirection()
		uidir.Push(d)
		defer uidir.Pop(d)
	}

	switch o := content.(type) {
	case core.Widget:
		renderer := cache.Renderer(o)
		for _, co := range renderer.Objects() {
			applyThemeTo(co, canv)
		}
		renderer.Layout(content.Size()) // theme can cause sizing changes
	case *bellui.Container:
		for _, co := range o.Objects {
			applyThemeTo(co, canv)
		}
		if l := o.Layout; l != nil {
			l.Layout(o.Objects, o.Size()) // theme can cause sizing changes
		}
	}
	content.Refresh()
}

// ApplySettings ensures that all widgets and themeable objects in an application will be updated for the current theme.
// It also checks that scale changes are reflected if required
func ApplySettings(set bellui.Settings, app bellui.App) {
	ApplySettingsWithCallback(set, app, nil)
}

// ApplySettingsWithCallback ensures that all widgets and themeable objects in an application will be updated for the current theme.
// It also checks that scale changes are reflected if required. Also it will call `onEveryWindow` on every window
// interaction
func ApplySettingsWithCallback(set bellui.Settings, app bellui.App, onEveryWindow func(w bellui.Window)) {
	for _, window := range app.Driver().AllWindows() {
		ApplyThemeTo(window.Content(), window.Canvas())
		for _, overlay := range window.Canvas().Overlays().List() {
			ApplyThemeTo(overlay, window.Canvas())
		}
		if onEveryWindow != nil {
			onEveryWindow(window)
		}
	}
}
