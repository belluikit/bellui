package component

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"

	"github.com/belluikit/bellui/x/charts/style"
)

// BreadcrumbLayout holds positions for hit testing breadcrumb clicks.
type BreadcrumbLayout struct {
	BackButton buttonRect
	Crumbs     []breadcrumbCrumb
}

type breadcrumbCrumb struct {
	Rect  buttonRect
	Depth int
}

// HitTest returns the action ("back" or "") and the crumb depth index (-1 if none).
func (bl *BreadcrumbLayout) HitTest(pos geo.Position) (string, int) {
	if bl == nil {
		return "", -1
	}
	if hitButton(pos, bl.BackButton) {
		return "back", -1
	}
	for _, crumb := range bl.Crumbs {
		if hitButton(pos, crumb.Rect) {
			return "", crumb.Depth
		}
	}
	return "", -1
}

// RenderBreadcrumb produces the breadcrumb trail UI.
// path is the ordered list of breadcrumb labels (e.g., ["Year", "Q1"]).
// Returns objects, layout for hit testing, and total height consumed.
func RenderBreadcrumb(path []string, resolver *style.Resolver, x, y, maxWidth float32) ([]core.CanvasObject, *BreadcrumbLayout, float32) {
	if len(path) == 0 {
		return nil, nil, 0
	}

	var objects []core.CanvasObject
	layout := &BreadcrumbLayout{}
	height := float32(22)

	bgColor := resolver.BreadcrumbBackgroundColor()
	textColor := resolver.BreadcrumbTextColor()
	accentColor := resolver.BreadcrumbAccentColor()
	sepColor := resolver.BreadcrumbSeparatorColor()

	currentX := x

	// Back button [<]
	backW := float32(24)
	backBg := canvas.NewRectangle(bgColor)
	backBg.CornerRadius = 3
	backBg.StrokeColor = accentColor
	backBg.StrokeWidth = 0.5
	backBg.Move(geo.NewPos(currentX, y))
	backBg.Resize(geo.NewSize(backW, height))
	backLabel := canvas.NewText("<", textColor)
	backLabel.TextSize = 14
	backLabel.TextStyle = bellui.TextStyle{Bold: true}
	backLabel.Alignment = bellui.TextAlignCenter
	backLabel.Move(geo.NewPos(currentX, y+2))
	backLabel.Resize(geo.NewSize(backW, height-2))
	objects = append(objects, backBg, backLabel)
	layout.BackButton = buttonRect{Pos: geo.NewPos(currentX, y), Size: geo.NewSize(backW, height)}
	currentX += backW + 6

	// "Root" label (clicking navigates to depth 0)
	rootText := "Root"
	rootW := float32(len(rootText))*7 + 4
	rootLabel := canvas.NewText(rootText, accentColor)
	rootLabel.TextSize = 11
	rootLabel.Move(geo.NewPos(currentX, y+4))
	rootLabel.Resize(geo.NewSize(rootW, height-4))
	objects = append(objects, rootLabel)
	layout.Crumbs = append(layout.Crumbs, breadcrumbCrumb{
		Rect:  buttonRect{Pos: geo.NewPos(currentX, y), Size: geo.NewSize(rootW, height)},
		Depth: 0,
	})
	currentX += rootW

	// Path segments
	for i, segment := range path {
		// Separator " > "
		sep := canvas.NewText(" > ", sepColor)
		sep.TextSize = 11
		sep.Move(geo.NewPos(currentX, y+4))
		sep.Resize(geo.NewSize(20, height-4))
		objects = append(objects, sep)
		currentX += 20

		// Segment label
		segW := float32(len(segment))*7 + 4
		segColor := accentColor
		if i == len(path)-1 {
			segColor = textColor // current level is not clickable
		}
		segLabel := canvas.NewText(segment, segColor)
		segLabel.TextSize = 11
		segLabel.Move(geo.NewPos(currentX, y+4))
		segLabel.Resize(geo.NewSize(segW, height-4))
		objects = append(objects, segLabel)

		if i < len(path)-1 { // only non-current levels are clickable
			layout.Crumbs = append(layout.Crumbs, breadcrumbCrumb{
				Rect:  buttonRect{Pos: geo.NewPos(currentX, y), Size: geo.NewSize(segW, height)},
				Depth: i + 1,
			})
		}
		currentX += segW
	}

	totalHeight := height + 4
	return objects, layout, totalHeight
}
