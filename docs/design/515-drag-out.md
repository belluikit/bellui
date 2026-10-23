# pre-rebrand tracker #515 Part B — Drag-out to other applications (design)

Status: **shipped.** Drag-out is implemented on macOS, Windows, Linux X11 and
Wayland (the BSDs use the X11 backend), and on the web through a separate
pull-based path (see *Web* below). This document began as the design plus a
macOS-only spike; the per-platform sections keep that design and note where the
built backend differs from it. Tracks bellui pre-rebrand tracker #515 Part B.

## Problem

Bellui's drag-and-drop is receive-only: `bellui.Droppable` / `DroppableWindow`
deliver OS file drops *into* a widget/window (pre-rebrand tracker #515 Part A, shipped). There is no
way to drag a file, a text selection, or an image *out* of a bellui app into
Finder/Explorer, a text editor, or a browser. GLFW exposes **no drag-source
API**, so every backend must be hand-rolled against the native window handle —
the same way the fork already hand-rolls clipboard flavors (pre-rebrand tracker #513), native file
dialogs (pre-rebrand tracker #514) and window modality (pre-rebrand tracker #498) via `RunNative`.

This is the outbound counterpart of the inbound `Droppable`.

## Public API (`driver/desktop`)

```go
// DragData is the payload carried out of the app during a drag-out. Any subset
// of flavors may be set; empty/nil fields are omitted from the drag.
type DragData struct {
	Text  string       // plain-text flavor
	URIs  []bellui.URI   // file references (only file:// URIs are carried)
	Image image.Image  // image flavor; when set, also used as the drag image
}

// DragSource is an optional capability of a bellui.Driver (asserted like the
// fork's other optional interfaces). GLFW has no drag-source API, so each
// desktop backend hand-rolls it; drivers that cannot report false.
type DragSource interface {
	// StartDragging begins a native drag session carrying data from the key
	// window. Call from a widget's Dragged handler on the main goroutine.
	// Returns false when the driver/platform cannot start a native drag, so the
	// caller keeps its in-app behavior.
	StartDragging(data DragData) bool
}

// StartDrag is package sugar over the driver assertion (mirrors the
// clipboard-flavor helpers): begins a drag-out if the active driver supports
// it, else returns false.
func StartDrag(data DragData) bool
```

### Why no window / event parameters

A drag-out is always initiated **synchronously from a `Draggable.Dragged`
handler**, which bellui dispatches on the main goroutine *inside* the platform's
mouse-drag event processing. So the backend can recover both pieces of native
context it needs without the caller threading them through:

- the originating window = the **key window** (`[NSApp keyWindow]` on macOS;
  `GetForegroundWindow`/the drag HWND on Windows; the focused surface on
  Wayland/X11);
- the mouse event that must seed the session = the **current event**
  (`[NSApp currentEvent]` on macOS).

This keeps the call site a one-liner (`desktop.StartDrag(data)`) — a widget does
not need a `bellui.Window` reference it usually lacks.

## Per-platform mechanism

The design had all four desktop backends reach the native handle through
`driver.NativeWindow.RunNative` (the `MacWindowContext` / `WindowsWindowContext` /
`X11WindowContext` / Wayland context already used by dialogs, modality and
clipboard). As built, X11 runs on its own Xlib connection and Wayland goes
through the glfw fork; see those sections.

### macOS — `NSDraggingSession` (implemented; the original spike)

`internal/driver/glfw/dragsource_darwin.{go,m,h}`, MRC ObjC cgo, mirroring
`dialog/file_darwin_native.*`:

1. Resolve `NSView *view = [[NSApp keyWindow] contentView]` and
   `NSEvent *event = [NSApp currentEvent]` (the mouse-drag event; if there is no
   mouse event we cannot seed a session → return false).
2. Build `NSArray<NSDraggingItem*>`: one item per file URL
   (`initWithPasteboardWriter:` an `NSURL fileURLWithPath:` — writes
   `public.file-url`), plus one for text (`initWithPasteboardWriter:` an
   `NSString` — writes `public.utf8-plain-text`). `NSString`/`NSURL` already
   conform to `NSPasteboardWriting`, so no manual pasteboard-type plumbing.
3. Each item's `setDraggingFrame:contents:` uses the supplied image, else the
   file's Finder icon, else the app icon — so the drag always has a visual.
4. `[view beginDraggingSessionWithItems:event:source:aSource]` where `aSource`
   is a retained singleton implementing `NSDraggingSource`
   (`sourceOperationMaskForDraggingContext:` → `Copy|Generic`).

`beginDraggingSessionWithItems:event:source:` is **non-blocking** (unlike the
deprecated `dragImage:…slideBack:`): it returns immediately and AppKit drives the
drag, so bellui's event loop is not stalled.

### Windows — OLE `DoDragDrop` (implemented, pre-rebrand tracker #837)

- `DoDragDrop(pDataObject, pDropSource, effects, &effect)` **blocks in a modal
  loop** until the drop completes — so it must NOT run on the render/UI thread.
  Run it on a dedicated `runtime.LockOSThread` goroutine that `OleInitialize`s
  (STA), exactly like the pre-rebrand tracker #514 native-dialog COM thread; marshal the result back
  with `bellui.Do`.
- Implement `IDataObject` (SetData `CF_HDROP` for files, `CF_UNICODETEXT` for
  text, `CF_DIB`/`"PNG"` for image — reuse the `internal/clipboard` DIB/PNG
  codecs from pre-rebrand tracker #513) and a minimal `IDropSource` (`QueryContinueDrag` /
  `GiveFeedback`). cgo-free via `x/sys/windows` COM syscalls, as pre-rebrand tracker #514 did for the
  file dialog.
- The drag image uses `IDragSourceHelper::InitializeFromBitmap`.
- As built: `internal/driver/glfw/dragsource_windows.go` + `dragsource_windows_com.go`.

### Linux/X11 — XDND source protocol (implemented, pre-rebrand tracker #838)

- Hand-roll the XDND source side: own the `XdndSelection`, advertise
  `XdndAware`, send `XdndEnter`/`XdndPosition`/`XdndDrop` client messages to the
  window under the pointer (found by walking from the root), answer
  `XdndStatus`, serve `SelectionRequest` for the negotiated MIME types
  (`text/uri-list`, `text/plain;charset=utf-8`, `image/png`) — the same MIME
  codecs as the pre-rebrand tracker #513 clipboard.
- Needs the X11 `Display`/`Window` from the glfw fork's `GetX11Window/Display`
  (already used by pre-rebrand tracker #513/#598). A grab + motion-tracking loop drives it.
- As built (`dragsource_x11.{go,c,h}`): the drag runs on a dedicated locked OS
  thread with its **own private Xlib `Display` connection**, polling the pointer,
  so it never touches GLFW's connection or event loop. An optional cursor image
  trails the pointer in a click-through window (pre-rebrand tracker #865). The
  BSDs use this backend too.

### Wayland — `wl_data_device.start_drag` (implemented)

- Create a `wl_data_source`, `offer` each MIME type, `wl_data_device.start_drag`
  with the origin surface + an icon surface, and answer the `send` events with
  the payload. Needs a serial from a real input event (the mouse-down) — same
  focus/serial constraint as the pre-rebrand tracker #513 Wayland clipboard. Route through the glfw
  fork's Wayland accessors.
- As built (`dragsource_wayland.go`): GLFW itself creates the data source and
  calls `start_drag` via the belluikit/glfw fork's `StartWaylandDrag` (v1.7.0+),
  which owns the seat, data device and pointer serial. The fork exposes no
  completion signal and no icon surface, so on Wayland `DragData.OnCompleted` is
  **not** called and no drag icon is attached.
- The default Linux binary carries both X11 and Wayland backends and picks one
  at runtime via `build.IsWayland` (`dragsource_xdg.go`).

### Web — HTML5 drag and drop (implemented, pre-rebrand tracker #857/#874)

The push model above (`desktop.StartDrag` from `Dragged`) cannot work in a
browser: a native drag starts only from a user gesture, and once `dragstart`
fires the page stops receiving the mouse events that would run `Dragged`. So
`StartDrag` declines on wasm, and `dragsource_wasm.go` is **pull-based**
instead: on mousedown it hit-tests the bellui tree, and if the deepest
`event.Draggable` under the pointer implements `desktop.DragOutProvider` it marks
the canvas element draggable for that gesture; `dragstart` then pulls
`DragOutData()` into `dataTransfer`, and `dragend` reports the outcome.

## Threading & lifecycle

- Called on the main goroutine from `Dragged`. macOS/Wayland are non-blocking
  (AppKit / the compositor drive the drag); **Windows blocks**, so it offloads
  to a dedicated COM-STA goroutine and reports completion via `bellui.Do`.
- The drag image is supplied by `DragData.Image` (already an `image.Image`); a
  future refinement can render a `core.CanvasObject` ghost (the pre-rebrand tracker #819
  `DragImageProvider` shape) to an image and reuse it here.

## Completion reporting (pre-rebrand tracker #841, done)

`DragData.OnCompleted func(accepted bool)` reports the final drop outcome,
fired on the main goroutine via `bellui.Do`. The shared
`internal/driver/glfw/dragsource.go` holds the pending callback for the one
in-flight drag and exposes `notifyDragOutCompleted(accepted)`; each backend
calls it from its native completion signal:
`NSDraggingSource.draggingSession:endedAtPoint:operation:` (macOS, wired),
`DoDragDrop`'s returned effect (Windows), `XdndFinished` (X11) and `dragend`
(web). Wayland does not report completion (the glfw fork keeps
`wl_data_source.dnd_finished` internal), so `OnCompleted` is not called there.
`accepted` is true when the operation is anything other than none.

## CanvasObject drag image (pre-rebrand tracker #842, done)

`DragData.DragImageObject core.CanvasObject` bridges the in-canvas drag ghost
(pre-rebrand tracker #819 `core.DragImageProvider`): the shared `StartDragging` software-renders it
(`driver/software.Render`) to supply the drag image when `DragData.Image` is
nil, so a widget declares one drag visual for both in-canvas DnD and drag-out.
A raw `Image` always wins.

## Composition

- Complements the inbound `bellui.Droppable` (pre-rebrand tracker #515 Part A) — an app can be both a
  drop target and a drag source.
- Reuses the pre-rebrand tracker #513 clipboard-flavor payload codecs (uri-list, DIB/PNG) on
  Windows/Linux, so the flavor encoders have one definition.

## Follow-ups

- ~~**Windows `DoDragDrop`** (OLE, COM-STA thread)~~ — done (pre-rebrand tracker #837).
- ~~**Linux X11 XDND source**~~ — done (pre-rebrand tracker #838).
- ~~**Linux Wayland `wl_data_device.start_drag`**~~ — done (pre-rebrand tracker #838), without completion reporting or a drag icon.
- ~~Completion callback (`DragData.OnCompleted`)~~ — done (pre-rebrand tracker #841).
- ~~`core.CanvasObject` drag-image rendering (bridge to pre-rebrand tracker #819 `DragImageProvider`)~~ — done (pre-rebrand tracker #842).
- Human GUI verification of the macOS proof (drag into Finder / TextEdit) — pre-rebrand tracker #843.

### Platform backend files

`startNativeDrag` is partitioned per platform, all in `internal/driver/glfw/`:
`dragsource_darwin.{go,m,h}` (macOS), `dragsource_windows.go` +
`dragsource_windows_com.go` (Windows), `dragsource_xdg.go` (Linux and the BSDs,
choosing at runtime between `dragsource_x11.{go,c,h}` and
`dragsource_wayland.go`; the `_stub` files cover forced single-backend builds),
and `dragsource_other.go` declining everywhere else — iOS, and wasm/js, where
the pull-based `dragsource_wasm.go` handles drag-out instead.
