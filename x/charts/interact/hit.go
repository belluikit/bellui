// Package interact provides hit testing, event dispatch, and interaction state for charts.
package interact

import (
	"fmt"
	"math"

	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/x/charts/coord"
	"github.com/belluikit/bellui/x/charts/series"
)

// HitResult describes a data point found near a pixel position.
type HitResult struct {
	SeriesIndex int          // index of the series in the chart option
	DataIndex   int          // index of the data point within the series
	SeriesName  string       // display name of the series
	Value       DataValue    // resolved data coordinates
	PixelPos    geo.Position // pixel position of the data point
	Distance    float32      // pixel distance from the query position
}

// DataValue holds the resolved data at a hit point.
type DataValue struct {
	X     float64 // X coordinate in data space
	Y     float64 // Y coordinate in data space
	Label string  // optional label (e.g. pie slice name)
}

// TriggerMode controls how hit testing behaves.
type TriggerMode int

const (
	TriggerItem TriggerMode = iota // nearest single data point
	TriggerAxis                    // all series at nearest X
)

// defaultGridCellSize is the default cell size in pixels for the spatial grid.
// It is set to match the typical hit-test threshold (~30px) so that a 3x3
// neighbourhood lookup covers the threshold radius.
const defaultGridCellSize float32 = 30

// HitTester finds data points near a pixel position.
type HitTester struct {
	entries []hitEntry
	grid    spatialGrid
}

// spatialGrid is a simple uniform-grid spatial index that maps pixel positions
// to the (entryIdx, pointIdx) pairs stored in HitTester.entries[].points[].
// It maintains two indices:
//   - cells: a 2D grid for FindNearest (keyed by (cx, cy) cell coordinates)
//   - xCols: a 1D column index for FindAllAtX (keyed by cx only)
type spatialGrid struct {
	cellSize float32
	cells    map[gridKey][]gridEntry
	xCols    map[int][]gridEntry
}

type gridKey struct {
	cx, cy int
}

type gridEntry struct {
	entryIdx int // index into HitTester.entries
	pointIdx int // index into entry.points
}

type hitEntry struct {
	seriesIndex int
	seriesName  string
	seriesType  series.Type
	points      []hitPoint
	bars        []hitBar // for bar series
	// For pie series
	pieCenter geo.Position
	pieRadius float32
	sectors   []PieSectorData
	// For heatmap series
	heatmap *heatmapGrid
}

type heatmapGrid struct {
	viewport coord.Rect
	rows     int
	cols     int
	data     []float64
	xLabels  []string
	yLabels  []string
}

type hitPoint struct {
	dataIndex int
	pixelPos  geo.Position
	value     DataValue
}

type hitBar struct {
	dataIndex int
	rect      barRect      // pixel-space rectangle
	pixelPos  geo.Position // center of bar (for tooltip positioning)
	value     DataValue
}

type barRect struct {
	x, y, w, h float32
}

// PieSectorData describes a pie sector for hit testing.
type PieSectorData struct {
	dataIndex  int
	startAngle float32
	endAngle   float32
	name       string
	value      float64
}

// NewHitTester creates an empty HitTester.
func NewHitTester() *HitTester {
	return &HitTester{
		grid: spatialGrid{
			cellSize: defaultGridCellSize,
			cells:    make(map[gridKey][]gridEntry),
			xCols:    make(map[int][]gridEntry),
		},
	}
}

// newGridKey returns the grid cell key for a pixel position.
func (g *spatialGrid) key(px, py float32) gridKey {
	return gridKey{
		cx: int(math.Floor(float64(px / g.cellSize))),
		cy: int(math.Floor(float64(py / g.cellSize))),
	}
}

// insert adds a point to the spatial grid.
func (g *spatialGrid) insert(px, py float32, entryIdx, pointIdx int) {
	k := g.key(px, py)
	ge := gridEntry{entryIdx: entryIdx, pointIdx: pointIdx}
	g.cells[k] = append(g.cells[k], ge)
	g.xCols[k.cx] = append(g.xCols[k.cx], ge)
}

// clear removes all entries from the grid.
func (g *spatialGrid) clear() {
	// Reuse the maps by deleting all keys (avoids allocation of new maps).
	for k := range g.cells {
		delete(g.cells, k)
	}
	for k := range g.xCols {
		delete(g.xCols, k)
	}
}

