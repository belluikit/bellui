package async

import (
	"runtime"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/internal/thread"
)

var mainRoutineID uint64

func init() {
	mainRoutineID = thread.GoroutineID()
}

func TestGoroutineID(t *testing.T) {
	assert.Equal(t, uint64(1), mainRoutineID)

	var childID1, childID2 uint64
	testID1 := thread.GoroutineID()
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		childID1 = thread.GoroutineID()
		wg.Done()
	}()
	go func() {
		childID2 = thread.GoroutineID()
		wg.Done()
	}()
	wg.Wait()
	testID2 := thread.GoroutineID()

	assert.Equal(t, testID1, testID2)
	assert.Greater(t, childID1, uint64(0))
	assert.NotEqual(t, testID1, childID1)
	assert.Greater(t, childID2, uint64(0))
	assert.NotEqual(t, childID1, childID2)
}

func TestCallerOutsideBellui(t *testing.T) {
	const cache = "/pkg/mod/github.com/belluikit/bellui@v1.3.0/"
	frame := func(fn, file string) runtime.Frame { return runtime.Frame{Function: fn, File: file} }
	ensure := frame("github.com/belluikit/bellui/internal/async.EnsureMain", cache+"internal/async/goroutine.go")
	refresh := frame("github.com/belluikit/bellui/widget.(*Label).SetText", cache+"widget/label.go")
	app := frame("github.com/belluikit/player/registry.(*AudioPlayer).applyStatus", "/src/player/registry/audio_player.go")
	// A file called runtime.go is ordinary code.
	appRuntime := frame("github.com/belluikit/player/compiled/ctx.Call", "/src/player/compiled/ctx/runtime.go")
	lookalike := frame("github.com/belluikit/bellui-extras.Do", "/src/extras/do.go")
	goexit := frame("runtime.goexit", "/go/src/runtime/asm_arm64.s")

	assert.Equal(t, app, callerOutsideBellui([]runtime.Frame{ensure, refresh, app, goexit}))
	assert.Equal(t, appRuntime, callerOutsideBellui([]runtime.Frame{ensure, appRuntime, goexit}))
	assert.Equal(t, lookalike, callerOutsideBellui([]runtime.Frame{ensure, lookalike, goexit}))
	assert.Equal(t, refresh, callerOutsideBellui([]runtime.Frame{ensure, refresh, goexit}),
		"a stack entirely inside bellui blames its last frame")
}
