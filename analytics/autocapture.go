package analytics

import (
	"strings"
	"sync/atomic"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver"
	"github.com/belluikit/bellui/geo"
)

// activateEvent is the single fixed event name auto-capture emits.
const activateEvent = "ui.activate"

// EnableAutoCapture installs a [core.OnActivation] consumer that emits a
// "ui.activate" event to t for every interactive activation — a tap, or a
// screen-reader press/toggle/select (both route through the same driver Tapped
// dispatch). The event is derived from the PUBLIC accessibility snapshot
// ([core.SnapshotAccessible]): its properties are the widget's semantic role,
// its label, and its boolean state — and nothing else.
//
// Everything inherits phase 1's guarantees: the event goes through t.Track, so
// it rides the same consent gate, disk spool, batching loop and backoff. It is
// impossible to wire auto-capture to a raw sink — it only ever calls
// [Tracker.Track]. With consent withheld (tracker disabled) nothing is captured.
//
// Privacy guardrails (all enforced here, none optional):
//   - Role, label and state ONLY. The mapper never reads Value or Description,
//     so text content, Entry values and free-form descriptions never leave the
//     process.
//   - A target that reports [core.AccessibleConcealed] true (a password Entry)
//     is excluded wholesale — belt-and-suspenders on top of never capturing
//     values.
//   - Any activation inside a [NoCapture] subtree is skipped.
//
// Roles are mapped to a fixed verb vocabulary (press/toggle/select); an
// activation whose role has no meaningful activation (a label, a plain text box)
// is not captured. Use [WithSampling] to capture 1-in-N of the eligible
// activations for a high-frequency UI. Install once at startup, after creating
// the tracker; call [DisableAutoCapture] to remove the consumer.
//
// The consumer runs on the main goroutine (the driver's input dispatch); Track
// is fire-into-queue, so it never blocks input.
func EnableAutoCapture(t Tracker, opts ...CaptureOption) {
	ac := &autoCapture{tracker: t}
	for _, o := range opts {
		o(&ac.cfg)
	}
	core.OnActivation = ac.onActivation
}

// DisableAutoCapture removes any consumer installed by [EnableAutoCapture],
// restoring the single-nil-check cost. Safe to call when none is installed.
func DisableAutoCapture() {
	core.OnActivation = nil
}

// CaptureOption configures [EnableAutoCapture].
type CaptureOption func(*captureConfig)

type captureConfig struct {
	sampleN uint64 // capture 1-in-N eligible activations; 0 or 1 = every one
}

// WithSampling captures 1-in-n of the activations that would otherwise be
// emitted (deterministic counter, not wall-clock): the first eligible
// activation and every n-th after it produce an event, the rest are dropped.
// n <= 1 captures every activation. Sampling is applied after the privacy
// filters, so it is a true 1-in-n of the events that pass them.
func WithSampling(n int) CaptureOption {
	return func(c *captureConfig) {
		if n < 1 {
			n = 1
		}
		c.sampleN = uint64(n)
	}
}

// autoCapture holds the consumer state wired into [core.OnActivation].
type autoCapture struct {
	tracker Tracker
	cfg     captureConfig
	counter uint64 // eligible-activation counter for deterministic sampling
}

func (a *autoCapture) onActivation(obj core.CanvasObject, viaKeyboard bool) {
	if a == nil || a.tracker == nil || obj == nil {
		return
	}

	// Concealed exclusion (password Entry and similar): skip wholesale.
	if c, ok := obj.(core.AccessibleConcealed); ok && c.AccessibleConcealed() {
		return
	}

	// Role → verb. Non-accessible objects and roles with no meaningful
	// activation are not captured.
	node, ok := core.SnapshotAccessible(obj)
	if !ok {
		return
	}
	verb, ok := verbForRole(node.Role)
	if !ok {
		return
	}

	// Subtree opt-out: skip when any ancestor is a NoCapture wrapper.
	if underNoCapture(obj) {
		return
	}

	// Sampling last, so it is 1-in-N of the events that pass every filter.
	if a.cfg.sampleN > 1 {
		if (atomic.AddUint64(&a.counter, 1)-1)%a.cfg.sampleN != 0 {
			return
		}
	}

	// Role, label and state ONLY — never Value, never Description.
	props := map[string]any{
		"role": strings.ToLower(node.Role.String()),
		"verb": verb,
	}
	if node.Label != "" {
		props["label"] = node.Label
	}
	if states := stateNames(node.State); len(states) > 0 {
		props["state"] = states
	}
	if viaKeyboard {
		props["via"] = "keyboard"
	}

	a.tracker.Track(activateEvent, props)
}

