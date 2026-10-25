package mobile

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Windows opened and closed on one goroutine while another reads the window
// list race on nothing: a Refresh on the main goroutine can look up the list
// (AllWindows) while a window closes elsewhere. Under -race this failed when
// the list had no lock.
func TestDriver_WindowListIsSafeAcrossGoroutines(t *testing.T) {
	drv := &driver{}
	keep := drv.CreateWindow("kept").(*window)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range 50 {
			drv.CreateWindow("transient").Close()
		}
	}()
	for range 200 {
		for _, w := range drv.AllWindows() {
			assert.NotNil(t, w)
		}
		_ = drv.CanvasForObject(nil)
		_ = drv.currentWindow()
	}
	wg.Wait()
	assert.Equal(t, keep, drv.AllWindows()[0])
	assert.Len(t, drv.AllWindows(), 1)
}
