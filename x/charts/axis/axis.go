// Package axis defines axis types, scales, and tick computation for charts.
package axis

import (
	"fmt"
	"image/color"
	"math"
	"time"

	"github.com/belluikit/bellui/x/charts/internal/mathutil"
)

// Type identifies the axis scale type.
type Type int

const (
	TypeValue    Type = iota // continuous numeric
	TypeCategory             // discrete categories
	TypeTime                 // time-based
	TypeLog                  // logarithmic
)

// Position specifies where the axis is drawn.
//
// An X axis uses PositionBottom (the zero value) or PositionTop. A Y axis uses
// PositionLeft or PositionRight; any other value keeps its default side, which
// is the left for the first Y axis and the right for every other one.
type Position int

const (
	PositionBottom Position = iota // below the plot area
	PositionTop                    // above the plot area
	PositionLeft                   // left of the plot area
	PositionRight                  // right of the plot area
)

// Config holds the user-configurable axis options (maps to ECharts xAxis/yAxis).
//
// A chart draws one X axis, from the first entry of Option.XAxis; further X
// axis entries are ignored.
type Config struct {
	Type Type
	// Name is the axis title. An X axis draws it centred below its tick
	// labels (above them for PositionTop); a Y axis draws it above the top
	// of the axis.
	Name     string
	Position Position // which side to draw on; see Position

	// For TypeCategory axes
	Data []string // category labels

	// For TypeValue/TypeLog/TypeTime axes
	Min *float64 // nil = auto
	Max *float64 // nil = auto

	SplitCount int // desired number of tick splits (default 5)
	// Inverse reverses the axis: an X axis runs right to left and a Y axis
	// top to bottom. Series, tooltips, zoom and pan all follow it.
	Inverse bool

	// Hide hides parts of the axis. Its zero value hides nothing, so a Config
	// written as a literal draws the whole axis.
	Hide Parts
	// SplitLine says whether the axis draws grid lines across the plot: at
	// its ticks, or between the bands of a category axis. SplitLineAuto, the zero value, draws them for the first Y axis
	// only: not for the X axis and not for any further Y axis. SplitLineShow
	// and SplitLineHide turn them on or off for this axis.
	SplitLine SplitLine
	// Offset moves the axis this many pixels outward from the plot edge it is
	// drawn along: down for a bottom X axis, up for a top one, left for a
	// left Y axis and right for a right one (ECharts xAxis/yAxis offset). Its
	// line, ticks, labels and name all move with it, and the chart makes room
	// for it, so an offset axis stays inside the chart. Use it to place
	// several Y axes side by side on one side: for a right-hand Y axis, an
	// Offset of 60 or more clears the labels of one at 0, and for a left-hand
	// one an Offset of the chart's left grid margin (Grid.Left) or more does.
	// Negative values count as 0.
	Offset float32

	// Deprecated: Use Hide.Line. ShowLine was never applied, and its zero
	// value, false, would hide the axis line of every Config written as a
	// literal; it has no effect.
	ShowLine bool
	// Deprecated: Use Hide.Ticks. ShowTicks was never applied; it has no
	// effect.
	ShowTicks bool
	// Deprecated: Use Hide.Labels. ShowLabels was never applied; it has no
	// effect.
	ShowLabels bool
	// Deprecated: Use SplitLine. ShowSplitLine was never applied; it has no
	// effect.
	ShowSplitLine bool

	// LineColor colours the axis line and tick marks; nil uses the theme's
	// axis colour. Grid lines keep the theme's grid colour.
	LineColor color.Color
	// LabelColor colours the tick labels and the axis name; nil uses the
	// theme's axis label colour.
	LabelColor color.Color

	// For TypeTime axes
	TimeFormat string // Go time format for labels
}

// DefaultConfig returns a Config with sensible defaults: SplitCount 5, with
// Hide, SplitLine and Offset at their zero values, so it draws the same axis
// as a literal Config of the same Type. It still sets the deprecated Show*
// fields as it always did (all true, ShowSplitLine for the value, log and time
// types), for code that reads them; the renderer does not.
func DefaultConfig(t Type) Config {
	return Config{
		Type:          t,
		SplitCount:    5,
		ShowLine:      true,
		ShowTicks:     true,
		ShowLabels:    true,
		ShowSplitLine: t == TypeValue || t == TypeLog || t == TypeTime,
	}
}

