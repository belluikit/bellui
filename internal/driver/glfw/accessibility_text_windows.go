//go:build accessibility && windows && !wasm

package glfw

/*
#include <stdlib.h>
#include "accessibility_windows.h"
*/
import "C"

import (
	"unsafe"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/accessibility"
	"github.com/belluikit/bellui/internal/textinput"
)

// This file is the Go half of the UIA Text pattern (#487): Narrator/NVDA
// text navigation asks the focused text field for its content, selection and
// unit boundaries, and writes back a selection. The C bridge keeps range
// endpoints as rune offsets — UIA text units are abstract, so no platform
// code-unit space ever reaches the client — and forwards each query to the
// exports below, which resolve the element's editor and read or edit its
// document through the shared text-input seam ([textinput], #772) — one live
// document view, no snapshot staleness — on top of the shared accessibility.TextIndex.
//
// All exports run on the main thread / main goroutine, like the other UIA
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
// document view through the shared seam ([textinput.Read], #772), so every text
// query reads one document view. ok is false when the handle has no editor.
func a11yTextTarget(handle C.uintptr_t) (core.AccessibleTexter, textinput.View, bool) {
	tx := a11yTexter(handle)
	if tx == nil {
		return nil, textinput.View{}, false
	}
	return tx, textinput.Read(tx), true
}

//export belluiWinA11yTextLength
func belluiWinA11yTextLength(handle C.uintptr_t) C.long {
	_, v, ok := a11yTextTarget(handle)
	if !ok {
		return 0
	}
	return C.long(accessibility.IndexText(v.Text).RuneLen())
}

// belluiWinA11yTextForRange returns the text of the rune range [start, end) as
// a C string the caller owns (frees). A negative end means the full length.
//
//export belluiWinA11yTextForRange
func belluiWinA11yTextForRange(handle C.uintptr_t, start, end C.long) *C.char {
	_, v, ok := a11yTextTarget(handle)
	if !ok {
		return nil
	}
	ix := accessibility.IndexText(v.Text)
	to := int(end)
	if to < 0 {
		to = ix.RuneLen()
	}
	return C.CString(ix.Substring(int(start), to)) // freed by the C caller
}

//export belluiWinA11yTextSelection
func belluiWinA11yTextSelection(handle C.uintptr_t, start, end *C.long) {
	*start, *end = 0, 0
	tx := a11yTexter(handle)
	if tx == nil {
		return
	}
	// UIA keeps rune offsets, so the selection needs no text; read selection
	// only to avoid re-materializing a large document per caret query (#789).
	s, e := textinput.ReadSelection(tx)
	*start = C.long(s)
	*end = C.long(e)
}

//export belluiWinA11yTextSetSelection
func belluiWinA11yTextSetSelection(handle C.uintptr_t, start, end C.long) {
	tx := a11yTexter(handle)
	if tx == nil {
		return
	}
	textinput.Select(tx, int(start), int(end))
}

// a11yTextLayout resolves the optional visual-layout extension (#990) behind
// an element handle, or nil when the editor exposes no layout — the line unit
// then falls back to logical (newline-delimited) lines from the text index.
func a11yTextLayout(handle C.uintptr_t) core.AccessibleTextLayouter {
	lay, _ := a11yTexter(handle).(core.AccessibleTextLayouter)
	return lay
}

// a11yVisualLineStep moves a rune offset by count visual lines (negative =
// backwards), mirroring TextIndex.UnitStep's line semantics — offsets move
// between line-start boundaries, and a backward step first snaps to the
// current line's start — against the widget's soft-wrapped layout.
func a11yVisualLineStep(lay core.AccessibleTextLayouter, off, count int) (newOff, moved int) {
	dir := 1
	if count < 0 {
		dir, count = -1, -count
	}
	for ; moved < count; moved++ {
		line := lay.AccessibleLineForOffset(off)
		start, _, ok := lay.AccessibleLineRange(line)
		if !ok {
			break
		}
		if dir > 0 {
			next, _, ok := lay.AccessibleLineRange(line + 1)
			if !ok {
				break
			}
			off = next
			continue
		}
		if off > start {
			off = start
			continue
		}
		prev, _, ok := lay.AccessibleLineRange(line - 1)
		if !ok {
			break
		}
		off = prev
	}
	return off, moved * dir
}

