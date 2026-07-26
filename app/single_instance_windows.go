//go:build windows

package app

import (
	"hash/fnv"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/Microsoft/go-winio"
	"golang.org/x/sys/windows"

	"github.com/belluikit/bellui"
)

// singleInstanceBind implements the Windows transport with the classic
// named-mutex (detection) + named-pipe (forwarding) pattern.
//
//   - Creating the named mutex is atomic: the process that creates it is the
//     primary; any later process gets ERROR_ALREADY_EXISTS and is a secondary.
//   - The primary serves a named pipe; secondaries forward os.Args[1:] to it.
//
// The mutex lives in the per-session Local\ namespace and the pipe is secured
// with a DACL granting access only to the current user (plus SYSTEM and
// Administrators), so another local user cannot inject arguments.
func singleInstanceBind(id string, deliver func(args []string)) (primary bool, closer func(), err error) {
	base := singleInstanceKey(id)
	mutexName, mErr := windows.UTF16PtrFromString(`Local\` + base)
	if mErr != nil {
		return false, nil, mErr
	}
	pipeName := `\\.\pipe\` + base

	handle, err := windows.CreateMutex(nil, false, mutexName)
	if err != nil && err != windows.ERROR_ALREADY_EXISTS {
		return false, nil, err
	}

	if err == windows.ERROR_ALREADY_EXISTS {
		// A primary already owns the mutex. Forward our arguments and step aside.
		// The primary keeps the mutex name alive, so we drop our handle.
		if handle != 0 {
			windows.CloseHandle(handle)
		}
		if fErr := forwardArgsWindows(pipeName); fErr != nil {
			return false, nil, fErr
		}
		return false, nil, nil
	}

	// We are the primary and now own the mutex handle for the process lifetime.
	l, lErr := winio.ListenPipe(pipeName, &winio.PipeConfig{SecurityDescriptor: currentUserSDDL()})
	if lErr != nil {
		windows.CloseHandle(handle)
		return false, nil, lErr
	}

	go serveWindows(l, deliver)

	closer = func() {
		_ = l.Close()
		windows.CloseHandle(handle)
	}
	return true, closer, nil
}

// serveWindows accepts forwarding connections and delivers their arguments.
func serveWindows(l net.Listener, deliver func(args []string)) {
	for {
		conn, err := l.Accept()
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

// forwardArgsWindows dials the primary's pipe and sends this launch's arguments.
// The primary owns the mutex but may still be creating its pipe (a simultaneous
// cold-launch race), so the dial is retried briefly - winio.DialPipe fails
// immediately with ERROR_FILE_NOT_FOUND when the pipe does not exist yet rather
// than waiting for it, unlike its ERROR_PIPE_BUSY handling.
func forwardArgsWindows(pipeName string) error {
	deadline := time.Now().Add(2 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		dialTimeout := 200 * time.Millisecond
		conn, err := winio.DialPipe(pipeName, &dialTimeout)
		if err != nil {
			lastErr = err
			time.Sleep(20 * time.Millisecond)
			continue
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
		return writeArgs(conn, os.Args[1:])
	}
	bellui.LogError("Failed to reach single-instance primary for argument forwarding", lastErr)
	return lastErr
}

// currentUserSDDL builds a DACL that grants full access only to the current
// user, plus SYSTEM and the Administrators group. If the user SID cannot be
// resolved it falls back to an empty descriptor (winio's default, creator +
// admins), which is still per-machine-user safe for the common case.
func currentUserSDDL() string {
	token := windows.GetCurrentProcessToken()
	user, err := token.GetTokenUser()
	if err != nil {
		return ""
	}
	sid := user.User.Sid.String()
	// P = protected (no inheritance); A;;GA = allow, generic all.
	return "D:P(A;;GA;;;SY)(A;;GA;;;BA)(A;;GA;;;" + sid + ")"
}

// singleInstanceKey derives a namespace-safe object name from the ID.
func singleInstanceKey(id string) string {
	h := fnv.New64a()
	_, _ = h.Write([]byte(id))
	return "bellui-si-" + strconv.FormatUint(h.Sum64(), 16)
}
