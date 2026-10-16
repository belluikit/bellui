//go:build darwin && !ios && !wasm && !js

#include <stdint.h>

// NSOpenPanel / NSSavePanel bridge for the opt-in native file dialogs (pre-rebrand tracker #514,
// phase 2). belluiShowFilePanel schedules the panel on the main thread and
// returns immediately; when the user finishes it calls back into Go via the
// exported belluiFileDialogResponse(handle, paths, count, cancelled). A cancelled
// panel reports count 0; a single-select open/save/folder reports count 1;
// a multi-select open (pre-rebrand tracker #716) reports one entry per chosen file.
//
// Ownership: every string passed in is borrowed and copied before the call
// returns, so Go frees its buffers as usual.
void belluiShowFilePanel(
    uintptr_t parentWindow,
    int forSave,
    int directory,
    int multi,
    const char *title,
    const char *prompt,
    const char *initialDir,
    const char *initialName,
    const char **exts, int extCount,
    const char **mimes, int mimeCount,
    const char **legacyExts, int legacyExtCount,
    int handle);
