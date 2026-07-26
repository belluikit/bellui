package bellui

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/internal/thread"
)

func TestIsMainGoroutine(t *testing.T) {
	thread.SetMainGoroutine()
	assert.True(t, IsMainGoroutine(), "caller that registered itself is main")

	var fromBackground bool
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		fromBackground = IsMainGoroutine()
	}()
	wg.Wait()
	assert.False(t, fromBackground, "a different goroutine is not main")
}

func TestRunOnMain_InlineWhenOnMain(t *testing.T) {
	thread.SetMainGoroutine()

	ran := false
	onMain := false
	RunOnMain(func() {
		ran = true
		onMain = IsMainGoroutine()
	})
	assert.True(t, ran, "fn ran")
	assert.True(t, onMain, "fn observed itself on the main goroutine")
}

func TestRunOnMain_InlineWithoutDriver(t *testing.T) {
	// From a background goroutine, with no running app/driver available,
	// RunOnMain must fall back to running fn inline rather than panicking
	// on CurrentApp().Driver().
	SetCurrentApp(nil)

	var wg sync.WaitGroup
	wg.Add(1)
	ran := false
	go func() {
		defer wg.Done()
		RunOnMain(func() { ran = true })
	}()
	wg.Wait()
	assert.True(t, ran, "fn ran inline when no driver is available")
}
