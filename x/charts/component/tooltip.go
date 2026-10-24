package component

import (
	"fmt"
	"image/color"
	"strings"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"

	"github.com/belluikit/bellui/x/charts/interact"
	"github.com/belluikit/bellui/x/charts/style"
)

// TooltipTrigger controls what causes the tooltip to appear.
type TooltipTrigger int

const (
	TooltipTriggerItem TooltipTrigger = iota // show on hover over data point
	TooltipTriggerAxis                       // show at nearest X across all series
	TooltipTriggerNone                       // disabled
)

// CrosshairMode controls which crosshair lines to show.
type CrosshairMode int

const (
	CrosshairVertical   CrosshairMode = iota // vertical line only (default)
	CrosshairBoth                            // vertical + horizontal
	CrosshairHorizontal                      // horizontal line only
	CrosshairNone                            // no crosshair
)

// TooltipConfig configures tooltip behavior.
type TooltipConfig struct {
	Trigger      TooltipTrigger
	Crosshair    CrosshairMode
	Formatter    func(hits []interact.HitResult) string // custom content; nil = default
	PinDuration  time.Duration                          // auto-dismiss after pin; 0 = 15s default
	ShowAxisTags bool                                   // show value tags at crosshair/axis intersections
	// CrosshairOnly: when true, hover shows only crosshair + axis tags.
	// The tooltip text box only appears when pinned (clicked).
	CrosshairOnly bool
	// AxisTagFormatter formats axis values for the crosshair tags.
	// Receives (dataX, dataY) and returns (xLabel, yLabel). Nil = default.
	AxisTagFormatter func(dataX, dataY float64) (string, string)
}

// TooltipState tracks current tooltip display state.
type TooltipState struct {
	Visible  bool                 // whether the tooltip is shown
	Hits     []interact.HitResult // data points near the cursor
	MousePos geo.Position         // last known cursor position
	Pinned   bool                 // tooltip is pinned (persists after mouse moves away)
	PinTime  time.Time            // when the tooltip was pinned (for auto-dismiss)
}

// IsPinExpired returns true if the pinned tooltip has exceeded its duration.
func (s *TooltipState) IsPinExpired(duration time.Duration) bool {
	if !s.Pinned {
		return false
	}
	if duration <= 0 {
		duration = 15 * time.Second
	}
	return time.Since(s.PinTime) > duration
}

// PlotArea describes the plot region, which clips the crosshair lines, and
// the sides its axes are drawn on, where the axis value tags go.
//
// The zero value of each side field is the default layout — X axis below the
// plot, Y axis to its left — so a PlotArea that sets only X, Y, W and H places
// the tags as before those fields existed.
type PlotArea struct {
	X, Y, W, H float32

	// XAxisTop is true when the X axis is drawn along the top of the plot
	// (axis.PositionTop). The X value tag then sits above the plot instead of
	// below it.
	XAxisTop bool

	// YAxisRight is true when the Y axis that the first hit's series is
	// plotted against is drawn to the right of the plot. The Y value tag, which
	// shows that series' value, then sits beside that axis instead of the left
	// one. A chart with several Y axes sets it for each hover, from the hit's
	// series.
	YAxisRight bool

	// XAxisOffset and YAxisOffset are how far outward from the plot edge the
	// X axis, and the Y axis of the first hit's series, are drawn
	// (axis.Config.Offset). The tags move out with them, to stay beside the
	// axis. Zero keeps them at the plot edge.
	XAxisOffset, YAxisOffset float32
}

