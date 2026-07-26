package analytics

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// recTracker records Track calls synchronously. It is enabled by default so the
// consumer's own filtering is what the mapping/filter tests observe; the real
// consent gate is exercised separately with a live tracker in
// TestAutoCapture_ConsentGate.
type recTracker struct {
	mu    sync.Mutex
	names []string
	props []map[string]any
	off   bool
}

func (r *recTracker) Track(name string, props map[string]any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.off {
		return
	}
	r.names = append(r.names, name)
	r.props = append(r.props, props)
}

func (r *recTracker) Identify(string, map[string]any) {}
func (r *recTracker) Flush() error                    { return nil }
func (r *recTracker) SetEnabled(enabled bool)         { r.off = !enabled }

func (r *recTracker) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.names)
}

func (r *recTracker) last() (string, map[string]any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.names) == 0 {
		return "", nil
	}
	return r.names[len(r.names)-1], r.props[len(r.names)-1]
}

// stubWidget is a minimal accessible CanvasObject with fully controllable
// semantics. It always implements the optional value/description/concealed
// interfaces so the "never captured" fields can be populated with sentinels.
type stubWidget struct {
	role      core.AccessibleRole
	label     string
	state     core.AccessibleState
	value     string
	desc      string
	concealed bool
	visible   bool
}

func newStub(role core.AccessibleRole, label string) *stubWidget {
	return &stubWidget{role: role, label: label, visible: true}
}

func (s *stubWidget) MinSize() geo.Size      { return geo.Size{} }
func (s *stubWidget) Move(geo.Position)      {}
func (s *stubWidget) Position() geo.Position { return geo.Position{} }
func (s *stubWidget) Resize(geo.Size)        {}
func (s *stubWidget) Size() geo.Size         { return geo.Size{} }
func (s *stubWidget) Hide()                  { s.visible = false }
func (s *stubWidget) Visible() bool          { return s.visible }
func (s *stubWidget) Show()                  { s.visible = true }
func (s *stubWidget) Refresh()               {}

func (s *stubWidget) AccessibleRole() core.AccessibleRole      { return s.role }
func (s *stubWidget) AccessibleLabel() string                  { return s.label }
func (s *stubWidget) AccessibleState() core.AccessibleState    { return s.state }
func (s *stubWidget) AccessibleValue() string                  { return s.value }
func (s *stubWidget) AccessibleRange() (min, max, cur float64) { return 0, 0, 0 }
func (s *stubWidget) AccessibleDescription() string            { return s.desc }
func (s *stubWidget) AccessibleConcealed() bool                { return s.concealed }

func fire(obj core.CanvasObject, viaKeyboard bool) {
	core.NotifyActivation(obj, viaKeyboard)
}

func TestAutoCapture_RoleToVerb(t *testing.T) {
	test.NewTempApp(t)

	cases := []struct {
		role core.AccessibleRole
		verb string
		ok   bool
	}{
		{core.RoleButton, core.ActionPress, true},
		{core.RoleLink, core.ActionPress, true},
		{core.RoleMenuItem, core.ActionPress, true},
		{core.RoleCheckbox, core.ActionToggle, true},
		{core.RoleSwitch, core.ActionToggle, true},
		{core.RoleListItem, core.ActionSelect, true},
		{core.RoleTreeItem, core.ActionSelect, true},
		{core.RoleTab, core.ActionSelect, true},
		{core.RoleTableCell, core.ActionSelect, true},
		// Roles with no meaningful activation are not captured.
		{core.RoleLabel, "", false},
		{core.RoleImage, "", false},
		{core.RoleTextBox, "", false},
		{core.RoleProgressBar, "", false},
	}
	for _, c := range cases {
		rec := &recTracker{}
		EnableAutoCapture(rec)
		t.Cleanup(DisableAutoCapture)

		fire(newStub(c.role, "X"), false)

		if !c.ok {
			assert.Zero(t, rec.count(), "role %v should not be captured", c.role)
			DisableAutoCapture()
			continue
		}
		require.Equal(t, 1, rec.count(), "role %v should be captured", c.role)
		name, props := rec.last()
		assert.Equal(t, "ui.activate", name)
		assert.Equal(t, c.verb, props["verb"], "verb for role %v", c.role)
		DisableAutoCapture()
	}
}

func TestAutoCapture_PropsRoleLabelState(t *testing.T) {
	test.NewTempApp(t)
	rec := &recTracker{}
	EnableAutoCapture(rec)
	t.Cleanup(DisableAutoCapture)

	s := newStub(core.RoleCheckbox, "Accept terms")
	s.state = core.StateChecked | core.StateDisabled
	fire(s, false)

	require.Equal(t, 1, rec.count())
	_, props := rec.last()
	assert.Equal(t, "checkbox", props["role"])
	assert.Equal(t, "toggle", props["verb"])
	assert.Equal(t, "Accept terms", props["label"])
	assert.ElementsMatch(t, []string{"checked", "disabled"}, props["state"])
}

