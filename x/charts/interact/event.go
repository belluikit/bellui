package interact

import (
	"sync"

	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
)

// EventType enumerates chart event kinds.
type EventType int

const (
	EventClick              EventType = iota // tap/click on data point
	EventDoubleClick                         // double-tap
	EventHover                               // mouse over data point
	EventMouseOut                            // mouse left chart
	EventLegendToggle                        // legend item toggled
	EventZoomChange                          // data window changed
	EventBrushSelect                         // brush selection completed
	EventDrilldown                           // navigated deeper into data
	EventDrillUp                             // navigated back up
	EventKeyPress                            // keyboard key pressed while chart focused
	EventDrawingCreated                      // new drawing finalized
	EventDrawingSelected                     // existing drawing clicked/selected
	EventDrawingDeleted                      // drawing removed
	EventDrawingToolChanged                  // active drawing tool changed (DrawingToolEvent)
)

// Event is the base payload for all chart events.
type Event struct {
	Type EventType
}

// DataPointEvent fires for click/hover on data.
type DataPointEvent struct {
	Event
	SeriesIndex int
	SeriesName  string
	DataIndex   int
	Value       DataValue
	PixelPos    geo.Position // pixel position of the event (for crosshair sync)
}

// LegendToggleEvent fires when a legend item is toggled.
type LegendToggleEvent struct {
	Event
	SeriesIndex int
	SeriesName  string
	Visible     bool
}

// ZoomChangeEvent fires when the data window changes.
type ZoomChangeEvent struct {
	Event
	Start, End float64 // normalized [0, 1]
	FromUser   bool
}

// DrawingToolEvent fires when the chart's active drawing tool changes, from
// any source: a drawing-tools UI, the app calling SetDrawingTool, or the chart
// itself. An app that replaces the default drawing tools keeps its own UI in
// sync with it (#796).
//
// Experimental: added with the drawing-tools UI (#796) and may change until a
// second app uses it. Chart.DrawingTool reads the same state.
type DrawingToolEvent struct {
	Event
	Tool     DrawingToolMode // the tool now active
	Previous DrawingToolMode // the tool active before
}

// BrushSelectEvent fires when brush selection completes.
type BrushSelectEvent struct {
	Event
	DataMinX, DataMaxX float64              // X range of the selection in data space
	DataMinY, DataMaxY float64              // Y range of the selection in data space
	Selected           []BrushSelectedPoint // points within the selection
}

// BrushSelectedPoint identifies a data point within a brush selection.
type BrushSelectedPoint struct {
	SeriesIndex int       // index of the series in the chart option
	DataIndex   int       // index of the data point within the series
	Value       DataValue // resolved data coordinates
}

// DrilldownEvent fires when the user drills into or out of data.
type DrilldownEvent struct {
	Event
	Path []string   // current breadcrumb path after navigation
	Hit  *HitResult // the data point that triggered drilldown (nil for drill-up)
}

// KeyPressEvent fires when a key is pressed while the chart is focused.
type KeyPressEvent struct {
	Event
	KeyName event.KeyName // named key (e.g., event.KeyEscape, event.KeyPlus)
	Rune    rune          // typed character (0 if not a printable key)
}

// DrawingEvent fires when a drawing is created, selected, or deleted.
type DrawingEvent struct {
	Event
	DrawingID DrawingID
	Drawing   Drawing // nil for delete events
}

// EventHandler is a callback for chart events.
type EventHandler func(evt interface{})

// EventFilter restricts which events a handler receives.
// Zero-value fields match all events.
type EventFilter struct {
	SeriesIndex *int        // nil = all series
	EventTypes  []EventType // empty = all types
}

// EventBus manages event subscriptions.
// It uses a copy-on-write snapshot so Emit never allocates.
type EventBus struct {
	mu       sync.Mutex
	handlers []handlerEntry // mutable list; replaced on every On/Off
	snap     []handlerEntry // immutable snapshot shared with Emit
	nextID   int
}

type handlerEntry struct {
	id      int
	filter  EventFilter
	handler EventHandler
}

// NewEventBus creates an EventBus.
func NewEventBus() *EventBus {
	return &EventBus{}
}

// On registers a handler for a specific event type. Returns an ID for removal.
func (b *EventBus) On(eventType EventType, handler EventHandler) int {
	return b.OnFiltered(EventFilter{EventTypes: []EventType{eventType}}, handler)
}

// OnFiltered registers a handler with a filter. Returns an ID for removal.
func (b *EventBus) OnFiltered(filter EventFilter, handler EventHandler) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.nextID++
	b.handlers = append(b.handlers, handlerEntry{
		id:      b.nextID,
		filter:  filter,
		handler: handler,
	})
	b.rebuildSnap()
	return b.nextID
}

// Off removes a handler by ID.
func (b *EventBus) Off(id int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for i, h := range b.handlers {
		if h.id == id {
			b.handlers = append(b.handlers[:i], b.handlers[i+1:]...)
			b.rebuildSnap()
			return
		}
	}
}

// rebuildSnap creates an immutable snapshot of handlers. Must be called with mu held.
func (b *EventBus) rebuildSnap() {
	s := make([]handlerEntry, len(b.handlers))
	copy(s, b.handlers)
	b.snap = s
}

// Emit dispatches an event to all matching handlers.
// It reads a pre-built snapshot, avoiding allocation on the hot path.
func (b *EventBus) Emit(evt interface{}) {
	b.mu.Lock()
	snap := b.snap
	b.mu.Unlock()

	if len(snap) == 0 {
		return
	}

	evtType := eventTypeOf(evt)

	for _, h := range snap {
		if !h.matchesType(evtType) {
			continue
		}
		if !h.matchesSeries(evt) {
			continue
		}
		h.handler(evt)
	}
}

func (h *handlerEntry) matchesType(t EventType) bool {
	if len(h.filter.EventTypes) == 0 {
		return true
	}
	for _, ft := range h.filter.EventTypes {
		if ft == t {
			return true
		}
	}
	return false
}

func (h *handlerEntry) matchesSeries(evt interface{}) bool {
	if h.filter.SeriesIndex == nil {
		return true
	}
	target := *h.filter.SeriesIndex
	switch e := evt.(type) {
	case DataPointEvent:
		return e.SeriesIndex == target
	case LegendToggleEvent:
		return e.SeriesIndex == target
	}
	return true
}

func eventTypeOf(evt interface{}) EventType {
	switch e := evt.(type) {
	case DataPointEvent:
		return e.Type
	case LegendToggleEvent:
		return e.Type
	case ZoomChangeEvent:
		return e.Type
	case BrushSelectEvent:
		return e.Type
	case DrilldownEvent:
		return e.Type
	case KeyPressEvent:
		return e.Type
	case DrawingEvent:
		return e.Type
	case DrawingToolEvent:
		return e.Type
	case Event:
		return e.Type
	}
	return EventClick
}
