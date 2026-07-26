package widget

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/theme"
)

// BottomSheet is an MD3 bottom sheet that shows content from the bottom of the screen.
type BottomSheet struct {
	Content core.CanvasObject
	Modal   bool

	canvas core.Canvas
	popUp  *PopUp
}

// NewBottomSheet creates a bottom sheet with the given content.
func NewBottomSheet(content core.CanvasObject) *BottomSheet {
	return &BottomSheet{Content: content}
}

// Show displays the bottom sheet on the given canvas.
func (s *BottomSheet) Show(c core.Canvas) {
	s.canvas = c
	s.Hide()

	th := theme.Current()
	v := bellui.CurrentApp().Settings().ThemeVariant()

	// Drag handle.
	handle := canvas.NewRectangle(th.Color(theme.ColorNameOnSurfaceVariant, v))
	handle.CornerRadius = 2
	handle.Resize(geo.NewSize(32, 4))

	// Sheet background with content.
	bg := canvas.NewRectangle(th.Color(theme.ColorNameSurfaceContainerLow, v))
	bg.CornerRadius = 28 // MD3: extra-large top corners

	sheetContent := &bellui.Container{
		Objects: []core.CanvasObject{bg, handle, s.Content},
		Layout:  &bottomSheetLayout{handle: handle, bg: bg},
	}

	if s.Modal {
		s.popUp = NewModalPopUp(sheetContent, c)
	} else {
		s.popUp = NewPopUp(sheetContent, c)
	}

	// Position at bottom.
	canvasSize := c.Size()
	contentSize := sheetContent.MinSize()
	width := canvasSize.Width
	if width > 640 {
		width = 640 // max width per MD3
	}
	x := (canvasSize.Width - width) / 2
	y := canvasSize.Height - contentSize.Height

	s.popUp.Resize(geo.NewSize(width, contentSize.Height))

	// Animate slide up from bottom.
	if bellui.CurrentApp().Settings().ShowAnimations() {
		startY := canvasSize.Height // off-screen bottom
		endY := y
		s.popUp.ShowAtPosition(geo.NewPos(x, startY))
		anim := bellui.NewAnimation(canvas.DurationMedium2, func(done float32) {
			cy := startY + (endY-startY)*done
			s.popUp.Move(geo.NewPos(x, cy))
		})
		anim.Curve = bellui.AnimationEaseStandardDecelerate
		anim.Start()
	} else {
		s.popUp.ShowAtPosition(geo.NewPos(x, y))
	}
}

// Hide dismisses the bottom sheet.
func (s *BottomSheet) Hide() {
	if s.popUp != nil {
		s.popUp.Hide()
		s.popUp = nil
	}
}

// ShowBottomSheet is a convenience function.
func ShowBottomSheet(content core.CanvasObject, c core.Canvas) {
	s := NewBottomSheet(content)
	s.Show(c)
}

// --- layout for sheet content ---

type bottomSheetLayout struct {
	handle *canvas.Rectangle
	bg     *canvas.Rectangle
}

func (l *bottomSheetLayout) MinSize(objects []core.CanvasObject) geo.Size {
	if len(objects) < 3 {
		return geo.NewSize(0, 0)
	}
	contentMin := objects[2].MinSize()
	return geo.NewSize(contentMin.Width+32, contentMin.Height+28) // padding + handle
}

func (l *bottomSheetLayout) Layout(objects []core.CanvasObject, size geo.Size) {
	l.bg.Resize(size)
	l.bg.Move(geo.NewPos(0, 0))

	l.handle.Move(geo.NewPos((size.Width-32)/2, 12))

	if len(objects) >= 3 {
		objects[2].Move(geo.NewPos(16, 24))
		objects[2].Resize(geo.NewSize(size.Width-32, size.Height-28))
	}
}
