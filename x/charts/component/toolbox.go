package component

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"

	"github.com/belluikit/bellui/x/charts/style"
)

// ToolboxConfig configures the chart toolbox.
type ToolboxConfig struct {
	SaveAsImage bool     // show save-as-image button
	MagicType   []string // chart type switching (e.g., ["line", "bar"])
}

// ToolboxLayout holds positions for hit testing toolbox clicks.
type ToolboxLayout struct {
	Buttons []toolboxButton
}

type toolboxButton struct {
	Rect   buttonRect
	Action string // "save", "magicType:line", "magicType:bar"
}

// HitTest returns the action string for the clicked button, or "".
func (tl *ToolboxLayout) HitTest(pos geo.Position) string {
	if tl == nil {
		return ""
	}
	for _, btn := range tl.Buttons {
		if hitButton(pos, btn.Rect) {
			return btn.Action
		}
	}
	return ""
}

// RenderToolbox renders toolbox buttons and returns the layout for click detection.
// Positioned below zoom buttons (or at top-right if no zoom).
func RenderToolbox(cfg *ToolboxConfig, x, y float32, resolver *style.Resolver) ([]core.CanvasObject, *ToolboxLayout) {
	if cfg == nil {
		return nil, nil
	}

	var objects []core.CanvasObject
	layout := &ToolboxLayout{}

	btnW := float32(24)
	btnH := float32(20)
	gap := float32(3)
	bgColor := resolver.ButtonBackgroundColor()
	textColor := resolver.ButtonTextColor()
	currentX := x

	addButton := func(label, action string) {
		w := btnW
		if len(label) > 2 {
			w = float32(len(label))*7 + 8
		}
		bg := canvas.NewRectangle(bgColor)
		bg.CornerRadius = 3
		bg.Move(geo.NewPos(currentX, y))
		bg.Resize(geo.NewSize(w, btnH))
		txt := canvas.NewText(label, textColor)
		txt.TextSize = resolver.ButtonTextSize()
		txt.Alignment = bellui.TextAlignCenter
		txt.Move(geo.NewPos(currentX, y+3))
		txt.Resize(geo.NewSize(w, btnH-3))
		objects = append(objects, bg, txt)
		layout.Buttons = append(layout.Buttons, toolboxButton{
			Rect:   buttonRect{Pos: geo.NewPos(currentX, y), Size: geo.NewSize(w, btnH)},
			Action: action,
		})
		currentX += w + gap
	}

	if cfg.SaveAsImage {
		addButton("Save", "save")
	}

	for _, t := range cfg.MagicType {
		label := t
		if len(label) > 4 {
			label = label[:4]
		}
		addButton(label, "magicType:"+t)
	}

	return objects, layout
}