// Clear removes all registered series.
func (ht *HitTester) Clear() {
	ht.entries = ht.entries[:0]
	ht.grid.clear()
}

// indexEntryPoints inserts all points of the entry at entryIdx into the spatial grid.
func (ht *HitTester) indexEntryPoints(entryIdx int) {
	for pi, p := range ht.entries[entryIdx].points {
		ht.grid.insert(p.pixelPos.X, p.pixelPos.Y, entryIdx, pi)
	}
}

// AddHitPoints registers point-based hit areas from a HitRegistrar series.
// Implements series.HitAreaCollector.
func (ht *HitTester) AddHitPoints(seriesIndex int, name string, sType series.Type, points []series.HitPoint) {
	e := hitEntry{
		seriesIndex: seriesIndex,
		seriesName:  name,
		seriesType:  sType,
		points:      make([]hitPoint, len(points)),
	}
	for i, p := range points {
		e.points[i] = hitPoint{
			dataIndex: p.DataIndex,
			pixelPos:  p.PixelPos,
			value:     DataValue{X: p.Value.X, Y: p.Value.Y, Label: p.Value.Label},
		}
	}
	ht.entries = append(ht.entries, e)
	ht.indexEntryPoints(len(ht.entries) - 1)
}

// AddHitBars registers rectangle-based hit areas from a HitRegistrar series.
// Implements series.HitAreaCollector.
func (ht *HitTester) AddHitBars(seriesIndex int, name string, sType series.Type, bars []series.HitBar) {
	e := hitEntry{
		seriesIndex: seriesIndex,
		seriesName:  name,
		seriesType:  sType,
		bars:        make([]hitBar, len(bars)),
	}
	for i, b := range bars {
		e.bars[i] = hitBar{
			dataIndex: b.DataIndex,
			rect:      barRect{x: b.Rect[0], y: b.Rect[1], w: b.Rect[2], h: b.Rect[3]},
			pixelPos:  b.PixelPos,
			value:     DataValue{X: b.Value.X, Y: b.Value.Y, Label: b.Value.Label},
		}
	}
	ht.entries = append(ht.entries, e)
}

// AddLineSeries registers pixel positions for a line series.
func (ht *HitTester) AddLineSeries(seriesIndex int, name string, sType series.Type, coordSys coord.System, data []float64) {
	e := hitEntry{
		seriesIndex: seriesIndex,
		seriesName:  name,
		seriesType:  sType,
		points:      make([]hitPoint, len(data)),
	}
	for i, v := range data {
		pos := coordSys.DataToPixel(float64(i), v)
		e.points[i] = hitPoint{
			dataIndex: i,
			pixelPos:  pos,
			value:     DataValue{X: float64(i), Y: v},
		}
	}
	ht.entries = append(ht.entries, e)
	ht.indexEntryPoints(len(ht.entries) - 1)
}

// AddBarSeries registers bar rectangles for hit testing.
// barWidth is the pixel width of each bar.
func (ht *HitTester) AddBarSeries(seriesIndex int, name string, coordSys coord.System, data []float64, barWidth float32) {
	baselinePos := coordSys.DataToPixel(0, 0)
	baseY := baselinePos.Y

	e := hitEntry{
		seriesIndex: seriesIndex,
		seriesName:  name,
		seriesType:  series.TypeBar,
		bars:        make([]hitBar, len(data)),
	}
	for i, v := range data {
		pos := coordSys.DataToPixel(float64(i), v)
		bx := pos.X - barWidth/2
		by := pos.Y
		bh := baseY - pos.Y
		if bh < 0 {
			by = baseY
			bh = -bh
		}
		e.bars[i] = hitBar{
			dataIndex: i,
			rect:      barRect{x: bx, y: by, w: barWidth, h: bh},
			pixelPos:  geo.NewPos(pos.X, pos.Y),
			value:     DataValue{X: float64(i), Y: v},
		}
	}
	ht.entries = append(ht.entries, e)
}

