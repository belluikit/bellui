package dev

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Watch monitors .go files in the given directory (recursively) and calls
// the rebuild function when changes are detected. Useful for development:
//
//	// In a separate goroutine or main:
//	dev.Watch(".", func() {
//	    // Rebuild and restart — or just log
//	    log.Println("Source changed, rebuild needed")
//	})
//
// For a full auto-restart workflow, use [WatchAndRun] instead.
func Watch(dir string, onChange func()) {
	go watchLoop(dir, onChange)
}

// WatchAndRun watches .go files and automatically rebuilds and restarts
// the application. This is a development-only convenience — do not use
// in production.
//
// It builds the package at pkgPath, runs the resulting binary, and
// restarts it when source files change.
//
//	// Typically called from a dev helper:
//	// go run ./cmd/dev-server
//	func main() {
//	    dev.WatchAndRun(".", ".")
//	}
func WatchAndRun(watchDir, pkgPath string) {
	var (
		mu   sync.Mutex
		proc *exec.Cmd
	)

	buildAndRun := func() {
		mu.Lock()
		defer mu.Unlock()

		// Kill previous process.
		if proc != nil && proc.Process != nil {
			proc.Process.Kill()
			proc.Wait()
		}

		// Build.
		tmpBin := filepath.Join(os.TempDir(), "bellui-dev-run")
		build := exec.Command("go", "build", "-o", tmpBin, pkgPath)
		build.Stdout = os.Stdout
		build.Stderr = os.Stderr
		if err := build.Run(); err != nil {
			log.Printf("[dev] Build failed: %v", err)
			return
		}

		// Run.
		proc = exec.Command(tmpBin)
		proc.Stdout = os.Stdout
		proc.Stderr = os.Stderr
		if err := proc.Start(); err != nil {
			log.Printf("[dev] Run failed: %v", err)
			return
		}
		log.Printf("[dev] Started PID %d", proc.Process.Pid)
	}

	// Initial build.
	buildAndRun()

	// Watch for changes.
	Watch(watchDir, func() {
		log.Println("[dev] Change detected, rebuilding...")
		buildAndRun()
	})

	// Block forever.
	select {}
}

func watchLoop(dir string, onChange func()) {
	modTimes := make(map[string]time.Time)
	scanFiles(dir, modTimes)

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		changed := false
		current := make(map[string]time.Time)
		scanFiles(dir, current)

		for path, t := range current {
			if prev, ok := modTimes[path]; !ok || !t.Equal(prev) {
				changed = true
				log.Printf("[dev] Changed: %s", path)
				break
			}
		}
		if !changed {
			for path := range modTimes {
				if _, ok := current[path]; !ok {
					changed = true
					log.Printf("[dev] Removed: %s", path)
					break
				}
			}
		}

		if changed {
			modTimes = current
			onChange()
		}
	}
}

func scanFiles(dir string, out map[string]time.Time) {
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			base := info.Name()
			if base == ".git" || base == "vendor" || base == "node_modules" || base == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".go") {
			out[path] = info.ModTime()
		}
		return nil
	})
}
