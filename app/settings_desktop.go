//go:build !android && !ios && !mobile && !wasm && !tamago && !noos && !tinygo

package app

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/belluikit/bellui"
	"github.com/belluikit/fsnotify"
)

func watchFileAddTarget(watcher *fsnotify.Watcher, path string) {
	dir := filepath.Dir(path)
	ensureDirExists(dir)

	// An already-closed watcher during teardown (stopWatching racing a Remove
	// re-arm) is expected, not a real failure — don't log it at error level.
	err := watcher.Add(dir)
	if err != nil && !errors.Is(err, fsnotify.ErrClosed) {
		bellui.LogError("Settings watch error:", err)
	}
}

func ensureDirExists(dir string) {
	if stat, err := os.Stat(dir); err == nil && stat.IsDir() {
		return
	}

	err := os.MkdirAll(dir, 0o700)
	if err != nil {
		bellui.LogError("Unable to create settings storage:", err)
	}
}

func watchFile(path string, callback func()) *fsnotify.Watcher {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		bellui.LogError("Failed to watch settings file:", err)
		return nil
	}

	go func() {
		for event := range watcher.Events {
			if event.Op.Has(fsnotify.Remove) { // if it was deleted then watch again
				watcher.Remove(path) // fsnotify returns false positives, see https://github.com/fsnotify/fsnotify/issues/268

				watchFileAddTarget(watcher, path)
			} else {
				bellui.Do(callback)
			}
		}

		// The Events channel only closes when stopWatching() closes the
		// watcher, which is the sole owner of Close() (#779). The loop ending
		// therefore means teardown already happened — nothing to do here.
	}()

	watchFileAddTarget(watcher, path)
	return watcher
}

func (s *settings) watchSettings() {
	s.watcher = watchFile(s.schema.StoragePath(), s.fileChanged)

	a := bellui.CurrentApp()
	if a != nil && s != nil && a.Settings() == s { // ignore if testing
		watchTheme(s)
	}
}

func (s *settings) stopWatching() {
	if s.watcher == nil {
		return
	}

	// stopWatching owns the single Close() per watcher lifecycle (#779);
	// closing the Events channel ends watchFile's goroutine.
	if err := s.watcher.(*fsnotify.Watcher).Close(); err != nil {
		bellui.LogError("Settings un-watch error:", err)
	}
}