// AddScatterSeries registers pixel positions for a scatter series.
func (ht *HitTester) AddScatterSeries(seriesIndex int, name string, coordSys coord.System, ss *series.ScatterSeries) {
	n := ss.PointCount()
	e := hitEntry{
		seriesIndex: seriesIndex,
		seriesName:  name,
		seriesType:  series.TypeScatter,
		points:      make([]hitPoint, n),
	}
	for i := 0; i < n; i++ {
		x, y := ss.Point(i)
		pos := coordSys.DataToPixel(x, y)
		e.points[i] = hitPoint{
			dataIndex: i,
			pixelPos:  pos,
			value:     DataValue{X: x, Y: y},
		}
	}
	ht.entries = append(ht.entries, e)
	ht.indexEntryPoints(len(ht.entries) - 1)
}

// AddCandlestickBars registers hit rectangles covering the full low-to-high range of each candle.
func (ht *HitTester) AddCandlestickBars(seriesIndex int, name string, coordSys coord.System, data []series.CandlestickData, barWidth float32) {
	e := hitEntry{
		seriesIndex: seriesIndex,
		seriesName:  name,
		seriesType:  series.TypeCandlestick,
		bars:        make([]hitBar, len(data)),
	}
	for i, d := range data {
		highPos := coordSys.DataToPixel(float64(i), d.High)
		lowPos := coordSys.DataToPixel(float64(i), d.Low)
		cx := highPos.X

		// Rect from high (top) to low (bottom) in pixel space
		top := highPos.Y
		bot := lowPos.Y
		if top > bot {
			top, bot = bot, top
		}

		e.bars[i] = hitBar{
			dataIndex: i,
			rect:      barRect{x: cx - barWidth/2, y: top, w: barWidth, h: bot - top},
			pixelPos:  geo.NewPos(cx, (top+bot)/2),
			value:     DataValue{X: float64(i), Y: d.Close, Label: fmt.Sprintf("O:%.0f H:%.0f L:%.0f C:%.0f", d.Open, d.High, d.Low, d.Close)},
		}
	}
	ht.entries = append(ht.entries, e)
}

// AddGeoPoints registers pre-projected pixel positions for geo scatter hit testing.
// pixelData is [px0, py0, px1, py1, ...] in widget pixel coordinates.
func (ht *HitTester) AddGeoPoints(seriesIndex int, name string, pixelData []float64) {
	n := len(pixelData) / 2
	e := hitEntry{
		seriesIndex: seriesIndex,
		seriesName:  name,
		seriesType:  series.TypeGeoScatter,
		points:      make([]hitPoint, n),
	}
	for i := 0; i < n; i++ {
		px := float32(pixelData[i*2])
		py := float32(pixelData[i*2+1])
		e.points[i] = hitPoint{
			dataIndex: i,
			pixelPos:  geo.NewPos(px, py),
			value:     DataValue{X: pixelData[i*2], Y: pixelData[i*2+1], Label: fmt.Sprintf("Point %d", i)},
		}
	}
	ht.entries = append(ht.entries, e)
	ht.indexEntryPoints(len(ht.entries) - 1)
}

// AddGeoPointsWithLabels registers pre-projected pixel positions with custom labels.
func (ht *HitTester) AddGeoPointsWithLabels(seriesIndex int, name string, pixelData []float64, labels []string) {
	n := len(pixelData) / 2
	e := hitEntry{
		seriesIndex: seriesIndex,
		seriesName:  name,
		seriesType:  series.TypeChoropleth,
		points:      make([]hitPoint, n),
	}
	for i := 0; i < n; i++ {
		px := float32(pixelData[i*2])
		py := float32(pixelData[i*2+1])
		label := ""
		if i < len(labels) {
			label = labels[i]
		}
		e.points[i] = hitPoint{
			dataIndex: i,
			pixelPos:  geo.NewPos(px, py),
			value:     DataValue{X: pixelData[i*2], Y: pixelData[i*2+1], Label: label},
		}
	}
	ht.entries = append(ht.entries, e)
	ht.indexEntryPoints(len(ht.entries) - 1)
}

// AddHeatmapSeries registers a heatmap grid for hit testing.
func (ht *HitTester) AddHeatmapSeries(seriesIndex int, name string, viewport coord.Rect, rows, cols int, data []float64, xLabels, yLabels []string) {
	ht.entries = append(ht.entries, hitEntry{
		seriesIndex: seriesIndex,
		seriesName:  name,
		seriesType:  series.TypeHeatmap,
		heatmap: &heatmapGrid{
			viewport: viewport,
			rows:     rows,
			cols:     cols,
			data:     data,
			xLabels:  xLabels,
			yLabels:  yLabels,
		},
	})
}