func TestAutoCapture_NeverValueOrDescription(t *testing.T) {
	test.NewTempApp(t)
	rec := &recTracker{}
	EnableAutoCapture(rec)
	t.Cleanup(DisableAutoCapture)

	s := newStub(core.RoleButton, "Save")
	s.value = "SECRET_VALUE"
	s.desc = "SECRET_DESCRIPTION"
	fire(s, false)

	require.Equal(t, 1, rec.count())
	_, props := rec.last()
	// The privacy line: value and description are never emitted, under any key,
	// and their contents never appear anywhere in the props.
	assert.NotContains(t, props, "value")
	assert.NotContains(t, props, "description")
	assert.NotContains(t, props, "desc")
	for k, v := range props {
		if sv, ok := v.(string); ok {
			assert.NotContains(t, sv, "SECRET", "key %q leaked a forbidden field", k)
		}
	}
}

func TestAutoCapture_ConcealedExcluded(t *testing.T) {
	test.NewTempApp(t)
	rec := &recTracker{}
	EnableAutoCapture(rec)
	t.Cleanup(DisableAutoCapture)

	// A concealed target is excluded even though its role maps to a verb — the
	// concealed check runs regardless of role.
	s := newStub(core.RoleButton, "Reveal")
	s.concealed = true
	fire(s, false)
	assert.Zero(t, rec.count(), "concealed target must be excluded")

	// Sanity: the same widget not concealed is captured.
	s.concealed = false
	fire(s, false)
	assert.Equal(t, 1, rec.count())
}

func TestAutoCapture_KeyboardFlag(t *testing.T) {
	test.NewTempApp(t)
	rec := &recTracker{}
	EnableAutoCapture(rec)
	t.Cleanup(DisableAutoCapture)

	fire(newStub(core.RoleButton, "A"), true)
	_, props := rec.last()
	assert.Equal(t, "keyboard", props["via"])

	fire(newStub(core.RoleButton, "B"), false)
	_, props = rec.last()
	assert.NotContains(t, props, "via", "pointer taps carry no via marker")
}

func TestAutoCapture_Sampling(t *testing.T) {
	test.NewTempApp(t)
	rec := &recTracker{}
	EnableAutoCapture(rec, WithSampling(3))
	t.Cleanup(DisableAutoCapture)

	// 1-in-3 of eligible activations: capture the 1st, 4th, 7th, 9th eligible.
	for i := 0; i < 9; i++ {
		fire(newStub(core.RoleButton, "X"), false)
	}
	assert.Equal(t, 3, rec.count(), "9 eligible activations at 1-in-3 => 3 captured")
}

func TestAutoCapture_SamplingCountsOnlyEligible(t *testing.T) {
	test.NewTempApp(t)
	rec := &recTracker{}
	EnableAutoCapture(rec, WithSampling(2))
	t.Cleanup(DisableAutoCapture)

	// Interleave non-capturable activations (labels): they must not advance the
	// sampling counter, so the eligible ones alternate deterministically.
	fire(newStub(core.RoleButton, "1"), false) // eligible #1 -> captured
	fire(newStub(core.RoleLabel, "noise"), false)
	fire(newStub(core.RoleButton, "2"), false) // eligible #2 -> skipped
	fire(newStub(core.RoleLabel, "noise"), false)
	fire(newStub(core.RoleButton, "3"), false) // eligible #3 -> captured
	assert.Equal(t, 2, rec.count())
}

func TestAutoCapture_NoCaptureSubtree(t *testing.T) {
	test.NewTempApp(t)
	rec := &recTracker{}
	EnableAutoCapture(rec)
	t.Cleanup(DisableAutoCapture)

	inside := newStub(core.RoleButton, "inside")
	outside := newStub(core.RoleButton, "outside")

	// inside is wrapped by NoCapture; outside sits beside it. Both live in the
	// same window so CanvasForObject resolves them.
	test.NewTempWindow(t, container.NewWithoutLayout(NoCapture(inside), outside))

	fire(inside, false)
	assert.Zero(t, rec.count(), "activation inside NoCapture must be skipped")

	fire(outside, false)
	assert.Equal(t, 1, rec.count(), "activation outside NoCapture is captured")
}

func TestAutoCapture_ConsentGate(t *testing.T) {
	test.NewTempApp(t)
	sink := newFakeSink()
	tr := New(sink, WithStore(&memStore{}), WithBatchSize(1))
	t.Cleanup(func() { _ = Close(tr) })

	EnableAutoCapture(tr)
	t.Cleanup(DisableAutoCapture)

	// Consent withheld (tracker starts disabled): auto-capture captures nothing.
	fire(newStub(core.RoleButton, "X"), false)
	require.NoError(t, tr.Flush())
	assert.Zero(t, sink.callCount(), "disabled tracker: nothing sent")

	// Grant consent, then the same activation flows through the queue and sink.
	tr.SetEnabled(true)
	fire(newStub(core.RoleButton, "Y"), false)
	require.NoError(t, tr.Flush())

	events := sink.allEvents()
	require.NotEmpty(t, events)
	var found bool
	for _, e := range events {
		if e.Name == "ui.activate" {
			found = true
			assert.Equal(t, "press", e.Props["verb"])
		}
	}
	assert.True(t, found, "ui.activate delivered through the consent-gated queue")
}

func TestAutoCapture_ImportedButNotEnabledIsInert(t *testing.T) {
	// Importing analytics does not install a consumer; the seam stays nil.
	DisableAutoCapture()
	assert.Nil(t, core.OnActivation)
}
