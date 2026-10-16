//go:build accessibility && darwin && !wasm

package glfw

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation -framework AppKit

#include <stdlib.h>
#include "accessibility_darwin.h"
*/
import "C"

import (
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/accessibility"
	"github.com/belluikit/bellui/internal/textinput"
)

// This file is the Go half of the NSAccessibility text protocol (pre-rebrand tracker #501):
// VoiceOver text navigation and editing ask the focused text field for its
// character count, selection, caret line and sub-ranges, and write back a
// selection or replacement text. The ObjC bridge forwards each query to the
// exports below, which resolve the element's editor and read or edit its
// document through the shared text-input seam ([textinput], pre-rebrand tracker #772) — one live
// document view, no snapshot staleness — converting between the widget's rune
// offsets and the UTF-16 code units NSAccessibility ranges are expressed in.
//
// All exports run on the main thread / main goroutine, like the other AX
// callbacks.

// a11yTexter resolves the AccessibleTexter behind an element handle, or nil.
func a11yTexter(handle C.uintptr_t) core.AccessibleTexter {
	t, ok := a11yTargets[uintptr(handle)]
	if !ok {
		return nil
	}
	tx, _ := t.node.Object.(core.AccessibleTexter)
	return tx
}

// a11yTextTarget resolves the editor behind an element handle and captures its
// document view through the shared seam ([textinput.Read], pre-rebrand tracker #772), so every text
// query reads one document view. ok is false when the handle has no editor.
func a11yTextTarget(handle C.uintptr_t) (core.AccessibleTexter, textinput.View, bool) {
	tx := a11yTexter(handle)
	if tx == nil {
		return nil, textinput.View{}, false
	}
	return tx, textinput.Read(tx), true
}

//export belluiAccessibilityTextLength
func belluiAccessibilityTextLength(handle C.uintptr_t) C.long {
	_, v, ok := a11yTextTarget(handle)
	if !ok {
		return 0
	}
	return C.long(accessibility.IndexText(v.Text).UTF16Len())
}

//export belluiAccessibilityTextSelection
func belluiAccessibilityTextSelection(handle C.uintptr_t, loc, length *C.long) C.bool {
	_, v, ok := a11yTextTarget(handle)
	if !ok {
		return C.bool(false)
	}
	ix := accessibility.IndexText(v.Text)
	u16Start := ix.UTF16FromRune(v.SelStart)
	*loc = C.long(u16Start)
	*length = C.long(ix.UTF16FromRune(v.SelEnd) - u16Start)
	return C.bool(true)
}

//export belluiAccessibilityTextSelected
func belluiAccessibilityTextSelected(handle C.uintptr_t) *C.char {
	_, v, ok := a11yTextTarget(handle)
	if !ok {
		return nil
	}
	ix := accessibility.IndexText(v.Text)
	return C.CString(ix.Substring(v.SelStart, v.SelEnd)) // freed by the ObjC caller
}

//export belluiAccessibilityTextForRange
func belluiAccessibilityTextForRange(handle C.uintptr_t, loc, length C.long) *C.char {
	_, v, ok := a11yTextTarget(handle)
	if !ok {
		return nil
	}
	ix := accessibility.IndexText(v.Text)
	return C.CString(ix.Substring(ix.RuneFromUTF16(int(loc)), ix.RuneFromUTF16(int(loc+length)))) // freed by the ObjC caller
}

// a11yTextLayout resolves the optional visual-layout extension (pre-rebrand tracker #990) behind
// an element handle, or nil when the editor exposes no layout — line queries
// then fall back to logical (newline-delimited) lines from the text index.
func a11yTextLayout(handle C.uintptr_t) core.AccessibleTextLayouter {
	lay, _ := a11yTexter(handle).(core.AccessibleTextLayouter)
	return lay
}

//export belluiAccessibilityTextInsertionLine
func belluiAccessibilityTextInsertionLine(handle C.uintptr_t) C.long {
	_, v, ok := a11yTextTarget(handle)
	if !ok {
		return 0
	}
	if lay := a11yTextLayout(handle); lay != nil {
		return C.long(lay.AccessibleLineForOffset(v.Caret))
	}
	return C.long(accessibility.IndexText(v.Text).LineForRune(v.Caret))
}

