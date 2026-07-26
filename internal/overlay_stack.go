package internal

import (
	"sync"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/internal/app"
	"github.com/belluikit/bellui/internal/widget"
)

// OverlayStack allows stacking overlays on top of each other.
// Removing an overlay will also remove all overlays above it.
// All methods are safe for concurrent use.
type OverlayStack struct {
	OnChange      func()
	Canvas        core.Canvas
	mu            sync.Mutex
	focusManagers []*app.FocusManager
	overlays      []core.CanvasObject
}

// Add puts an overlay on the stack.
func (s *OverlayStack) Add(overlay core.CanvasObject) {
	if overlay == nil {
		return
	}

	s.mu.Lock()

	s.overlays = append(s.overlays, overlay)

	// TODO this should probably apply to all once #707 is addressed
	if _, ok := overlay.(*widget.OverlayContainer); ok {
		safePos, safeSize := s.Canvas.InteractiveArea()

		overlay.Resize(safeSize)
		overlay.Move(safePos)
	}

	s.focusManagers = append(s.focusManagers, app.NewFocusManager(overlay))

	s.mu.Unlock()

	if s.OnChange != nil {
		s.OnChange()
	}
}

// List returns all overlays on the stack from bottom to top.
func (s *OverlayStack) List() []core.CanvasObject {
	s.mu.Lock()
	defer s.mu.Unlock()
	ret := make([]core.CanvasObject, len(s.overlays))
	copy(ret, s.overlays)
	return ret
}

// ListFocusManagers returns all focus managers on the stack from bottom to top.
func (s *OverlayStack) ListFocusManagers() []*app.FocusManager {
	s.mu.Lock()
	defer s.mu.Unlock()
	ret := make([]*app.FocusManager, len(s.focusManagers))
	copy(ret, s.focusManagers)
	return ret
}

// Remove deletes an overlay and all overlays above it from the stack.
func (s *OverlayStack) Remove(overlay core.CanvasObject) {
	s.mu.Lock()

	overlayIdx := -1
	for i, o := range s.overlays {
		if o == overlay {
			overlayIdx = i
			break
		}
	}
	if overlayIdx == -1 {
		s.mu.Unlock()
		return
	}
	// set removed elements in backing array to nil to release memory references
	for i := overlayIdx; i < len(s.overlays); i++ {
		s.overlays[i] = nil
		s.focusManagers[i] = nil
	}
	s.overlays = s.overlays[:overlayIdx]
	s.focusManagers = s.focusManagers[:overlayIdx]

	s.mu.Unlock()

	if s.OnChange != nil {
		s.OnChange()
	}
}

// Top returns the top-most overlay of the stack.
func (s *OverlayStack) Top() core.CanvasObject {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.overlays) == 0 {
		return nil
	}
	return s.overlays[len(s.overlays)-1]
}

// TopFocusManager returns the app.FocusManager assigned to the top-most
// overlay of the stack that takes part in focus routing. Focus-transparent
// overlays (see widget.OverlayContainer.FocusTransparent) are skipped, so the
// layer beneath them keeps keyboard focus; nil is returned when no overlay
// participates, letting the canvas fall back to its menu/content manager.
func (s *OverlayStack) TopFocusManager() *app.FocusManager {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := len(s.focusManagers) - 1; i >= 0; i-- {
		if pt, ok := s.overlays[i].(interface{ PassesInputThrough() bool }); ok && pt.PassesInputThrough() {
			// Input-pass-through overlays (Toast, Snackbar, tooltips) never
			// own the keyboard either (#569).
			continue
		}
		if oc, ok := s.overlays[i].(*widget.OverlayContainer); ok && oc.FocusTransparent {
			continue
		}
		return s.focusManagers[i]
	}
	return nil
}
