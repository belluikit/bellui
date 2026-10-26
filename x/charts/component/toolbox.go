package component

import (
	"strconv"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"

	"github.com/belluikit/bellui/x/charts/interact"
	"github.com/belluikit/bellui/x/charts/style"
)

// ToolboxConfig configures the chart toolbox.
type ToolboxConfig struct {
	SaveAsImage bool     // show save-as-image button
	MagicType   []string // chart type switching (e.g., ["line", "bar"])

	// DrawingTools shows these drawing tools as buttons inside the chart, above
	// the plot, with the active one highlighted and a Clear All button after
	// them. interact.DefaultDrawingTools() gives all of them. It is the
	// in-chart shortcut for chart.DrawingToolbar, which an app places itself.
	//
	// Experimental: added in #796 and may change until a second app uses it.
	DrawingTools []interact.DrawingToolMode
}

// ToolboxLayout holds positions for hit testing toolbox clicks.
type ToolboxLayout struct {
	Buttons []toolboxButton
}

type toolboxButton struct {
	Rect   buttonRect
	Action string // "save", "magicType:line", "magicType:bar", "drawingTool:3", "clearDrawings"
}

// Toolbox actions for the drawing tools: ActionDrawingTool plus the tool's
// number, and ActionClearDrawings.
const (
	ActionDrawingTool   = "drawingTool:"
	ActionClearDrawings = "clearDrawings"
)

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

// ToolboxButtonHeight is the height of the toolbox buttons' row.
const ToolboxButtonHeight float32 = 20

// HasButtons reports whether the toolbox draws any button.
func (cfg *ToolboxConfig) HasButtons() bool {
	return cfg != nil && (cfg.SaveAsImage || len(cfg.MagicType) > 0)
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
	btnH := ToolboxButtonHeight
	gap := float32(3)
	bgColor := resolver.ButtonBackgroundColor()
	textColor := resolver.ButtonTextColor()
	currentX := x

	addButton := func(label, action string) {
		w := toolboxButtonWidth(label, btnW)
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

// toolboxButtonWidth is the width of a toolbox button labelled label:
// minWidth for one or two characters, else enough for the label.
func toolboxButtonWidth(label string, minWidth float32) float32 {
	if len(label) > 2 {
		return float32(len(label))*7 + 8
	}
	return minWidth
}

// drawingToolsRowStep is the distance between two rows of drawing tools.
const drawingToolsRowStep = ToolboxButtonHeight + 4

// drawingToolButtons lays the drawing tools of cfg out in rows from (x, y),
// wrapping at width, and returns each button's label, rectangle and action.
func drawingToolButtons(cfg *ToolboxConfig, x, y, width float32) (labels []string, buttons []toolboxButton) {
	if cfg == nil || len(cfg.DrawingTools) == 0 {
		return nil, nil
	}
	const minW, gap = 24, 3
	cx, cy := x, y
	add := func(label, action string) {
		w := toolboxButtonWidth(label, minW)
		if cx > x && cx+w > x+width {
			cx, cy = x, cy+drawingToolsRowStep
		}
		labels = append(labels, label)
		buttons = append(buttons, toolboxButton{
			Rect:   buttonRect{Pos: geo.NewPos(cx, cy), Size: geo.NewSize(w, ToolboxButtonHeight)},
			Action: action,
		})
		cx += w + gap
	}
	for _, t := range cfg.DrawingTools {
		add(t.String(), ActionDrawingTool+strconv.Itoa(int(t)))
	}
	add("Clear All", ActionClearDrawings)
	return labels, buttons
}

// DrawingToolsHeight is the height the drawing tools of cfg take when laid out
// in width, or 0 when cfg shows none.
func DrawingToolsHeight(cfg *ToolboxConfig, width float32) float32 {
	_, buttons := drawingToolButtons(cfg, 0, 0, width)
	if len(buttons) == 0 {
		return 0
	}
	return buttons[len(buttons)-1].Rect.Pos.Y + ToolboxButtonHeight
}

// RenderDrawingTools renders the drawing tools of cfg in rows from (x, y),
// wrapping at width, with active highlighted, and adds their buttons to layout
// for click detection.
func RenderDrawingTools(cfg *ToolboxConfig, active interact.DrawingToolMode, x, y, width float32, resolver *style.Resolver, layout *ToolboxLayout) []core.CanvasObject {
	labels, buttons := drawingToolButtons(cfg, x, y, width)
	activeAction := ActionDrawingTool + strconv.Itoa(int(active))
	var objects []core.CanvasObject
	for i, b := range buttons {
		bgColor := resolver.ButtonBackgroundColor()
		if b.Action == activeAction {
			bgColor = resolver.AccentColor()
		}
		bg := canvas.NewRectangle(bgColor)
		bg.CornerRadius = 3
		bg.Move(b.Rect.Pos)
		bg.Resize(b.Rect.Size)
		txt := canvas.NewText(labels[i], resolver.ButtonTextColor())
		txt.TextSize = resolver.ButtonTextSize()
		txt.Alignment = bellui.TextAlignCenter
		txt.Move(geo.NewPos(b.Rect.Pos.X, b.Rect.Pos.Y+3))
		txt.Resize(geo.NewSize(b.Rect.Size.Width, b.Rect.Size.Height-3))
		objects = append(objects, bg, txt)
	}
	if layout != nil {
		layout.Buttons = append(layout.Buttons, buttons...)
	}
	return objects
}
