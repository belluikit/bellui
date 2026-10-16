package app

import (
	"sort"
	"sync"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/mobile"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/driver"
	"github.com/belluikit/bellui/lang"
)

// FocusManager represents a standard manager of input focus for a canvas.
// FocusManager is safe for concurrent use.
type FocusManager struct {
	content core.CanvasObject
	mu      sync.Mutex
	focused event.Focusable
}

// NewFocusManager returns a new instance of the standard focus manager for a canvas.
func NewFocusManager(c core.CanvasObject) *FocusManager {
	return &FocusManager{content: c}
}

// Focus focuses the given obj.
func (f *FocusManager) Focus(obj event.Focusable) bool {
	if obj != nil {
		var hiddenAncestor core.CanvasObject
		hidden := false
		found := driver.WalkCompleteObjectTree(
			f.content,
			func(object core.CanvasObject, _, _ geo.Position, _ geo.Size) bool {
				if hiddenAncestor == nil && !object.Visible() {
					hiddenAncestor = object
				}
				if object == obj.(core.CanvasObject) {
					hidden = hiddenAncestor != nil
					return true
				}
				return false
			},
			func(object core.CanvasObject, pos geo.Position, _ core.CanvasObject) {
				if hiddenAncestor == object {
					hiddenAncestor = nil
				}
			},
		)
		if !found {
			return false
		}
		if hidden {
			return true
		}
		if dis, ok := obj.(core.Disableable); ok && dis.Disabled() {
			type selectableText interface {
				SelectedText() string
			}
			if _, isSelectableText := obj.(selectableText); !isSelectableText {
				return true
			}
			// On mobile a disabled text *input* must not take focus (it would
			// summon the soft keyboard), but a read-only selectable — which
			// reports Disabled only to skip tab traversal — must, so touch
			// selection can show its highlight and handles (pre-rebrand tracker #944). The
			// distinction is Keyboardable: only keyboard-summoning widgets
			// stay excluded.
			if _, needsKeyboard := obj.(mobile.Keyboardable); needsKeyboard && bellui.CurrentDevice().IsMobile() {
				return true
			}
		}
	}
	f.focus(obj)
	return true
}

// Focused returns the currently focused object or nil if none.
func (f *FocusManager) Focused() event.Focusable {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.focused
}

// FocusGained signals to the manager that its content got focus (due to window/overlay switch for instance).
func (f *FocusManager) FocusGained() {
	f.mu.Lock()
	focused := f.focused
	f.mu.Unlock()
	if focused != nil {
		focused.FocusGained()
	}
}

// FocusLost signals to the manager that its content lost focus (due to window/overlay switch for instance).
func (f *FocusManager) FocusLost() {
	f.mu.Lock()
	focused := f.focused
	f.mu.Unlock()
	if focused != nil {
		focused.FocusLost()
	}
}

// FocusNext will find the item after the current that can be focused and focus it.
// If current is nil then the first focusable item in the canvas will be focused.
func (f *FocusManager) FocusNext() {
	f.mu.Lock()
	current := f.focused
	f.mu.Unlock()
	f.focus(f.nextInChain(current))
}

// FocusPrevious will find the item before the current that can be focused and focus it.
// If current is nil then the last focusable item in the canvas will be focused.
func (f *FocusManager) FocusPrevious() {
	f.mu.Lock()
	current := f.focused
	f.mu.Unlock()
	f.focus(f.previousInChain(current))
}

func (f *FocusManager) focus(obj event.Focusable) {
	f.mu.Lock()
	if f.focused == obj {
		f.mu.Unlock()
		return
	}

	old := f.focused
	f.focused = obj
	f.mu.Unlock()

	// Call callbacks outside the lock to avoid deadlocks with widget code
	// that may call back into the FocusManager.
	if old != nil {
		old.FocusLost()
	}
	if obj != nil {
		obj.FocusGained()
	}
}

func (f *FocusManager) nextInChain(current event.Focusable) event.Focusable {
	return f.step(current, 1)
}

func (f *FocusManager) previousInChain(current event.Focusable) event.Focusable {
	return f.step(current, -1)
}

// focusEntry pairs a focusable with its absolute position and size so the focus
// order can be re-derived from the visual arrangement under a right-to-left
// layout.
type focusEntry struct {
	focusable event.Focusable
	pos       geo.Position
	size      geo.Size
}

// step returns the focusable dir places from current in the focus order — dir is
// +1 for the next item and -1 for the previous — wrapping around the ends. When
// current is nil the first (next) or last (previous) focusable is returned.
func (f *FocusManager) step(current event.Focusable, dir int) event.Focusable {
	order := f.focusOrder()
	if len(order) == 0 {
		return nil
	}

	edge := func(forward bool) event.Focusable {
		if forward {
			return order[0].focusable
		}
		return order[len(order)-1].focusable
	}
	if current == nil {
		return edge(dir > 0)
	}
	for i, o := range order {
		if o.focusable == current {
			return order[(i+dir+len(order))%len(order)].focusable
		}
	}
	// current is no longer part of the chain; behave as a fresh traversal
	return edge(dir > 0)
}

// focusOrder collects the focusable widgets in visible-tree (registration)
// order, skipping disabled ones. Under a right-to-left layout the result is
// reordered to follow the visual arrangement (rows top-to-bottom, each row
// right-to-left); under the default left-to-right layout the tree order is
// returned unchanged.
func (f *FocusManager) focusOrder() []focusEntry {
	var order []focusEntry
	driver.WalkVisibleObjectTree(f.content, func(obj core.CanvasObject, pos geo.Position, _ geo.Position, _ geo.Size) bool {
		if w, ok := obj.(core.Disableable); ok && w.Disabled() {
			return false // disabled widget cannot receive focus
		}
		if focus, ok := obj.(event.Focusable); ok {
			// The walker's trailing size is the clip rectangle, not the widget;
			// use the object's own size for visual row grouping.
			order = append(order, focusEntry{focusable: focus, pos: pos, size: obj.Size()})
		}
		return false
	}, nil)

	if lang.IsRTLDirection() {
		order = reorderRTLVisual(order)
	}
	return order
}

// reorderRTLVisual re-sequences focusables into visual right-to-left order:
// entries are grouped into rows by vertical-span overlap (preserving their
// first-seen order for stable ties), the rows are ordered top-to-bottom, and
// within each row the entries are ordered right-to-left. A column of widgets
// (no two sharing a row) is therefore left untouched, while a horizontal
// arrangement — a toolbar, segmented button or navigation bar — is reversed.
func reorderRTLVisual(in []focusEntry) []focusEntry {
	if len(in) < 2 {
		return in
	}

	type row struct {
		top, bottom float32
		items       []focusEntry
	}
	var rows []*row
	for _, e := range in {
		top, bottom := e.pos.Y, e.pos.Y+e.size.Height
		var placed *row
		for _, r := range rows {
			if top < r.bottom && bottom > r.top { // vertical spans overlap → same row
				placed = r
				break
			}
		}
		if placed == nil {
			rows = append(rows, &row{top: top, bottom: bottom, items: []focusEntry{e}})
			continue
		}
		placed.items = append(placed.items, e)
		placed.top = geo.Min(placed.top, top)
		placed.bottom = geo.Max(placed.bottom, bottom)
	}

	sort.SliceStable(rows, func(i, j int) bool { return rows[i].top < rows[j].top })
	out := make([]focusEntry, 0, len(in))
	for _, r := range rows {
		items := r.items
		sort.SliceStable(items, func(i, j int) bool { return items[i].pos.X > items[j].pos.X })
		out = append(out, items...)
	}
	return out
}