//export belluiAccessibilityTextLineForIndex
func belluiAccessibilityTextLineForIndex(handle C.uintptr_t, index C.long) C.long {
	_, v, ok := a11yTextTarget(handle)
	if !ok {
		return 0
	}
	ix := accessibility.IndexText(v.Text)
	if lay := a11yTextLayout(handle); lay != nil {
		return C.long(lay.AccessibleLineForOffset(ix.RuneFromUTF16(int(index))))
	}
	return C.long(ix.LineForRune(ix.RuneFromUTF16(int(index))))
}

//export belluiAccessibilityTextRangeForLine
func belluiAccessibilityTextRangeForLine(handle C.uintptr_t, line C.long, loc, length *C.long) C.bool {
	_, v, ok := a11yTextTarget(handle)
	if !ok {
		return C.bool(false)
	}
	ix := accessibility.IndexText(v.Text)
	var start, end int
	if lay := a11yTextLayout(handle); lay != nil {
		start, end, ok = lay.AccessibleLineRange(int(line))
	} else {
		start, end, ok = ix.LineRuneRange(int(line))
	}
	if !ok {
		return C.bool(false)
	}
	u16Start := ix.UTF16FromRune(start)
	*loc = C.long(u16Start)
	*length = C.long(ix.UTF16FromRune(end) - u16Start)
	return C.bool(true)
}

// belluiAccessibilityTextFrameForRange resolves AXBoundsForRange (pre-rebrand tracker #990): the
// on-screen bounds of a UTF-16 character range, answered in the same
// window-local coordinate space (top-left origin, points) as the element's
// localFrame — the ObjC side converts to the screen exactly as
// accessibilityFrame does. Returns false when the editor exposes no layout or
// no part of the range is visible; VoiceOver then falls back to the element
// bounds.
//
//export belluiAccessibilityTextFrameForRange
func belluiAccessibilityTextFrameForRange(handle C.uintptr_t, loc, length C.long, x, y, w, h *C.double) C.bool {
	_, v, ok := a11yTextTarget(handle)
	if !ok {
		return C.bool(false)
	}
	lay := a11yTextLayout(handle)
	if lay == nil {
		return C.bool(false)
	}
	t, ok := a11yTargets[uintptr(handle)]
	if !ok {
		return C.bool(false)
	}
	ix := accessibility.IndexText(v.Text)
	rects := lay.AccessibleRangeBounds(ix.RuneFromUTF16(int(loc)), ix.RuneFromUTF16(int(loc+length)))
	if len(rects) == 0 {
		return C.bool(false)
	}
	u := rects[0]
	for _, r := range rects[1:] {
		u = unionRect(u, r)
	}
	// Widget-local dp → absolute canvas dp, then trimmed to the node's
	// ancestor clip (pre-rebrand tracker #1003): a field half-scrolled out of a parent Scroll must
	// not report range geometry past the scroll container, consistent with the
	// element-frame Clip culling in the reconcile.
	abs := geo.Rectangle{Pos: geo.NewPos(t.node.Pos.X+u.Pos.X, t.node.Pos.Y+u.Pos.Y), Size: u.Size}
	if t.node.HasClip {
		var visible bool
		if abs, visible = intersectRect(abs, t.node.Clip); !visible {
			return C.bool(false)
		}
	}
	// Canvas dp → window points — the same dp × Scale() mapping applyA11yProps
	// uses for the element frame.
	sc := float64(t.win.canvas.Scale())
	*x = C.double(float64(abs.Pos.X) * sc)
	*y = C.double(float64(abs.Pos.Y) * sc)
	*w = C.double(float64(abs.Size.Width) * sc)
	*h = C.double(float64(abs.Size.Height) * sc)
	return C.bool(true)
}

//export belluiAccessibilityTextSetSelection
func belluiAccessibilityTextSetSelection(handle C.uintptr_t, loc, length C.long) {
	tx, v, ok := a11yTextTarget(handle)
	if !ok {
		return
	}
	ix := accessibility.IndexText(v.Text)
	textinput.Select(tx, ix.RuneFromUTF16(int(loc)), ix.RuneFromUTF16(int(loc+length)))
}

//export belluiAccessibilityTextReplaceSelection
func belluiAccessibilityTextReplaceSelection(handle C.uintptr_t, text *C.char) {
	tx := a11yTexter(handle)
	if tx == nil {
		return
	}
	textinput.InsertText(tx, C.GoString(text))
}

//export belluiAccessibilityTextSetValue
func belluiAccessibilityTextSetValue(handle C.uintptr_t, text *C.char) {
	tx := a11yTexter(handle)
	if tx == nil {
		return
	}
	textinput.ReplaceAll(tx, C.GoString(text))
}