// verbForRole maps a semantic role to the fixed activation verb, reusing the
// screen-reader action vocabulary ([core.ActionPress]/Toggle/Select). ok is
// false for roles with no meaningful activation, which are not captured.
func verbForRole(r core.AccessibleRole) (string, bool) {
	switch r {
	case core.RoleButton, core.RoleLink, core.RoleMenuItem:
		return core.ActionPress, true
	case core.RoleCheckbox, core.RoleSwitch:
		return core.ActionToggle, true
	case core.RoleListItem, core.RoleTreeItem, core.RoleTab, core.RoleTableCell:
		return core.ActionSelect, true
	}
	return "", false
}

// stateNames renders the active boolean state flags as stable lowercase names,
// in bit order. Only role-relevant flags are meaningful (see
// [core.AccessibleStateful]); this reports whichever are set.
func stateNames(s core.AccessibleState) []string {
	if s == 0 {
		return nil
	}
	var names []string
	for _, e := range stateOrder {
		if s.Has(e.flag) {
			names = append(names, e.name)
		}
	}
	return names
}

var stateOrder = []struct {
	flag core.AccessibleState
	name string
}{
	{core.StateChecked, "checked"},
	{core.StateExpanded, "expanded"},
	{core.StateSelected, "selected"},
	{core.StateDisabled, "disabled"},
	{core.StateFocused, "focused"},
	{core.StateRequired, "required"},
	{core.StateReadOnly, "readonly"},
	{core.StatePartial, "partial"},
	{core.StateModal, "modal"},
}

// noCaptureLayout is the marker on the NoCapture wrapper container. It also lays
// the single child out to fill the wrapper, so the wrapper is layout-transparent.
type noCaptureLayout struct{}

func (noCaptureLayout) Layout(objs []core.CanvasObject, size geo.Size) {
	for _, o := range objs {
		o.Resize(size)
		o.Move(geo.NewPos(0, 0))
	}
}

func (noCaptureLayout) MinSize(objs []core.CanvasObject) geo.Size {
	var m geo.Size
	for _, o := range objs {
		m = m.Max(o.MinSize())
	}
	return m
}

// NoCapture wraps content in a layout-transparent container that marks its whole
// subtree opt-out: [EnableAutoCapture] skips any activation that occurs within
// it. It does not change hit-testing, focus or appearance — only the capture
// decision (option B: a top-down ancestor walk finds the marker; no
// CanvasObject change). Nest it around a sensitive region (a payment form, a
// login panel) to exclude the region wholesale.
func NoCapture(content core.CanvasObject) core.CanvasObject {
	return container.New(noCaptureLayout{}, content)
}

// isNoCapture reports whether obj is a NoCapture wrapper (a container carrying
// the marker layout).
func isNoCapture(obj core.CanvasObject) bool {
	if c, ok := obj.(*bellui.Container); ok {
		_, ok := c.Layout.(noCaptureLayout)
		return ok
	}
	return false
}

// underNoCapture reports whether obj sits inside a NoCapture subtree, by walking
// the root→obj ancestor chain of obj's canvas. Taps are rare, so the O(tree)
// walk per activation is acceptable. Returns false when the canvas is
// unavailable (obj not attached).
func underNoCapture(obj core.CanvasObject) bool {
	app := bellui.CurrentApp()
	if app == nil {
		return false
	}
	drv := app.Driver()
	if drv == nil {
		return false
	}
	canvas := drv.CanvasForObject(obj)
	if canvas == nil {
		return false
	}
	path, ok := driver.AncestorsOfObject(canvas, obj)
	if !ok {
		return false
	}
	for _, anc := range path {
		if isNoCapture(anc) {
			return true
		}
	}
	return false
}
