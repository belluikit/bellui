//go:build windows

package webview

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// This file finds the WebView2 runtime and creates an environment in it,
// without cgo and without a redistributable of our own.
//
// There are two ways in. The supported one is WebView2Loader.dll from the SDK,
// which an app may ship beside its executable — if it is there we use it, so a
// fixed-version or self-contained deployment keeps working. Otherwise we do
// what that DLL does: read the Evergreen Runtime's install path out of the
// registry and call the runtime client's own entry point. Neither path embeds a
// binary in bellui, and neither adds a dependency.
//
// If both fail there is no runtime on the machine, and Attach reports it as an
// error. That is the documented degradation for this backend and it needs no
// special case: a NativeViewHost whose Attach fails is inert — it occupies its
// space and draws nothing — which is exactly how an unsupported platform
// behaves.

// webview2MinAPIVersion is the minimum runtime this backend accepts, taken from
// the SDK release its API surface comes from (1.0.1150.38). The runtime is
// Evergreen, so anything current is far past it; the check exists so a machine
// pinned to something ancient reports ErrRuntimeMissing rather than failing
// deep inside a call that is not there.
const webview2MinAPIVersion = 1150

// Registry location of the Evergreen Runtime, per Microsoft's own detection
// guidance. The value is written under the 32-bit view even on 64-bit Windows,
// which is why the key is opened with KEY_WOW64_32KEY.
const (
	webview2ClientStateKey = `SOFTWARE\Microsoft\EdgeUpdate\ClientState\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}`
	webview2ClientStateVal = "EBWebView"
)

// COREWEBVIEW2_RUNTIME_TYPE, the runtime kind the client entry point is told to
// use: the machine-wide Evergreen install, or a fixed-version folder the app
// pointed us at.
const (
	webview2RuntimeInstalled = 0
	webview2RuntimeEmbedded  = 1
)

const loadWithAlteredSearchPath = 0x00000008

// webview2Env is the resolved way to create an environment, worked out once per
// process. Exactly one of the two procedure addresses is set.
type webview2Env struct {
	loaderProc   uintptr // WebView2Loader.dll CreateCoreWebView2EnvironmentWithOptions
	clientProc   uintptr // the runtime client's CreateWebViewEnvironmentWithOptionsInternal
	runtimeType  uintptr
	browserDir   string // fixed-version folder, when one was named
	resolveError error
}

var (
	webview2Once sync.Once
	webview2     webview2Env
)

// webview2Runtime resolves the runtime once and reports how to reach it.
func webview2Runtime() (*webview2Env, error) {
	webview2Once.Do(resolveWebView2Runtime)
	if webview2.resolveError != nil {
		return nil, webview2.resolveError
	}
	return &webview2, nil
}

func resolveWebView2Runtime() {
	// A fixed-version runtime the app unpacked itself, named the way the SDK
	// names it. Honoured on both paths below.
	webview2.browserDir = os.Getenv("WEBVIEW2_BROWSER_EXECUTABLE_FOLDER")

	// The SDK loader, if the app ships one. LoadLibrary's ordinary search finds
	// it beside the executable, which is where the SDK puts it.
	if mod, err := windows.LoadLibrary("WebView2Loader.dll"); err == nil {
		if proc, err := windows.GetProcAddress(mod, "CreateCoreWebView2EnvironmentWithOptions"); err == nil {
			webview2.loaderProc = proc
			return
		}
	}

	dir := webview2.browserDir
	webview2.runtimeType = webview2RuntimeEmbedded
	if dir == "" {
		var err error
		if dir, err = installedWebView2Dir(); err != nil {
			webview2.resolveError = err
			return
		}
		webview2.runtimeType = webview2RuntimeInstalled
	}

	dll := webview2ClientDLL(dir)
	mod, err := windows.LoadLibraryEx(dll, 0, loadWithAlteredSearchPath)
	if err != nil {
		webview2.resolveError = fmt.Errorf("%w: cannot load %s: %v", ErrRuntimeMissing, dll, err)
		return
	}
	proc, err := windows.GetProcAddress(mod, "CreateWebViewEnvironmentWithOptionsInternal")
	if err != nil {
		webview2.resolveError = fmt.Errorf("%w: %s has no environment entry point: %v", ErrRuntimeMissing, dll, err)
		return
	}
	webview2.clientProc = proc
}

