package webview

// Where the Linux (CEF) backend keeps its browsing profile — the #125
// decision. This file is portable so a cross-platform app configures the
// choice without build tags; only the Linux runtime
// (cefruntime_linux.go) reads it, when CEF initializes on the first Attach.
//
// The default is persistence, at a directory owned by the running app: every
// other backend's engine persists its profile (WebKit's default data store,
// WebView2's user-data folder, android.webkit's app-private storage), so a
// login surviving a restart is the portable expectation, and Linux matching
// it is the least surprising shape. In-memory is the explicit opt-out.

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/belluikit/bellui"
)

// errCEFProfileLocked reports a profile call made after the engine started.
var errCEFProfileLocked = errors.New(
	"webview: CEF is already initialized; the profile location cannot change (set it before the first Attach)",
)

// cefProfile is the pre-Attach profile choice. Main-goroutine state, like
// everything else in this package; locked is set by the Linux runtime the
// moment the choice is handed to CEF, which is once-per-process.
var cefProfile struct {
	dir      string // explicit directory; empty means the app-storage default
	inMemory bool
	locked   bool
}

// SetCEFProfileDir tells the Linux (CEF) backend to persist its browsing
// profile — cookies, localStorage, the HTTP cache — at dir. The directory is
// created by the engine as needed and should be an absolute path; a relative
// one is resolved against the working directory when the engine starts. An
// empty dir restores the default.
//
// The profile persists by default, at "webview-cef" under the running app's
// [bellui.Storage] root, so this call is for apps that want it somewhere
// specific. Call [SetCEFProfileInMemory] to discard it on exit instead.
//
// CEF is initialized once per process, on the first [WebView.Attach], and the
// profile location is fixed from then on: call this before that first Attach,
// on the main goroutine. Afterwards it changes nothing and reports an error.
//
// CEF guards a profile directory with a process-singleton lock. When a second
// app instance reaches its first Attach while another holds the same
// directory, the engine fails to initialize and every web view in that
// instance stays inert, reporting the failure from Attach. An app expecting
// concurrent instances should give each its own directory, keep the profile
// in memory, or use single-instance mode.
//
// Only the Linux backend has a profile of its own to place — everywhere else
// the platform engine manages it — so on other platforms this reports nil and
// changes nothing.
//
// Since: 1.0
func SetCEFProfileDir(dir string) error {
	if cefProfile.locked {
		return errCEFProfileLocked
	}
	cefProfile.dir = dir
	cefProfile.inMemory = false
	return nil
}

// SetCEFProfileInMemory tells the Linux (CEF) backend to keep its browsing
// profile in memory, discarded when the process exits — the explicit opt-out
// from the persistence described on [SetCEFProfileDir]. Web views then behave
// like an incognito window: no cookie, localStorage value or cached response
// survives a restart.
//
// The engine still writes a little installation-specific data (its
// process-singleton lock among it); that goes to a fresh directory under
// [os.TempDir], so concurrent instances never contend and nothing durable is
// left behind.
//
// The same restrictions as [SetCEFProfileDir] apply: call it before the first
// [WebView.Attach], on the main goroutine; it has no effect off Linux.
//
// Since: 1.0
func SetCEFProfileInMemory() error {
	if cefProfile.locked {
		return errCEFProfileLocked
	}
	cefProfile.dir = ""
	cefProfile.inMemory = true
	return nil
}

// resolveCEFProfile turns the recorded choice into the two paths CEF is
// configured with: cache is the profile (empty means incognito), root the
// installation-data directory holding the process-singleton lock. root is
// never empty unless everything failed — with both empty CEF falls back to a
// per-user directory shared by every CEF app on the machine, the singleton
// hazard #125 exists to settle.
func resolveCEFProfile() (cache, root string) {
	if cefProfile.inMemory {
		return "", tempCEFRoot()
	}

	dir := cefProfile.dir
	if dir == "" {
		dir = defaultCEFProfileDir()
		if dir == "" {
			// No app storage to derive a location from; visible fallback to
			// the in-memory behaviour rather than a silently shared profile.
			bellui.LogError("webview: no app storage root for the CEF profile; the profile will not persist", nil)
			return "", tempCEFRoot()
		}
	}
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	return dir, dir
}

// tempCEFRoot is a fresh per-process directory for CEF's installation data
// when no profile persists: nothing durable, nothing contended between
// instances.
func tempCEFRoot() string {
	tmp, err := os.MkdirTemp("", "bellui-webview-cef-")
	if err != nil {
		bellui.LogError("webview: no temporary directory for CEF's installation data", err)
		return ""
	}
	return tmp
}

// defaultCEFProfileDir is the profile location when the app chose nothing:
// "webview-cef" under the app's storage root, the per-app directory every
// other platform's engine has an equivalent of.
func defaultCEFProfileDir() string {
	a := bellui.CurrentApp()
	if a == nil || a.Storage() == nil {
		return ""
	}
	root := a.Storage().RootURI()
	if root == nil || root.Path() == "" {
		return ""
	}
	return filepath.Join(root.Path(), "webview-cef")
}