// belluiWinA11yTextUnitBounds returns the rune range [start, end) of the UIA
// text unit containing the offset (see accessibility.TextIndex.UnitBounds for the
// unit semantics). The line unit resolves against the widget's visual
// (soft-wrapped) layout when it exposes one (#990); paragraph stays logical.
//
//export belluiWinA11yTextUnitBounds
func belluiWinA11yTextUnitBounds(handle C.uintptr_t, unit C.int, off C.long, start, end *C.long) {
	*start, *end = 0, 0
	_, v, ok := a11yTextTarget(handle)
	if !ok {
		return
	}
	if int(unit) == accessibility.UnitLine {
		if lay := a11yTextLayout(handle); lay != nil {
			if s, e, ok := lay.AccessibleLineRange(lay.AccessibleLineForOffset(int(off))); ok {
				*start, *end = C.long(s), C.long(e)
				return
			}
		}
	}
	s, e := accessibility.IndexText(v.Text).UnitBounds(int(unit), int(off))
	*start = C.long(s)
	*end = C.long(e)
}

// belluiWinA11yTextUnitStep moves a rune offset by count units (negative =
// backwards), reporting the units actually crossed through moved. The line
// unit steps visual (soft-wrapped) lines when the widget exposes its layout
// (#990).
//
//export belluiWinA11yTextUnitStep
func belluiWinA11yTextUnitStep(handle C.uintptr_t, unit C.int, off C.long, count C.int, moved *C.int) C.long {
	*moved = 0
	_, v, ok := a11yTextTarget(handle)
	if !ok {
		return off
	}
	if int(unit) == accessibility.UnitLine {
		if lay := a11yTextLayout(handle); lay != nil {
			newOff, n := a11yVisualLineStep(lay, int(off), int(count))
			*moved = C.int(n)
			return C.long(newOff)
		}
	}
	newOff, n := accessibility.IndexText(v.Text).UnitStep(int(unit), int(off), int(count))
	*moved = C.int(n)
	return C.long(newOff)
}

// belluiWinA11yTextRangeRects answers the Text pattern's GetBoundingRectangles
// (#990): one rectangle per visible visual line of the rune range [start,
// end), as l,t,w,h quadruples in CLIENT-area pixels — the widget's viewport
// clipping already dropped scrolled-out lines, and the C side offsets by the
// window's screen origin before packing the SAFEARRAY. Returns the rectangle
// count; *out is a C-malloc'd array of 4*count doubles the caller frees, and
// stays untouched when the count is 0 (no layout, or nothing visible — the
// client then falls back to the element bounds).
//
//export belluiWinA11yTextRangeRects
func belluiWinA11yTextRangeRects(handle C.uintptr_t, start, end C.long, out **C.double) C.int {
	lay := a11yTextLayout(handle)
	if lay == nil {
		return 0
	}
	t, ok := a11yTargets[uintptr(handle)]
	if !ok {
		return 0
	}
	rects := lay.AccessibleRangeBounds(int(start), int(end))
	if len(rects) == 0 {
		return 0
	}
	// Widget-local dp → absolute canvas dp, trimmed to the node's ancestor
	// clip (#1003): a field half-scrolled out of a parent Scroll must not
	// report line rectangles past the scroll container, consistent with the
	// element-bounds Clip culling in the reconcile. Lines clipped away
	// entirely are dropped.
	clipped := rects[:0]
	for _, r := range rects {
		abs := geo.Rectangle{Pos: geo.NewPos(t.node.Pos.X+r.Pos.X, t.node.Pos.Y+r.Pos.Y), Size: r.Size}
		if t.node.HasClip {
			var visible bool
			if abs, visible = intersectRect(abs, t.node.Clip); !visible {
				continue
			}
		}
		clipped = append(clipped, abs)
	}
	if len(clipped) == 0 {
		return 0
	}
	// Canvas dp → client pixels — the same dp × Scale() mapping
	// applyWinA11yProps uses for the element bounds.
	sc := float64(t.win.canvas.Scale())
	buf := (*C.double)(C.malloc(C.size_t(len(clipped) * 4 * C.sizeof_double)))
	vals := unsafe.Slice(buf, len(clipped)*4)
	for i, r := range clipped {
		vals[i*4+0] = C.double(float64(r.Pos.X) * sc)
		vals[i*4+1] = C.double(float64(r.Pos.Y) * sc)
		vals[i*4+2] = C.double(float64(r.Size.Width) * sc)
		vals[i*4+3] = C.double(float64(r.Size.Height) * sc)
	}
	*out = buf
	return C.int(len(clipped))
}

// belluiWinA11yValueSet implements Value.SetValue: replace the whole content
// as one undo-recorded edit.
//
//export belluiWinA11yValueSet
func belluiWinA11yValueSet(handle C.uintptr_t, text *C.char) {
	tx := a11yTexter(handle)
	if tx == nil {
		return
	}
	textinput.ReplaceAll(tx, C.GoString(text))
}
