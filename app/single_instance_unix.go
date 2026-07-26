//go:build (linux || darwin || freebsd || openbsd || netbsd) && !android && !ios && !wasm

package app

import (
	"hash/fnv"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/belluikit/bellui"
)

// singleInstanceBind implements the Unix (Linux/BSD/macOS) transport.
//
// Primary election uses an advisory file lock (flock) on a per-user lock file
// named from the app ID: flock is atomic, so exactly one racing launch wins,
// and the kernel releases the lock automatically when the holder exits or
// crashes - so stale-lock recovery is free (no liveness probing). The Unix
// domain socket beside the lock is used only to forward arguments to the
// elected primary.
//
//   - Lock acquired -> this process is the primary: it removes any stale socket
//     left by a crashed predecessor, binds a fresh one, and serves forwarded
//     arguments to deliver in the background. Returns primary=true.
//   - Lock held by another process -> a primary is running: this launch forwards
//     os.Args[1:] over the socket and returns primary=false.
//
// On macOS Launch Services usually activates the running instance directly
// (delivering odoc/GetURL Apple Events rather than relaunching), so forwarding
// is rarely exercised there, but the detection API behaves identically.
func singleInstanceBind(id string, deliver func(args []string)) (primary bool, closer func(), err error) {
	sockPath := singleInstanceSocketPath(id)
	lockPath := sockPath + ".lock"

	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return false, nil, err
	}

	if flockErr := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); flockErr != nil {
		// Lock is held: a primary is (or is coming) up. Forward and step aside.
		lockFile.Close()
		if fErr := forwardArgs(sockPath); fErr != nil {
			return false, nil, fErr
		}
		return false, nil, nil
	}

	// We hold the lock and are the sole primary. No other process can own the
	// socket, so removing a stale one left by a crashed predecessor is safe.
	_ = os.Remove(sockPath)
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		releaseLock(lockFile)
		return false, nil, err
	}

	// Restrict to the owning user (the runtime dir is normally already 0700, but
	// os.TempDir() is world-writable, so this is the real access gate).
	_ = os.Chmod(sockPath, 0o600)

	go serve(ln, deliver)

	closer = func() {
		_ = ln.Close()
		_ = os.Remove(sockPath)
		releaseLock(lockFile)
	}
	return true, closer, nil
}

func releaseLock(lockFile *os.File) {
	_ = syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)
	_ = lockFile.Close()
}

// serve accepts forwarding connections and delivers their decoded arguments.
func serve(ln net.Listener, deliver func(args []string)) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return // listener closed on Quit
		}
		go func() {
			defer conn.Close()
			args, err := readArgs(conn)
			if err != nil {
				bellui.LogError("Failed to read forwarded single-instance arguments", err)
				return
			}
			deliver(args)
		}()
	}
}

// forwardArgs dials the primary and sends this launch's arguments. The primary
// holds the lock but may still be binding its socket, so the dial is retried
// briefly before giving up.
func forwardArgs(path string) error {
	deadline := time.Now().Add(2 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("unix", path, 500*time.Millisecond)
		if err != nil {
			lastErr = err
			time.Sleep(20 * time.Millisecond)
			continue
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
		return writeArgs(conn, os.Args[1:])
	}
	return lastErr
}

// singleInstanceSocketPath derives a per-user socket path from the app ID.
// $XDG_RUNTIME_DIR is preferred (per-user, 0700, tmpfs); os.TempDir() is the
// fallback. The ID is hashed to keep the filename short and filesystem-safe
// while staying stable across launches. The uid is included so distinct users
// on a shared temp dir never collide.
func singleInstanceSocketPath(id string) string {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir != "" {
		if info, err := os.Stat(dir); err != nil || !info.IsDir() {
			dir = "" // set but unusable (e.g. macOS) -> fall back
		}
	}
	if dir == "" {
		dir = os.TempDir()
	}

	h := fnv.New64a()
	_, _ = h.Write([]byte(id))
	name := "bellui-si-" + strconv.FormatUint(uint64(os.Getuid()), 10) + "-" +
		strconv.FormatUint(h.Sum64(), 16) + ".sock"
	return filepath.Join(dir, name)
}
