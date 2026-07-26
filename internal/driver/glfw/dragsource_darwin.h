//go:build darwin && !ios && !wasm && !js

#include <stdbool.h>

// belluiStartDragOut begins an NSDraggingSession from the key window's content
// view, seeded by [NSApp currentEvent] (the in-flight mouse-drag event). text is
// a plain-text flavor ("" for none); filePathsNL is a newline-separated list of
// file-system paths ("" for none); imageData/imageLen is an optional PNG used as
// the drag image (NULL/0 for none). Returns false when there is no key window,
// no current mouse event, or no payload.
bool belluiStartDragOut(const char* text, const char* filePathsNL, const void* imageData, int imageLen);
