package bellui

import (
	"os"
	"os/exec"
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

// MainGoroutineRegistered is false until a driver registers its main goroutine,
// and true from then on (#1356). Registration is process-wide and has no undo,
// and other tests in this package register, so the false half runs in a child
// process of its own.
func TestMainGoroutineRegistered(t *testing.T) {
	if os.Getenv("BELLUI_TEST_UNREGISTERED") == "1" {
		if MainGoroutineRegistered() {
			os.Exit(3)
		}
		thread.SetMainGoroutine()
		if !MainGoroutineRegistered() {
			os.Exit(4)
		}
		os.Exit(0)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestMainGoroutineRegistered$")
	cmd.Env = append(os.Environ(), "BELLUI_TEST_UNREGISTERED=1")
	out, err := cmd.CombinedOutput()
	if exit, ok := err.(*exec.ExitError); ok {
		switch exit.ExitCode() {
		case 3:
			t.Fatal("registered in a process where no main goroutine was registered")
		case 4:
			t.Fatal("not registered after SetMainGoroutine")
		}
	}
	assert.NoError(t, err, "child process: %s", out)

	thread.SetMainGoroutine()
	assert.True(t, MainGoroutineRegistered(), "registered after SetMainGoroutine")
}
