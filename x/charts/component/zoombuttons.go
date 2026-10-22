package component

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"

	"github.com/belluikit/bellui/x/charts/style"
)

// ZoomButtonsLayout holds the positions of zoom control buttons for hit testing.
type ZoomButtonsLayout struct {
	ZoomInRect  buttonRect // bounding box for the zoom-in (+) button
	ZoomOutRect buttonRect // bounding box for the zoom-out (-) button
	ResetRect   buttonRect // bounding box for the reset button
}

type buttonRect struct {
	Pos  geo.Position
	Size geo.Size
}

// HitTest returns "in", "out", "reset", or "" depending on which button was clicked.
func (z *ZoomButtonsLayout) HitTest(pos geo.Position) string {
	if z == nil {
		return ""
	}
	if hitButton(pos, z.ZoomInRect) {
		return "in"
	}
	if hitButton(pos, z.ZoomOutRect) {
		return "out"
	}
	if hitButton(pos, z.ResetRect) {
		return "reset"
	}
	return ""
}

func hitButton(pos geo.Position, btn buttonRect) bool {
	return pos.X >= btn.Pos.X && pos.X <= btn.Pos.X+btn.Size.Width &&
		pos.Y >= btn.Pos.Y && pos.Y <= btn.Pos.Y+btn.Size.Height
}

// ZoomButtonHeight is the height of the zoom buttons' row.
const ZoomButtonHeight float32 = 22

// RenderZoomButtons draws +, −, and reset buttons horizontally.
// rightX is the right edge to align to, y is the top position.
func RenderZoomButtons(rightX, y float32, resolver *style.Resolver) ([]core.CanvasObject, *ZoomButtonsLayout) {
	btnW := float32(28)
	btnH := ZoomButtonHeight
	btnSize := geo.NewSize(btnW, btnH)
	gap := float32(3)
	bgColor := resolver.ZoomButtonBackgroundColor()
	hoverColor := resolver.AccentColor()
	textColor := resolver.ButtonTextColor()

	var objects []core.CanvasObject
	layout := &ZoomButtonsLayout{}

	// Layout right-to-left: [+] [−] [Reset]
	// Reset button (rightmost)
	resetX := rightX - btnW
	resetPos := geo.NewPos(resetX, y)
	resetBg := canvas.NewRectangle(bgColor)
	resetBg.CornerRadius = 3
	resetBg.StrokeColor = hoverColor
	resetBg.StrokeWidth = 0.5
	resetBg.Move(resetPos)
	resetBg.Resize(btnSize)
	resetLabel := canvas.NewText("Reset", textColor)
	resetLabel.TextSize = 10
	resetLabel.Alignment = bellui.TextAlignCenter
	// Wider hit area for reset
	resetBtnW := btnW + 12
	resetX2 := rightX - resetBtnW
	resetPos2 := geo.NewPos(resetX2, y)
	resetBg.Move(resetPos2)
	resetBg.Resize(geo.NewSize(resetBtnW, btnH))
	resetLabelY := y + (btnH-resetLabel.TextSize)/2
	resetLabel.Move(geo.NewPos(resetX2, resetLabelY))
	resetLabel.Resize(geo.NewSize(resetBtnW, resetLabel.TextSize+2))
	objects = append(objects, resetBg, resetLabel)
	layout.ResetRect = buttonRect{Pos: resetPos2, Size: geo.NewSize(resetBtnW, btnH)}

	// Zoom Out (−)
	outX := resetX2 - gap - btnW
	outPos := geo.NewPos(outX, y)
	outBg := canvas.NewRectangle(bgColor)
	outBg.CornerRadius = 3
	outBg.StrokeColor = hoverColor
	outBg.StrokeWidth = 0.5
	outBg.Move(outPos)
	outBg.Resize(btnSize)
	outLabel := canvas.NewText("−", textColor)
	outLabel.TextSize = 16
	outLabel.TextStyle = bellui.TextStyle{Bold: true}
	outLabel.Alignment = bellui.TextAlignCenter
	outLabelY := y + (btnH-outLabel.TextSize)/2
	outLabel.Move(geo.NewPos(outX, outLabelY))
	outLabel.Resize(btnSize)
	objects = append(objects, outBg, outLabel)
	layout.ZoomOutRect = buttonRect{Pos: outPos, Size: btnSize}

	// Zoom In (+)
	inX := outX - gap - btnW
	inPos := geo.NewPos(inX, y)
	inBg := canvas.NewRectangle(bgColor)
	inBg.CornerRadius = 3
	inBg.StrokeColor = hoverColor
	inBg.StrokeWidth = 0.5
	inBg.Move(inPos)
	inBg.Resize(btnSize)
	inLabel := canvas.NewText("+", textColor)
	inLabel.TextSize = 16
	inLabel.TextStyle = bellui.TextStyle{Bold: true}
	inLabel.Alignment = bellui.TextAlignCenter
	inLabelY := y + (btnH-inLabel.TextSize)/2
	inLabel.Move(geo.NewPos(inX, inLabelY))
	inLabel.Resize(btnSize)
	objects = append(objects, inBg, inLabel)
	layout.ZoomInRect = buttonRect{Pos: inPos, Size: btnSize}

	return objects, layout
}