// RenderTooltip produces canvas objects for a tooltip near the given hits.
// seriesColors maps series index to its display color.
func RenderTooltip(state *TooltipState, cfg *TooltipConfig, resolver *style.Resolver, seriesColors map[int]color.Color, widgetSize geo.Size, plotArea PlotArea) []core.CanvasObject {
	if state == nil || !state.Visible || len(state.Hits) == 0 {
		return nil
	}

	// CrosshairOnly mode: render only crosshair + axis tags on hover.
	// The tooltip text box only appears when pinned (clicked).
	crosshairOnly := cfg != nil && cfg.CrosshairOnly && !state.Pinned
	if crosshairOnly {
		return renderCrosshairOnly(state, cfg, resolver, plotArea)
	}

	// Build tooltip text lines.
	// If a custom Formatter is set, call it once with all hits and split
	// the result on newlines. Each line is rendered as a separate text object
	// since canvas.Text doesn't support embedded \n or \t characters.
	var lines []tooltipLine
	if cfg != nil && cfg.Formatter != nil {
		text := cfg.Formatter(state.Hits)
		text = strings.ReplaceAll(text, "\t", "    ")
		for _, raw := range strings.Split(strings.TrimRight(text, "\n"), "\n") {
			c := color.Color(color.White)
			if len(state.Hits) > 0 {
				if sc := seriesColors[state.Hits[0].SeriesIndex]; sc != nil {
					c = sc
				}
			}
			lines = append(lines, tooltipLine{text: raw, color: c, hideMarker: true})
		}
	} else {
		for _, hit := range state.Hits {
			text := formatHitDefault(hit)
			c := seriesColors[hit.SeriesIndex]
			if c == nil {
				c = color.White
			}
			lines = append(lines, tooltipLine{text: text, color: c})
		}
	}

	if len(lines) == 0 {
		return nil
	}

	// Compute tooltip dimensions
	padding := resolver.TooltipPadding()
	lineHeight := resolver.TooltipLineHeight()
	markerSize := resolver.TooltipMarkerSize()
	markerGap := float32(6)

	maxTextWidth := float32(0)
	for _, l := range lines {
		w := float32(len(l.text)) * 7
		if w > maxTextWidth {
			maxTextWidth = w
		}
	}

	hasMarkers := false
	for _, l := range lines {
		if !l.hideMarker {
			hasMarkers = true
			break
		}
	}
	tooltipW := padding*2 + maxTextWidth
	if hasMarkers {
		tooltipW += markerSize + markerGap
	}
	tooltipH := padding*2 + float32(len(lines))*lineHeight

	// Position: near the first hit point, offset to avoid cursor overlap
	tx := state.Hits[0].PixelPos.X + 15
	ty := state.Hits[0].PixelPos.Y - tooltipH/2

	// Clamp to widget bounds
	if tx+tooltipW > widgetSize.Width-5 {
		tx = state.Hits[0].PixelPos.X - tooltipW - 15
	}
	if ty < 5 {
		ty = 5
	}
	if ty+tooltipH > widgetSize.Height-5 {
		ty = widgetSize.Height - tooltipH - 5
	}

	var objects []core.CanvasObject

	// Background
	bg := canvas.NewRectangle(resolver.TooltipBackgroundColor())
	bg.CornerRadius = 4
	bg.Move(geo.NewPos(tx, ty))
	bg.Resize(geo.NewSize(tooltipW, tooltipH))
	objects = append(objects, bg)

	// Border
	border := canvas.NewRectangle(color.Transparent)
	border.StrokeColor = resolver.TooltipBorderColor()
	border.StrokeWidth = 1
	border.CornerRadius = 4
	border.Move(geo.NewPos(tx, ty))
	border.Resize(geo.NewSize(tooltipW, tooltipH))
	objects = append(objects, border)

	// Lines with optional color markers
	for i, l := range lines {
		ly := ty + padding + float32(i)*lineHeight
		textX := tx + padding

		if !l.hideMarker {
			// Color marker dot
			marker := canvas.NewCircle(l.color)
			marker.Position1 = geo.NewPos(tx+padding, ly+3)
			marker.Position2 = geo.NewPos(tx+padding+markerSize, ly+3+markerSize)
			objects = append(objects, marker)
			textX += markerSize + markerGap
		}

		// Text
		text := canvas.NewText(l.text, resolver.TooltipTextColor())
		text.TextSize = 12
		text.Move(geo.NewPos(textX, ly))
		text.Resize(geo.NewSize(maxTextWidth, lineHeight))
		objects = append(objects, text)
	}

	// Crosshair lines (skip for pie charts where Label is set)
	crosshairMode := CrosshairVertical
	if cfg != nil {
		crosshairMode = cfg.Crosshair
	}
	if len(state.Hits) > 0 && crosshairMode != CrosshairNone {
		crosshairColor := resolver.CrosshairColor()
		hitPos := state.Hits[0].PixelPos

		// Vertical crosshair (clipped to plot area)
		if crosshairMode == CrosshairVertical || crosshairMode == CrosshairBoth {
			vLine := &canvas.Line{
				Position1:   geo.NewPos(hitPos.X, plotArea.Y),
				Position2:   geo.NewPos(hitPos.X, plotArea.Y+plotArea.H),
				StrokeColor: crosshairColor,
				StrokeWidth: 1,
			}
			objects = append(objects, vLine)
		}

		// Horizontal crosshair (clipped to plot area)
		if crosshairMode == CrosshairHorizontal || crosshairMode == CrosshairBoth {
			hLine := &canvas.Line{
				Position1:   geo.NewPos(plotArea.X, hitPos.Y),
				Position2:   geo.NewPos(plotArea.X+plotArea.W, hitPos.Y),
				StrokeColor: crosshairColor,
				StrokeWidth: 1,
			}
			objects = append(objects, hLine)
		}

		// Axis value tags at crosshair/axis intersections.
		objects = append(objects, renderAxisTags(state, cfg, crosshairMode, hitPos, plotArea)...)
	}

	return objects
}

type tooltipLine struct {
	text       string
	color      color.Color
	hideMarker bool // true for custom formatter lines (no series color dot)
}