// Parts lists the parts of an axis to hide; each true field hides one. The
// zero value hides nothing.
type Parts struct {
	Line   bool // the axis line
	Ticks  bool // the tick marks; a left-hand Y axis draws none
	Labels bool // the tick labels (the axis Name stays; leave it empty to drop it)
}

// SplitLine says whether an axis draws grid lines across the plot.
type SplitLine int

const (
	// SplitLineAuto draws grid lines for the first Y axis only.
	SplitLineAuto SplitLine = iota
	// SplitLineShow draws grid lines for this axis: horizontal for a Y axis,
	// vertical for an X axis. A value, log or time axis draws them at its
	// ticks. A category axis draws them between its bands, one fewer than
	// its categories and none on the plot edges, so each line separates two
	// bars; its tick marks and labels stay at the band centres.
	SplitLineShow
	// SplitLineHide draws no grid lines for this axis.
	SplitLineHide
)

// Scale maps data values to a normalized [0, 1] range and back.
type Scale interface {
	// DataToNormalized maps a data value to [0, 1].
	DataToNormalized(value float64) float64
	// NormalizedToData maps [0, 1] back to a data value.
	NormalizedToData(norm float64) float64
	// SetRange sets the data range (auto-computed from series data).
	SetRange(dataMin, dataMax float64)
	// Range returns the current nice min/max.
	Range() (min, max float64)
	// Ticks returns the computed tick positions and labels.
	Ticks() []Tick
}

// Tick represents one tick mark on an axis.
type Tick struct {
	Value float64 // data-space value
	Norm  float64 // normalized [0, 1] position
	Label string  // display label
}

// LinearScale implements Scale for continuous numeric data.
type LinearScale struct {
	niceMin, niceMax, interval float64
	splitCount                 int
	ticks                      []Tick
	explicitMin, explicitMax   *float64
}

// NewLinearScale creates a LinearScale with an optional target tick count.
func NewLinearScale(splitCount int, explicitMin, explicitMax *float64) *LinearScale {
	if splitCount <= 0 {
		splitCount = 5
	}
	return &LinearScale{
		splitCount:  splitCount,
		explicitMin: explicitMin,
		explicitMax: explicitMax,
	}
}

// SetRange computes nice tick boundaries from the given data range.
func (s *LinearScale) SetRange(dataMin, dataMax float64) {
	dMin := dataMin
	dMax := dataMax
	if s.explicitMin != nil {
		dMin = *s.explicitMin
	}
	if s.explicitMax != nil {
		dMax = *s.explicitMax
	}
	s.niceMin, s.niceMax, s.interval = mathutil.NiceTicks(dMin, dMax, s.splitCount)
	s.buildTicks()
}

// DataToNormalized maps a data value to [0, 1] using linear interpolation.
func (s *LinearScale) DataToNormalized(value float64) float64 {
	return mathutil.InverseLerp(s.niceMin, s.niceMax, value)
}

// NormalizedToData maps a [0, 1] value back to data space.
func (s *LinearScale) NormalizedToData(norm float64) float64 {
	return mathutil.Lerp(s.niceMin, s.niceMax, norm)
}

// Range returns the nice min and max values.
func (s *LinearScale) Range() (float64, float64) {
	return s.niceMin, s.niceMax
}

// Ticks returns the computed tick marks.
func (s *LinearScale) Ticks() []Tick {
	return s.ticks
}

func (s *LinearScale) buildTicks() {
	vals := mathutil.TickValues(s.niceMin, s.niceMax, s.interval)
	s.ticks = make([]Tick, len(vals))
	for i, v := range vals {
		s.ticks[i] = Tick{
			Value: v,
			Norm:  s.DataToNormalized(v),
			Label: formatNumber(v),
		}
	}
}

// CategoryScale implements Scale for discrete categorical data.
// It supports zooming via SetRange to show a sub-range of categories.
type CategoryScale struct {
	categories []string
	rangeMin   float64 // visible range min (default 0)
	rangeMax   float64 // visible range max (default n-1)
	ticks      []Tick
}

