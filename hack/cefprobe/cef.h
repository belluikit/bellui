// Shim between the Go probe and CEF's C API (#60 phase 0, belluikit/bellui#102).
//
// Everything CEF-shaped lives on this side of the boundary. The Go side sees
// six functions and one callback, so cgo never has to describe a CEF struct.
//
// Two decisions this file exists to hold, both of which carry into
// belluikit/gocef (belluikit/bellui#103):
//
//   - We bind the **C API**, so libcef_dll_wrapper and the C++ ABI are not
//     involved. CEF's C API is exported straight from libcef.so; handlers are
//     structs of function pointers we fill in, which is the same shape as the
//     hand-rolled COM vtables in webview/webview2_windows.go.
//   - libcef.so is resolved with dlopen at **runtime**, not linked. Nothing
//     here needs CEF present to build, which is what keeps `go build` hermetic
//     on machines that have never seen a Chromium payload — the same bargain
//     webview/webview2loader_windows.go strikes for the WebView2 runtime.

#ifndef BELLUI_CEFPROBE_CEF_H
#define BELLUI_CEFPROBE_CEF_H

#include <stddef.h>

// belluiCefInit loads libcef.so out of cefDir and initializes the browser
// process. helperPath names the subprocess executable (see helper.c); switches
// is a space-separated list of Chromium command-line switches appended to our
// argv, which is how the probe reaches --disable-gpu and friends without
// implementing on_before_command_line_processing.
//
// Returns 0 on success, non-zero on failure, with belluiCefError describing it.
int belluiCefInit(const char *cefDir, const char *helperPath, const char *switches);

// belluiCefCreateBrowser creates a windowless browser showing url at w*h
// pixels. sharedTexture asks for cef_window_info_t::shared_texture_enabled, so
// frames arrive through on_accelerated_paint (dma-buf planes) instead of
// on_paint (a BGRA buffer) — the zero-copy question WPE could not pose.
int belluiCefCreateBrowser(const char *url, int w, int h, int sharedTexture);

// belluiCefWork runs one iteration of CEF's message loop. It must be called
// from the thread that called belluiCefInit — the browser UI thread — which on
// the bellui side is the main goroutine, so paint callbacks arrive there too.
void belluiCefWork(void);

// belluiCefResize tells CEF the view is now w*h pixels and asks for a repaint.
void belluiCefResize(int w, int h);

// belluiCefShutdown closes the browser and shuts CEF down.
void belluiCefShutdown(void);

// belluiCefError returns the last failure message, or "" if there was none.
const char *belluiCefError(void);

// belluiCefPaint is implemented in Go. buffer is BGRA with an upper-left
// origin, w*h*4 bytes, valid only for the duration of the call.
extern void belluiCefPaint(void *buffer, int w, int h, int isPopup);

// belluiCefAccelPaint is implemented in Go, and reports what a dma-buf frame
// looks like: how many planes, the fd and stride of the first, and the format
// modifier. The probe only measures that this path fires and what it carries;
// importing it into a texture is a later phase.
extern void belluiCefAccelPaint(int planeCount, int fd, unsigned int stride,
                                unsigned long long modifier, int format);

#endif // BELLUI_CEFPROBE_CEF_H