// installedWebView2Dir reads the Evergreen Runtime's install directory, per
// machine first and then per user, and checks it is new enough.
func installedWebView2Dir() (string, error) {
	for _, root := range []registry.Key{registry.LOCAL_MACHINE, registry.CURRENT_USER} {
		key, err := registry.OpenKey(root, webview2ClientStateKey, registry.QUERY_VALUE|registry.WOW64_32KEY)
		if err != nil {
			continue
		}
		dir, _, err := key.GetStringValue(webview2ClientStateVal)
		key.Close()
		if err != nil || dir == "" {
			continue
		}
		if !webview2VersionOK(filepath.Base(strings.TrimRight(dir, `\/`))) {
			continue
		}
		return dir, nil
	}
	return "", ErrRuntimeMissing
}

// webview2VersionOK reports whether an Evergreen Runtime version string names a
// runtime whose API is at least the one this backend was written against. The
// third field is the API version: 1.0.1150.38 and 120.0.2210.91 both carry it
// in the same place.
func webview2VersionOK(version string) bool {
	fields := strings.Split(version, ".")
	if len(fields) < 3 {
		return false
	}
	api, err := strconv.Atoi(fields[2])
	if err != nil {
		return false
	}
	return api >= webview2MinAPIVersion
}

// webview2ClientDLL is the path of the runtime client inside an install
// directory. The runtime ships one build per architecture and only the matching
// one can be loaded into this process.
func webview2ClientDLL(dir string) string {
	arch := runtime.GOARCH
	switch arch {
	case "amd64":
		arch = "x64"
	case "386":
		arch = "x86"
	}
	return filepath.Join(dir, "EBWebView", arch, "EmbeddedBrowserWebView.dll")
}

// createWebView2Environment starts environment creation and reports the
// synchronous HRESULT. Success only means the request was accepted: the
// environment itself arrives later, on the handler.
func createWebView2Environment(userDataFolder string, h *comHandler) (int32, error) {
	env, err := webview2Runtime()
	if err != nil {
		return 0, err
	}

	folder, err := windows.UTF16PtrFromString(userDataFolder)
	if err != nil {
		return 0, err
	}
	defer runtime.KeepAlive(folder)

	if env.loaderProc != 0 {
		var browser *uint16
		if env.browserDir != "" {
			if browser, err = windows.UTF16PtrFromString(env.browserDir); err != nil {
				return 0, err
			}
			defer runtime.KeepAlive(browser)
		}
		hr, _, _ := syscall.SyscallN(env.loaderProc,
			uintptr(unsafe.Pointer(browser)), uintptr(unsafe.Pointer(folder)), 0, h.ptr())
		return int32(hr), nil
	}

	// The client entry point takes the arguments the loader would have passed
	// it: a flag the loader always sets, which runtime kind was found, and then
	// the same user data folder, options and completion handler.
	hr, _, _ := syscall.SyscallN(env.clientProc,
		1, env.runtimeType, uintptr(unsafe.Pointer(folder)), 0, h.ptr())
	return int32(hr), nil
}

// webview2UserDataFolder is where the runtime keeps this app's browser profile.
// It must be writable, and the runtime's own default — beside the executable —
// is not when the app is installed under Program Files, so a location is always
// supplied.
func webview2UserDataFolder() (string, error) {
	if dir := os.Getenv("WEBVIEW2_USER_DATA_FOLDER"); dir != "" {
		return dir, nil
	}

	base, err := os.UserCacheDir() // %LOCALAPPDATA% on Windows
	if err != nil {
		return "", err
	}

	name := "bellui-webview"
	if exe, err := os.Executable(); err == nil {
		name = strings.TrimSuffix(filepath.Base(exe), filepath.Ext(exe))
	}

	dir := filepath.Join(base, name, "WebView2")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}