// NewCategoryScale creates a CategoryScale from category labels.
func NewCategoryScale(categories []string) *CategoryScale {
	s := &CategoryScale{
		categories: categories,
		rangeMin:   0,
		rangeMax:   float64(len(categories) - 1),
	}
	if len(categories) == 0 {
		s.rangeMax = 0
	}
	s.buildTicks()
	return s
}

// SetRange sets the visible data range for zooming.
func (s *CategoryScale) SetRange(dataMin, dataMax float64) {
	s.rangeMin = dataMin
	s.rangeMax = dataMax
	if s.rangeMax < s.rangeMin {
		s.rangeMax = s.rangeMin
	}
	s.buildTicks()
}

// DataToNormalized maps a category index to [0, 1] within the visible range.
// Uses band positioning with padding so items don't sit on edges.
func (s *CategoryScale) DataToNormalized(value float64) float64 {
	visibleCount := s.rangeMax - s.rangeMin + 1
	if visibleCount <= 0 {
		return 0.5
	}
	// Position within the visible range, centered in band
	return (value - s.rangeMin + 0.5) / visibleCount
}

// NormalizedToData maps a [0, 1] value back to a category index.
func (s *CategoryScale) NormalizedToData(norm float64) float64 {
	visibleCount := s.rangeMax - s.rangeMin + 1
	if visibleCount <= 0 {
		return s.rangeMin
	}
	return s.rangeMin + norm*visibleCount - 0.5
}

// Range returns the visible min and max category indices.
func (s *CategoryScale) Range() (float64, float64) {
	return s.rangeMin, s.rangeMax
}

// BandWidth returns the width of one category band as a fraction of [0, 1].
func (s *CategoryScale) BandWidth() float64 {
	span := s.rangeMax - s.rangeMin + 1
	if span <= 0 {
		return 1
	}
	return 1.0 / span
}

// Ticks returns tick marks for all visible categories.
func (s *CategoryScale) Ticks() []Tick {
	return s.ticks
}

// CategoryCount returns the number of categories.
func (s *CategoryScale) CategoryCount() int {
	return len(s.categories)
}

// Label returns the category label at index i.
func (s *CategoryScale) Label(i int) string {
	if i >= 0 && i < len(s.categories) {
		return s.categories[i]
	}
	return ""
}

func (s *CategoryScale) buildTicks() {
	s.ticks = nil
	for i, cat := range s.categories {
		fi := float64(i)
		if fi < s.rangeMin-0.5 || fi > s.rangeMax+0.5 {
			continue
		}
		s.ticks = append(s.ticks, Tick{
			Value: fi,
			Norm:  s.DataToNormalized(fi),
			Label: cat,
		})
	}
}

// LogScale implements Scale for logarithmic data.
type LogScale struct {
	base                     float64
	niceMin, niceMax         float64
	splitCount               int
	ticks                    []Tick
	explicitMin, explicitMax *float64
}

// NewLogScale creates a LogScale with the given base (typically 10).
func NewLogScale(base float64, splitCount int, explicitMin, explicitMax *float64) *LogScale {
	if base <= 0 || base == 1 {
		base = 10
	}
	if splitCount <= 0 {
		splitCount = 5
	}
	return &LogScale{
		base:        base,
		splitCount:  splitCount,
		explicitMin: explicitMin,
		explicitMax: explicitMax,
	}
}

// SetRange computes logarithmic tick boundaries from the given data range.
func (s *LogScale) SetRange(dataMin, dataMax float64) {
	dMin := dataMin
	dMax := dataMax
	if s.explicitMin != nil {
		dMin = *s.explicitMin
	}
	if s.explicitMax != nil {
		dMax = *s.explicitMax
	}
	if dMin <= 0 {
		dMin = 1
	}
	if dMax <= dMin {
		dMax = dMin * s.base
	}
	s.niceMin = math.Pow(s.base, math.Floor(math.Log(dMin)/math.Log(s.base)))
	s.niceMax = math.Pow(s.base, math.Ceil(math.Log(dMax)/math.Log(s.base)))
	s.buildTicks()
}

// DataToNormalized maps a data value to [0, 1] using logarithmic interpolation.
func (s *LogScale) DataToNormalized(value float64) float64 {
	if value <= 0 {
		return 0
	}
	return (math.Log(value) - math.Log(s.niceMin)) / (math.Log(s.niceMax) - math.Log(s.niceMin))
}