// AddPieSeries registers pie sectors for hit testing.
func (ht *HitTester) AddPieSeries(seriesIndex int, name string, center geo.Position, radius float32, sectors []PieSectorData) {
	ht.entries = append(ht.entries, hitEntry{
		seriesIndex: seriesIndex,
		seriesName:  name,
		seriesType:  series.TypePie,
		pieCenter:   center,
		pieRadius:   radius,
		sectors:     sectors,
	})
}

// MakePieSector creates a PieSectorData for AddPieSeries.
func MakePieSector(dataIndex int, startAngle, endAngle float32, name string, value float64) PieSectorData {
	return PieSectorData{
		dataIndex:  dataIndex,
		startAngle: startAngle,
		endAngle:   endAngle,
		name:       name,
		value:      value,
	}
}

// FindNearest returns the nearest hit to the given position within threshold pixels.
// Returns nil if nothing is close enough.
func (ht *HitTester) FindNearest(pos geo.Position, threshold float32) *HitResult {
	var best *HitResult
	bestDist := float32(math.MaxFloat32)

	// First pass: check pie, heatmap, and bar entries (these don't use the spatial grid).
	for _, e := range ht.entries {
		if e.seriesType == series.TypePie {
			if r := ht.hitTestPie(e, pos); r != nil {
				return r
			}
			continue
		}

		// Heatmap: grid cell lookup
		if e.heatmap != nil {
			if r := ht.hitTestHeatmap(e, pos); r != nil {
				return r
			}
			continue
		}

		// Bar series: test rectangle containment
		for _, b := range e.bars {
			if pos.X >= b.rect.x && pos.X <= b.rect.x+b.rect.w &&
				pos.Y >= b.rect.y && pos.Y <= b.rect.y+b.rect.h {
				return &HitResult{
					SeriesIndex: e.seriesIndex,
					DataIndex:   b.dataIndex,
					SeriesName:  e.seriesName,
					Value:       b.value,
					PixelPos:    b.pixelPos,
					Distance:    0,
				}
			}
		}
	}

	// Second pass: point-based series via spatial grid.
	// Query the 3x3 neighbourhood of cells around pos, expanded to cover the threshold.
	cellRadius := int(math.Ceil(float64(threshold / ht.grid.cellSize)))
	center := ht.grid.key(pos.X, pos.Y)
	for dcx := -cellRadius; dcx <= cellRadius; dcx++ {
		for dcy := -cellRadius; dcy <= cellRadius; dcy++ {
			k := gridKey{cx: center.cx + dcx, cy: center.cy + dcy}
			for _, ge := range ht.grid.cells[k] {
				p := ht.entries[ge.entryIdx].points[ge.pointIdx]
				dx := pos.X - p.pixelPos.X
				dy := pos.Y - p.pixelPos.Y
				dist := float32(math.Sqrt(float64(dx*dx + dy*dy)))
				if dist < bestDist && dist <= threshold {
					bestDist = dist
					e := ht.entries[ge.entryIdx]
					best = &HitResult{
						SeriesIndex: e.seriesIndex,
						DataIndex:   p.dataIndex,
						SeriesName:  e.seriesName,
						Value:       p.value,
						PixelPos:    p.pixelPos,
						Distance:    dist,
					}
				}
			}
		}
	}

	return best
}

