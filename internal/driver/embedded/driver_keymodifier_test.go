package embedded_test

import (
	"image"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/driver/embedded"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	intNoos "github.com/belluikit/bellui/internal/driver/embedded"
)

type noosFocusable struct {
	canvas.Rectangle
	onFocus func()
}

func (f *noosFocusable) FocusGained() {
	if f.onFocus != nil {
		f.onFocus()
	}
}
func (f *noosFocusable) FocusLost()               {}
func (f *noosFocusable) TypedRune(rune)           {}
func (f *noosFocusable) TypedKey(*event.KeyEvent) {}

// TestNoOSDriver_TypedKeyCarriesModifier pins pre-rebrand tracker #1042 on the embedded driver:
// the Modifiers reported on an embedded.KeyEvent reach the delivered
// event.KeyEvent.
func TestNoOSDriver_TypedKeyCarriesModifier(t *testing.T) {
	queue := make(chan embedded.Event, 2)
	size := func() geo.Size { return geo.NewSquareSize(100) }
	d := intNoos.NewNoOSDriver(func(image.Image) {}, func(f func()) { f() }, queue, size)
	w := d.CreateWindow("Test")

	var got []*event.KeyEvent
	w.Canvas().SetOnTypedKey(func(k *event.KeyEvent) {
		got = append(got, k)
		if len(got) == 2 {
			close(queue)
		}
	})
	queue <- &embedded.KeyEvent{Name: event.KeyF11}
	queue <- &embedded.KeyEvent{Name: event.KeyF11, Modifiers: event.KeyModifierShift}
	d.Run()

	require.Len(t, got, 2)
	assert.Equal(t, event.KeyModifier(0), got[0].Modifier)
	assert.Equal(t, event.KeyModifierShift, got[1].Modifier)
}

// Shift+Tab moves focus backwards now that embedded key events carry modifiers.
func TestNoOSDriver_ShiftTabFocusesPrevious(t *testing.T) {
	queue := make(chan embedded.Event, 1)
	size := func() geo.Size { return geo.NewSquareSize(100) }
	d := intNoos.NewNoOSDriver(func(image.Image) {}, func(f func()) { f() }, queue, size)
	w := d.CreateWindow("Test")

	first := &noosFocusable{}
	second := &noosFocusable{}
	w.SetContent(container.NewWithoutLayout(first, second))

	var once sync.Once
	stop := func() { once.Do(func() { close(queue) }) }
	first.onFocus = stop
	go func() { // safety net so a regression fails instead of hanging
		time.Sleep(5 * time.Second)
		stop()
	}()

	w.Canvas().Focus(second)
	queue <- &embedded.KeyEvent{Name: event.KeyTab, Modifiers: event.KeyModifierShift}
	d.Run()

	assert.Equal(t, first, w.Canvas().Focused())
}

type noosTabKeeper struct{ noosFocusable }

func (*noosTabKeeper) AcceptsTab() bool { return true }

// Ctrl+Tab leaves a widget that keeps Tab on the embedded driver too (#340).
func TestNoOSDriver_CtrlTabLeavesWidgetThatKeepsTab(t *testing.T) {
	queue := make(chan embedded.Event, 1)
	size := func() geo.Size { return geo.NewSquareSize(100) }
	d := intNoos.NewNoOSDriver(func(image.Image) {}, func(f func()) { f() }, queue, size)
	w := d.CreateWindow("Test")

	field := &noosTabKeeper{}
	after := &noosFocusable{}
	w.SetContent(container.NewWithoutLayout(field, after))

	var once sync.Once
	stop := func() { once.Do(func() { close(queue) }) }
	after.onFocus = stop
	go func() { // safety net so a regression fails instead of hanging
		time.Sleep(5 * time.Second)
		stop()
	}()

	w.Canvas().Focus(field)
	queue <- &embedded.KeyEvent{Name: event.KeyTab, Modifiers: event.KeyModifierControl}
	d.Run()

	assert.Equal(t, after, w.Canvas().Focused())
}

// Escape then Tab leaves a widget that keeps Tab on the embedded driver (#357).
func TestNoOSDriver_EscapeThenTabLeavesWidgetThatKeepsTab(t *testing.T) {
	queue := make(chan embedded.Event, 2)
	size := func() geo.Size { return geo.NewSquareSize(100) }
	d := intNoos.NewNoOSDriver(func(image.Image) {}, func(f func()) { f() }, queue, size)
	w := d.CreateWindow("Test")

	field := &noosTabKeeper{}
	after := &noosFocusable{}
	w.SetContent(container.NewWithoutLayout(field, after))

	var once sync.Once
	stop := func() { once.Do(func() { close(queue) }) }
	after.onFocus = stop
	go func() { // safety net so a regression fails instead of hanging
		time.Sleep(5 * time.Second)
		stop()
	}()

	w.Canvas().Focus(field)
	queue <- &embedded.KeyEvent{Name: event.KeyEscape}
	queue <- &embedded.KeyEvent{Name: event.KeyTab}
	d.Run()

	assert.Equal(t, after, w.Canvas().Focused())
}

type noosTabCounter struct {
	noosTabKeeper
	tabs int
}

func (k *noosTabCounter) TypedKey(ev *event.KeyEvent) {
	if ev.Name == event.KeyTab {
		k.tabs++
	}
}

// Text typed after Escape cancels the way out, also when it arrives as a
// CharacterEvent with no key event at all.
func TestNoOSDriver_CharacterAfterEscapeKeepsTab(t *testing.T) {
	queue := make(chan embedded.Event, 5)
	size := func() geo.Size { return geo.NewSquareSize(100) }
	d := intNoos.NewNoOSDriver(func(image.Image) {}, func(f func()) { f() }, queue, size)
	w := d.CreateWindow("Test")

	field := &noosTabCounter{}
	after := &noosFocusable{}
	w.SetContent(container.NewWithoutLayout(field, after))

	var once sync.Once
	stop := func() { once.Do(func() { close(queue) }) }
	after.onFocus = stop
	go func() { // safety net so a regression fails instead of hanging
		time.Sleep(5 * time.Second)
		stop()
	}()

	w.Canvas().Focus(field)
	queue <- &embedded.KeyEvent{Name: event.KeyEscape}
	queue <- &embedded.CharacterEvent{Rune: 'a'}
	queue <- &embedded.KeyEvent{Name: event.KeyTab} // typed into the field
	queue <- &embedded.KeyEvent{Name: event.KeyEscape}
	queue <- &embedded.KeyEvent{Name: event.KeyTab} // leaves, and ends the run
	d.Run()

	assert.Equal(t, after, w.Canvas().Focused())
	assert.Equal(t, 1, field.tabs, "the Tab after the typed character stayed in the field")
}
