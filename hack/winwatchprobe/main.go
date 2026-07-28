// Command winwatchprobe is the live-verify harness for belluikit/bellui#68 —
// intermittent Go runtime fatals (runtime.semawakeup / runtime.semasleep /
// netpoll failed, all errno=6 = ERROR_INVALID_HANDLE) in internal/repository on
// windows/arm64.
//
// The cause was a double CloseHandle in fsnotify's Windows backend: a
// completion packet queued before a watch was torn down still carries that
// watch's address, and the unconditional re-arm at the end of readEvents closed
// its already-closed handle a second time. Windows reissues a closed handle's
// value to the next caller, which in a Go program is most often the runtime's
// own CreateEvent for a new M — so the stray close destroys a handle the
// runtime believes it owns and the process aborts somewhere unrelated. Fixed in
// github.com/belluikit/fsnotify; see that repo's FORK.md.
//
// This probe reports the two numbers that track the fix, neither of which needs
// an instrumented build:
//
//   - Bail-out rate. A watcher Close that hangs is the second, causally linked
//     defect (fsnotify's sendError swallowed Close's handshake token, which the
//     failing CancelIo of a double-closed handle is what triggers). bellui's
//     closeWatcher gives up after 5s and leaks the reader, so a Destroy taking
//     that long is a bail-out. Was 4 per 200 rounds; expected 0.
//
//   - Handle drift. Each bail-out leaks the reader's OS handles, so the process
//     handle count creeps up. This is also what refuted the original theory in
//     #68, which blamed the crash on handle *exhaustion*: the count never grows
//     anywhere near far enough, and ERROR_INVALID_HANDLE on a handle the runtime
//     already owns is a stray close, not a failure to allocate. Expected flat.
//
// Run on a Windows box (the windows/arm64 runner is where this reproduces):
//
//	go run ./hack/winwatchprobe -rounds 200
//
// Exit status is non-zero if any bail-out is seen.
package main

func main() { run() }
