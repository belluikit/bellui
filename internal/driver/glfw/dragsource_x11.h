//go:build (linux && !wayland && !wasm && !test_web_driver) || freebsd || openbsd || netbsd

#ifndef BELLUI_DRAGSOURCE_X11_H
#define BELLUI_DRAGSOURCE_X11_H

// BelluiDragFlavor is one MIME representation of the drag payload, encoded in Go
// with the shared pre-rebrand tracker #513 clipboard codecs and served to the XDND target when it
// converts the XdndSelection. mime is a NUL-terminated MIME/target name (also
// used as the X11 target-atom name); data/len is the payload bytes.
typedef struct {
	const char* mime;
	const void* data;
	int         len;
} BelluiDragFlavor;

// belluiX11Available reports whether an X11 display connection can be opened (1)
// or not (0). It is a cheap capability probe so the caller can decline the drag
// and keep in-app behaviour when X is unreachable.
int belluiX11Available(void);

// belluiStartDragX11 runs a hand-rolled XDND (X Drag-and-Drop) source drag to
// completion on the calling thread (a modal poll loop) and returns 1 if a
// target accepted the drop, else 0. It opens its own Xlib connection, owns the
// XdndSelection, polls the pointer to find the XDND-aware window under it,
// negotiates via XdndEnter/Position/Status, serves SelectionRequests for the
// flavors, and reports the XdndFinished outcome. See pre-rebrand tracker #838 / pre-rebrand tracker #515 Part B.
//
// iconARGB (optional, may be NULL) is a drag-icon image drawn in a small,
// click-through window that trails the pointer for the whole drag, giving the
// visual feedback plain XDND lacks (pre-rebrand tracker #865). It is iconW×iconH pixels of
// premultiplied BGRA (one 32-bit little-endian 0xAARRGGBB word per pixel, row
// major, top-down); it is copied before the drag and need not outlive the call.
int belluiStartDragX11(BelluiDragFlavor* flavors, int count,
    const unsigned char* iconARGB, int iconW, int iconH);

#endif
