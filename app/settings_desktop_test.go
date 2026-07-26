//go:build !android && !ios && !mobile

package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
)

func TestEnsureDir(t *testing.T) {
	tmpDir := testPath("belluitest")

	ensureDirExists(tmpDir)
	if st, err := os.Stat(tmpDir); err != nil || !st.IsDir() {
		t.Error("Could not ensure directory exists")
	}

	os.Remove(tmpDir)
}

func TestWatchSettings(t *testing.T) {
	settings := &settings{}
	listener := make(chan bellui.Settings, 1)
	settings.AddListener(func(s bellui.Settings) {
		select {
		case listener <- s:
		default:
		}
	})

	settings.fileChanged() // simulate the settings file changing

	select {
	case <-listener:
	case <-time.After(100 * time.Millisecond):
		t.Error("Settings listener was not called")
	}
}

func TestWatchFile(t *testing.T) {
	path := testPath("bellui-temp-watch.txt")
	f, _ := os.Create(path)
	f.Close()
	defer os.Remove(path)

	called := make(chan any, 1)
	watcher := watchFile(path, func() {
		// Non-blocking: fsnotify can deliver more events than the buffer
		// holds, and the test driver serializes bellui.Do callbacks under a
		// mutex (#480) — a blocked send here would hold that mutex and
		// starve every later bellui.Do in the package (#492).
		select {
		case called <- true:
		default:
		}
	})
	if watcher == nil {
		assert.Fail(t, "Could not start watcher")
		return
	}
	// Close the watcher so its callback cannot fire into later tests that
	// reuse the same path (#492).
	defer watcher.Close()

	file, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	file.WriteString(" ")
	file.Close()

	select {
	case <-called:
	case <-time.After(100 * time.Millisecond):
		t.Error("File watcher callback was not called")
	}
}

func TestFileWatcher_FileDeleted(t *testing.T) {
	path := testPath("bellui-temp-watch.txt")
	f, _ := os.Create(path)
	f.Close()
	defer os.Remove(path)

	called := make(chan any, 1)
	watcher := watchFile(path, func() {
		// Non-blocking for the same reason as in TestWatchFile: the
		// delete→recreate sequence can deliver several events, and a send
		// blocked inside bellui.Do would hold the test driver's Do mutex
		// forever (#492) — watcher.Close cannot unblock a channel send.
		select {
		case called <- true:
		default:
		}
	})
	if watcher == nil {
		assert.Fail(t, "Could not start watcher")
		return
	}

	defer watcher.Close()
	os.Remove(path)
	f, _ = os.Create(path)

	select {
	case <-called:
	case <-time.After(100 * time.Millisecond):
		t.Error("File watcher callback was not called")
	}
	f.Close()
}

func testPath(child string) string {
	// TMPDIR would be more normal but fsnotify cannot watch that on macOS...
	return filepath.Join("testdata", child)
}
