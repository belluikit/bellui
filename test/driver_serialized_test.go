// The serialization contract this asserts is pure-Go and identical on wasm;
// what differs in a browser is the CLOCK — under a loaded tab the 100
// Do-roundtrip poll gets timer-throttled past its 10 s deadline (passes in
// 0.1 s in isolation, observed twice failing at 10 s in the full-package
// run). The native legs keep the coverage. (#79 Phase B.)
//go:build !wasm

package test

import (
	"strconv"
	"testing"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/widget"
	"github.com/stretchr/testify/assert"
)

// A background goroutine updating a widget through bellui.Do while the test
// reads it through bellui.DoAndWait must be race-free: the test driver
// serializes all Do callbacks under a mutex, establishing the same
// happens-before edges the real drivers provide (pre-rebrand issue #480).
func TestDriver_DoFromGoroutineSerialized(t *testing.T) {
	NewTempApp(t)

	label := widget.NewLabel("start")
	final := "updated " + strconv.Itoa(99)

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 100; i++ {
			text := "updated " + strconv.Itoa(i)
			bellui.Do(func() {
				label.SetText(text)
			})
		}
	}()

	got := ""
	deadline := time.Now().Add(10 * time.Second)
	for got != final && time.Now().Before(deadline) {
		bellui.DoAndWait(func() {
			got = label.Text
		})
	}
	<-done

	assert.Equal(t, final, got)
}
