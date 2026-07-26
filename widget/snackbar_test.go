package widget_test

import (
	"sync"
	"testing"
	"time"

	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"

	"github.com/stretchr/testify/assert"
)

func TestSnackbar_New(t *testing.T) {
	s := widget.NewSnackbar("Hello", "Undo", nil)
	assert.NotNil(t, s)
	assert.Equal(t, "Hello", s.Text)
	assert.Equal(t, "Undo", s.ActionText)
	assert.Equal(t, 4*time.Second, s.Duration)
}

func TestSnackbar_Show(t *testing.T) {
	app := test.NewTempApp(t)
	w := app.NewWindow("Test")
	defer w.Close()
	w.Resize(geo.NewSize(400, 300))

	s := widget.NewSnackbar("Message", "", nil)
	assert.NotPanics(t, func() { s.Show(w.Canvas()) })
	s.Hide()
}

func TestSnackbar_Hide(t *testing.T) {
	s := widget.NewSnackbar("Test", "", nil)
	// Hide on a snackbar that was never shown should not panic.
	assert.NotPanics(t, func() { s.Hide() })
}

func TestSnackbar_WithAction(t *testing.T) {
	s := widget.NewSnackbar("Error", "Retry", func() {})
	assert.NotEmpty(t, s.ActionText)
	assert.NotNil(t, s.OnAction)
}

// TestSnackbar_ConcurrentHide exercises the mutex that guards the internal
// timer/popUp fields: with the auto-dismiss timer hopping onto the UI goroutine
// via bellui.Do, several Hide calls must not race or double-dismiss (run -race).
func TestSnackbar_ConcurrentHide(t *testing.T) {
	app := test.NewTempApp(t)
	w := app.NewWindow("Test")
	defer w.Close()
	w.Resize(geo.NewSize(400, 300))

	s := widget.NewSnackbar("Message", "Undo", func() {})
	s.Duration = 10 * time.Millisecond
	s.Show(w.Canvas())

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			assert.NotPanics(t, func() { s.Hide() })
		}()
	}
	wg.Wait()
}