// NormalizedToData maps a [0, 1] value back to data space using exponential interpolation.
func (s *LogScale) NormalizedToData(norm float64) float64 {
	logMin := math.Log(s.niceMin)
	logMax := math.Log(s.niceMax)
	return math.Exp(logMin + norm*(logMax-logMin))
}

// Range returns the nice min and max values.
func (s *LogScale) Range() (float64, float64) {
	return s.niceMin, s.niceMax
}

// Ticks returns the computed logarithmic tick marks.
func (s *LogScale) Ticks() []Tick {
	return s.ticks
}

func (s *LogScale) buildTicks() {
	s.ticks = nil
	for v := s.niceMin; v <= s.niceMax*1.001; v *= s.base {
		s.ticks = append(s.ticks, Tick{
			Value: v,
			Norm:  s.DataToNormalized(v),
			Label: formatNumber(v),
		})
	}
}

// TimeScale implements Scale for time-based data.
type TimeScale struct {
	minTime, maxTime time.Time
	format           string
	splitCount       int
	ticks            []Tick
}

// NewTimeScale creates a TimeScale.
func NewTimeScale(splitCount int, format string) *TimeScale {
	if splitCount <= 0 {
		splitCount = 5
	}
	if format == "" {
		format = "2006-01-02"
	}
	return &TimeScale{splitCount: splitCount, format: format}
}

// SetTimeRange sets the time range directly.
func (s *TimeScale) SetTimeRange(min, max time.Time) {
	s.minTime = min
	s.maxTime = max
	s.buildTicks()
}

// SetRange sets the time range from Unix timestamps.
func (s *TimeScale) SetRange(dataMin, dataMax float64) {
	s.minTime = time.Unix(int64(dataMin), 0)
	s.maxTime = time.Unix(int64(dataMax), 0)
	s.buildTicks()
}

// DataToNormalized maps a Unix timestamp to [0, 1] within the time range.
func (s *TimeScale) DataToNormalized(value float64) float64 {
	t := time.Unix(int64(value), 0)
	dur := s.maxTime.Sub(s.minTime)
	if dur == 0 {
		return 0.5
	}
	return float64(t.Sub(s.minTime)) / float64(dur)
}

// NormalizedToData maps a [0, 1] value back to a Unix timestamp.
func (s *TimeScale) NormalizedToData(norm float64) float64 {
	dur := s.maxTime.Sub(s.minTime)
	t := s.minTime.Add(time.Duration(norm * float64(dur)))
	return float64(t.Unix())
}

// Range returns the time range as Unix timestamps.
func (s *TimeScale) Range() (float64, float64) {
	return float64(s.minTime.Unix()), float64(s.maxTime.Unix())
}

// Ticks returns the computed time-based tick marks.
func (s *TimeScale) Ticks() []Tick {
	return s.ticks
}

func (s *TimeScale) buildTicks() {
	s.ticks = nil
	dur := s.maxTime.Sub(s.minTime)
	if dur <= 0 {
		return
	}
	step := dur / time.Duration(s.splitCount)
	for i := 0; i <= s.splitCount; i++ {
		t := s.minTime.Add(step * time.Duration(i))
		val := float64(t.Unix())
		s.ticks = append(s.ticks, Tick{
			Value: val,
			Norm:  s.DataToNormalized(val),
			Label: t.Format(s.format),
		})
	}
}

// NewScale creates the appropriate Scale for the given axis Config.
func NewScale(cfg Config) Scale {
	switch cfg.Type {
	case TypeCategory:
		return NewCategoryScale(cfg.Data)
	case TypeLog:
		return NewLogScale(10, cfg.SplitCount, cfg.Min, cfg.Max)
	case TypeTime:
		return NewTimeScale(cfg.SplitCount, cfg.TimeFormat)
	default:
		return NewLinearScale(cfg.SplitCount, cfg.Min, cfg.Max)
	}
}

func formatNumber(v float64) string {
	if v == math.Trunc(v) && math.Abs(v) < 1e15 {
		return fmt.Sprintf("%d", int64(v))
	}
	return fmt.Sprintf("%.2f", v)
}