// FindAllAtX returns hits from all series at the nearest X position (axis trigger mode).
func (ht *HitTester) FindAllAtX(pos geo.Position, threshold float32) []HitResult {
	// Use the xCols index to find the nearest X across all point-based series.
	// We scan X columns within the threshold radius.
	nearestX := float32(math.MaxFloat32)
	nearestXDist := float32(math.MaxFloat32)

	cellRadius := int(math.Ceil(float64(threshold / ht.grid.cellSize)))
	centerCX := int(math.Floor(float64(pos.X / ht.grid.cellSize)))

	for dcx := -cellRadius; dcx <= cellRadius; dcx++ {
		for _, ge := range ht.grid.xCols[centerCX+dcx] {
			p := ht.entries[ge.entryIdx].points[ge.pointIdx]
			dx := float32(math.Abs(float64(pos.X - p.pixelPos.X)))
			if dx < nearestXDist {
				nearestXDist = dx
				nearestX = p.pixelPos.X
			}
		}
	}

	if nearestXDist > threshold {
		return nil
	}

	// Collect one hit per series at the nearest X.
	type seriesHit struct {
		hit HitResult
	}
	seriesHits := make(map[int]*seriesHit) // keyed by entryIdx

	// Scan X columns in a narrow band around nearestX.
	nearCX := int(math.Floor(float64(nearestX / ht.grid.cellSize)))
	for dcx := -1; dcx <= 1; dcx++ {
		for _, ge := range ht.grid.xCols[nearCX+dcx] {
			p := ht.entries[ge.entryIdx].points[ge.pointIdx]
			if float32(math.Abs(float64(p.pixelPos.X-nearestX))) < 1 {
				if _, ok := seriesHits[ge.entryIdx]; !ok {
					e := ht.entries[ge.entryIdx]
					seriesHits[ge.entryIdx] = &seriesHit{
						hit: HitResult{
							SeriesIndex: e.seriesIndex,
							DataIndex:   p.dataIndex,
							SeriesName:  e.seriesName,
							Value:       p.value,
							PixelPos:    p.pixelPos,
						},
					}
				}
			}
		}
	}

	if len(seriesHits) == 0 {
		return nil
	}

	// Return results in entry order (stable ordering).
	results := make([]HitResult, 0, len(seriesHits))
	for ei := range ht.entries {
		if sh, ok := seriesHits[ei]; ok {
			results = append(results, sh.hit)
		}
	}

	return results
}

func (ht *HitTester) hitTestPie(e hitEntry, pos geo.Position) *HitResult {
	dx := pos.X - e.pieCenter.X
	dy := pos.Y - e.pieCenter.Y
	dist := float32(math.Sqrt(float64(dx*dx + dy*dy)))

	if dist > e.pieRadius {
		return nil
	}

	// Compute angle (0=top, clockwise)
	angle := float32(math.Atan2(float64(dy), float64(dx))*180/math.Pi) + 90
	if angle < 0 {
		angle += 360
	}

	for _, s := range e.sectors {
		if angle >= s.startAngle && angle < s.endAngle {
			return &HitResult{
				SeriesIndex: e.seriesIndex,
				DataIndex:   s.dataIndex,
				SeriesName:  e.seriesName,
				Value:       DataValue{Label: s.name, Y: s.value},
				PixelPos:    e.pieCenter,
			}
		}
	}

	return nil
}

func (ht *HitTester) hitTestHeatmap(e hitEntry, pos geo.Position) *HitResult {
	g := e.heatmap
	vp := g.viewport

	// Check if pos is inside the heatmap viewport
	if pos.X < vp.X || pos.X > vp.X+vp.W || pos.Y < vp.Y || pos.Y > vp.Y+vp.H {
		return nil
	}

	// Determine which cell (row, col) the cursor is in
	relX := pos.X - vp.X
	relY := pos.Y - vp.Y
	col := int(relX / (vp.W / float32(g.cols)))
	row := int(relY / (vp.H / float32(g.rows)))

	if col >= g.cols {
		col = g.cols - 1
	}
	if row >= g.rows {
		row = g.rows - 1
	}

	idx := row*g.cols + col
	if idx < 0 || idx >= len(g.data) {
		return nil
	}

	// Build label from axis labels if available
	label := fmt.Sprintf("[%d, %d]", row, col)
	if col < len(g.xLabels) && row < len(g.yLabels) {
		label = fmt.Sprintf("%s, %s", g.yLabels[row], g.xLabels[col])
	} else if col < len(g.xLabels) {
		label = fmt.Sprintf("Row %d, %s", row, g.xLabels[col])
	} else if row < len(g.yLabels) {
		label = fmt.Sprintf("%s, Col %d", g.yLabels[row], col)
	}

	// Pixel position at the center of the cell
	cellW := vp.W / float32(g.cols)
	cellH := vp.H / float32(g.rows)
	cellCenterX := vp.X + float32(col)*cellW + cellW/2
	cellCenterY := vp.Y + float32(row)*cellH + cellH/2

	return &HitResult{
		SeriesIndex: e.seriesIndex,
		DataIndex:   idx,
		SeriesName:  e.seriesName,
		Value:       DataValue{X: float64(col), Y: g.data[idx], Label: label},
		PixelPos:    geo.NewPos(cellCenterX, cellCenterY),
		Distance:    0,
	}
}
