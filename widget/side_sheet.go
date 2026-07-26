package widget

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/layout"
	"github.com/belluikit/bellui/theme"
)

// SideSheet is an MD3 side-anchored supplementary panel.
type SideSheet struct {
	Content core.CanvasObject
	Modal   bool

	canvas core.Canvas
	popUp  *PopUp
}

// NewSideSheet creates a side sheet with the given content.
func NewSideSheet(content core.CanvasObject) *SideSheet {
	return &SideSheet{Content: content}
}

// Show displays the side sheet on the right side of the canvas.
func (s *SideSheet) Show(c core.Canvas) {
	s.canvas = c
	s.Hide()

	th := theme.Current()
	v := bellui.CurrentApp().Settings().ThemeVariant()

	// Handle with title area.
	handle := canvas.NewRectangle(th.Color(theme.ColorNameOnSurfaceVariant, v))
	handle.CornerRadius = 2
	handle.Resize(geo.NewSize(4, 32))

	bgColor := th.Color(theme.ColorNameSurfaceContainerLow, v)
	padded := &bellui.Container{Layout: layout.NewPaddedLayout(), Objects: []core.CanvasObject{s.Content}}

	if s.Modal {
		s.popUp = NewModalPopUp(padded, c)
	} else {
		s.popUp = newPopUpWithBgColor(padded, c, bgColor)
	}

	canvasSize := c.Size()
	sheetWidth := float32(400)
	if sheetWidth > canvasSize.Width*0.8 {
		sheetWidth = canvasSize.Width * 0.8
	}

	endX := canvasSize.Width - sheetWidth
	s.popUp.Resize(geo.NewSize(sheetWidth, canvasSize.Height))

	if bellui.CurrentApp().Settings().ShowAnimations() {
		startX := canvasSize.Width // off-screen right
		s.popUp.ShowAtPosition(geo.NewPos(startX, 0))
		anim := bellui.NewAnimation(canvas.DurationMedium2, func(done float32) {
			cx := startX + (endX-startX)*done
			s.popUp.Move(geo.NewPos(cx, 0))
		})
		anim.Curve = bellui.AnimationEaseStandardDecelerate
		anim.Start()
	} else {
		s.popUp.ShowAtPosition(geo.NewPos(endX, 0))
	}
}

// Hide dismisses the side sheet.
func (s *SideSheet) Hide() {
	if s.popUp != nil {
		s.popUp.Hide()
		s.popUp = nil
	}
}

// ShowSideSheet is a convenience function.
func ShowSideSheet(content core.CanvasObject, c core.Canvas) {
	s := NewSideSheet(content)
	s.Show(c)
}
