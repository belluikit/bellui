//go:build !wasm

package app

import (
	"io"
	"os"
	"path/filepath"
	"strings"
)

func (p *preferences) storageWriter() (writeSyncCloser, error) {
	return p.storageWriterForPath(p.storagePath())
}

func (p *preferences) storageReader() (io.ReadCloser, error) {
	r, err := p.storageReaderForPath(p.storagePath())
	if err == errEmptyPreferencesStore {
		// Read-both migration window: no PXF store yet, so fall back to the legacy
		// JSON store if one exists (loadFromStorage sniffs the format on read).
		if legacy := p.legacyStoragePath(); legacy != "" && legacy != p.storagePath() {
			if lr, lerr := p.storageReaderForPath(legacy); lerr == nil {
				return lr, nil
			}
		}
	}
	return r, err
}

// legacyStoragePath is the pre-migration JSON preferences path (preferences.json)
// alongside the current PXF store, read during the migration window and removed by
// cleanupLegacyStore once a PXF store has been written.
func (p *preferences) legacyStoragePath() string {
	path := p.storagePath()
	return strings.TrimSuffix(path, ".pxf") + ".json"
}

// cleanupLegacyStore removes the superseded legacy JSON preferences file after a
// successful PXF save, completing the migration for this store.
func (p *preferences) cleanupLegacyStore() {
	if legacy := p.legacyStoragePath(); legacy != "" && legacy != p.storagePath() {
		_ = os.Remove(legacy)
	}
}

// atomicFileWriter writes to a temporary file and renames it over the target
// on Close. This prevents corruption if the process crashes mid-write.
type atomicFileWriter struct {
	target   string
	tempFile *os.File
}

func (w *atomicFileWriter) Write(p []byte) (int, error) {
	return w.tempFile.Write(p)
}

func (w *atomicFileWriter) Sync() error {
	return w.tempFile.Sync()
}

func (w *atomicFileWriter) Close() error {
	if err := w.tempFile.Close(); err != nil {
		os.Remove(w.tempFile.Name())
		return err
	}
	// Atomic rename: on the same filesystem this is safe against crashes.
	// atomicRename retries on Windows, where the rename can fail transiently
	// while another handle (a directory watcher, antivirus) has the target open.
	if err := atomicRename(w.tempFile.Name(), w.target); err != nil {
		os.Remove(w.tempFile.Name())
		return err
	}
	return nil
}

// abort discards the pending write without committing: it closes and removes the
// temp file, leaving the existing target untouched. saveToStorage calls this
// instead of Close when the encode or sync failed, so a partial or empty file is
// never renamed over good data.
func (w *atomicFileWriter) abort() error {
	closeErr := w.tempFile.Close()
	rmErr := os.Remove(w.tempFile.Name())
	if closeErr != nil {
		return closeErr
	}
	return rmErr
}

func (p *preferences) storageWriterForPath(path string) (writeSyncCloser, error) {
	err := os.MkdirAll(filepath.Dir(path), 0o700)
	if err != nil {
		return nil, err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".preferences-*.tmp")
	if err != nil {
		return nil, err
	}
	return &atomicFileWriter{target: path, tempFile: tmp}, nil
}

func (p *preferences) storageReaderForPath(path string) (io.ReadCloser, error) {
	file, err := os.Open(path) // #nosec
	if err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				return nil, err
			}
			return nil, errEmptyPreferencesStore
		}
		return nil, err
	}
	return file, nil
}

// the following are only used in tests to save preferences to a tmp file

func (p *preferences) saveToFile(path string) error {
	file, err := p.storageWriterForPath(path)
	if err != nil {
		return err
	}
	return p.saveToStorage(file)
}

func (p *preferences) loadFromFile(path string) error {
	file, err := p.storageReaderForPath(path)
	if err != nil {
		return err
	}
	return p.loadFromStorage(file)
}
