//go:build linux

// Minimal offscreen WPE WebKit driver for the #60 spike.
//
// WPE and GLib own a thread of their own here: everything — EGL setup, the web
// view, the main loop — runs on one pthread started by belluiWpeStart, and
// finished frames are copied into a mutex-guarded staging buffer that Go polls.
// That keeps GLib entirely out of the Go scheduler, which is the same reason
// the driver bridges elsewhere in this repo marshal rather than share.
#ifndef BELLUI_WPE_H
#define BELLUI_WPE_H

#include <stdbool.h>
#include <stdint.h>

// belluiWpeStart brings up an offscreen web view of the given size showing
// html, and starts producing frames. It returns 0 on success.
int belluiWpeStart(int width, int height, const char *html);

// belluiWpeFrame copies the most recent frame into out (BGRA/ARGB as the
// engine produced it) and reports its geometry. It returns the frame's serial
// number, or 0 when no frame has arrived yet; a caller that sees the same
// serial twice has not missed anything.
uint64_t belluiWpeFrame(unsigned char *out, int outLen, int *w, int *h, int *stride);

// belluiWpeExportKind reports how the engine hands frames over: 1 SHM (a CPU
// buffer), 2 EGLImage (a GPU texture), 0 nothing yet. This is the measurement
// the spike exists for.
int belluiWpeExportKind(void);

void belluiWpeStop(void);

#endif