// renderCrosshairOnly renders crosshair lines and axis tags without the tooltip text box.
func renderCrosshairOnly(state *TooltipState, cfg *TooltipConfig, resolver *style.Resolver, plotArea PlotArea) []core.CanvasObject {
	if len(state.Hits) == 0 {
		return nil
	}

	var objects []core.CanvasObject
	crosshairColor := resolver.CrosshairColor()
	hitPos := state.Hits[0].PixelPos

	crosshairMode := CrosshairVertical
	if cfg != nil {
		crosshairMode = cfg.Crosshair
	}

	if crosshairMode == CrosshairVertical || crosshairMode == CrosshairBoth {
		vLine := &canvas.Line{
			Position1:   geo.NewPos(hitPos.X, plotArea.Y),
			Position2:   geo.NewPos(hitPos.X, plotArea.Y+plotArea.H),
			StrokeColor: crosshairColor,
			StrokeWidth: 1,
		}
		objects = append(objects, vLine)
	}
	if crosshairMode == CrosshairHorizontal || crosshairMode == CrosshairBoth {
		hLine := &canvas.Line{
			Position1:   geo.NewPos(plotArea.X, hitPos.Y),
			Position2:   geo.NewPos(plotArea.X+plotArea.W, hitPos.Y),
			StrokeColor: crosshairColor,
			StrokeWidth: 1,
		}
		objects = append(objects, hLine)
	}

	// Axis value tags.
	objects = append(objects, renderAxisTags(state, cfg, crosshairMode, hitPos, plotArea)...)

	return objects
}

// renderAxisTags draws the axis value tags where the crosshair meets the axes:
// the X value beside the X axis, above or below the plot, and the Y value
// beside the Y axis of the first hit's series, left or right of the plot.
func renderAxisTags(state *TooltipState, cfg *TooltipConfig, crosshairMode CrosshairMode, hitPos geo.Position, plotArea PlotArea) []core.CanvasObject {
	if cfg == nil || !cfg.ShowAxisTags {
		return nil
	}
	tagBg := color.NRGBA{R: 50, G: 50, B: 55, A: 220}
	tagText := color.NRGBA{R: 220, G: 220, B: 220, A: 255}
	const (
		tagH        float32 = 16
		tagGap      float32 = 2
		tagFontSize float32 = 10
	)

	xLabel := fmt.Sprintf("%.1f", state.Hits[0].Value.X)
	yLabel := fmt.Sprintf("%.2f", state.Hits[0].Value.Y)
	if cfg.AxisTagFormatter != nil {
		xLabel, yLabel = cfg.AxisTagFormatter(state.Hits[0].Value.X, state.Hits[0].Value.Y)
	}

	tag := func(label string, x, y, w float32, align bellui.TextAlign) []core.CanvasObject {
		bg := canvas.NewRectangle(tagBg)
		bg.CornerRadius = 3
		bg.Move(geo.NewPos(x, y))
		bg.Resize(geo.NewSize(w, tagH))
		txt := canvas.NewText(label, tagText)
		txt.TextSize = tagFontSize
		txt.Alignment = align
		txt.Move(geo.NewPos(x, y+1))
		txt.Resize(geo.NewSize(w, tagH))
		return []core.CanvasObject{bg, txt}
	}

	var objects []core.CanvasObject
	if crosshairMode == CrosshairVertical || crosshairMode == CrosshairBoth {
		w := float32(len(xLabel))*6.5 + 8
		y := plotArea.Y + plotArea.H + plotArea.XAxisOffset + tagGap
		if plotArea.XAxisTop {
			y = plotArea.Y - plotArea.XAxisOffset - tagH - tagGap
		}
		objects = append(objects, tag(xLabel, hitPos.X-w/2, y, w, bellui.TextAlignCenter)...)
	}
	if crosshairMode == CrosshairHorizontal || crosshairMode == CrosshairBoth {
		w := float32(len(yLabel))*6.5 + 8
		x, align := plotArea.X-plotArea.YAxisOffset-w-tagGap, bellui.TextAlignTrailing
		if plotArea.YAxisRight {
			x, align = plotArea.X+plotArea.W+plotArea.YAxisOffset+tagGap, bellui.TextAlignLeading
		}
		objects = append(objects, tag(yLabel, x, hitPos.Y-tagH/2, w, align)...)
	}
	return objects
}

func formatHitDefault(hit interact.HitResult) string {
	name := hit.SeriesName
	if name == "" {
		name = fmt.Sprintf("Series %d", hit.SeriesIndex)
	}
	if hit.Value.Label != "" {
		return fmt.Sprintf("%s: %s (%.1f)", name, hit.Value.Label, hit.Value.Y)
	}
	return fmt.Sprintf("%s: %.1f", name, hit.Value.Y)
}
