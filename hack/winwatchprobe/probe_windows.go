//go:build windows

package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/belluikit/bellui/internal/repository"
	"github.com/belluikit/bellui/storage"
)

// bailOutWait is closeWatcher's bounded wait in internal/repository. A Destroy
// that takes at least this long gave up on fsnotify's Close and leaked the
// watcher's reader goroutine and OS handles.
const bailOutWait = 5 * time.Second

// x/sys/windows has no GetProcessHandleCount binding.
var procGetProcessHandleCount = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetProcessHandleCount")

func handleCount() uint32 {
	var n uint32
	r, _, _ := procGetProcessHandleCount.Call(uintptr(windows.CurrentProcess()), uintptr(unsafe.Pointer(&n)))
	if r == 0 {
		return 0
	}
	return n
}

func run() {
	var (
		rounds     = flag.Int("rounds", 200, "watcher rounds; each builds a repository, churns subscriptions, then destroys it")
		perRound   = flag.Int("per-round", 40, "subscribe/cancel cycles per goroutine per round")
		goroutines = flag.Int("goroutines", 8, "concurrent subscribe/cancel goroutines (matches TestFileRepository_WatchURI_ConcurrentControlOps)")
	)
	flag.Parse()

	dir, err := os.MkdirTemp("", "winwatchprobe")
	if err != nil {
		fmt.Println("mkdtemp:", err)
		os.Exit(2)
	}
	defer os.RemoveAll(dir)

	fmt.Printf("winwatchprobe: rounds=%d goroutines=%d per-round=%d\n", *rounds, *goroutines, *perRound)

	// Steady write pressure, so a completion packet is almost always in flight
	// when a watch is torn down — that overlap is the whole bug.
	var stop atomic.Bool
	var writer sync.WaitGroup
	writer.Add(1)
	go func() {
		defer writer.Done()
		for i := 0; !stop.Load(); i++ {
			_ = os.WriteFile(filepath.Join(dir, "churn.txt"), []byte{byte(i)}, 0o644)
			time.Sleep(time.Millisecond)
		}
	}()

	uri := storage.NewFileURI(dir)
	start := handleCount()
	peak, bailOuts := start, 0

	for round := 0; round < *rounds; round++ {
		repo := repository.NewFileRepository()

		var wg sync.WaitGroup
		for g := 0; g < *goroutines; g++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < *perRound; j++ {
					cancel, err := repo.WatchURI(uri, func() {})
					if err != nil {
						continue
					}
					cancel()
				}
			}()
		}
		wg.Wait()

		destroyed := time.Now()
		repo.Destroy("file")
		if took := time.Since(destroyed); took >= bailOutWait {
			bailOuts++
			fmt.Printf("round %3d: BAIL-OUT — Destroy took %s; closeWatcher leaked the reader\n", round, took.Round(time.Millisecond))
		}

		if n := handleCount(); n > peak {
			peak = n
		}
		if round%20 == 0 || round == *rounds-1 {
			fmt.Printf("round %3d: handles=%d peak=%d bail-outs=%d\n", round, handleCount(), peak, bailOuts)
		}
	}

	stop.Store(true)
	writer.Wait()

	end := handleCount()
	fmt.Printf("\nhandles: start=%d end=%d peak=%d (drift %+d)\n", start, end, peak, int64(end)-int64(start))
	fmt.Printf("bail-outs: %d in %d rounds\n", bailOuts, *rounds)
	if bailOuts > 0 {
		fmt.Println("RESULT: FAIL — fsnotify's Close still hangs; see belluikit/fsnotify FORK.md")
		os.Exit(1)
	}
	fmt.Println("RESULT: ok — no watcher Close hung, no reader leaked.")
}
