//go:build accessibility && linux && !android && !wasm && !js

package glfw

import (
	"github.com/godbus/dbus/v5"

	bellui "github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/internal/accessibility"
)

// P1 of the Linux AT-SPI2 backend (#986): the Action, Value, Text and
// EditableText interfaces layered on the P0 provider. Reads are served from
// snapshots captured on the main goroutine during reconcile (atspiNode.text /
// actions / val*), so a D-Bus query on a godbus goroutine never touches a live
// widget; mutations dispatch back to the widget via bellui.Do and the next
// reconcile re-snapshots and emits the change events.
//
// Value is property-only (served through propValue). Action, Text and
// EditableText add methods, exported on every node in exportNode; the handlers
// gate on the per-node capability flags so a call to an unsupported interface
// returns empty/false rather than acting. GetInterfaces advertises only the
// interfaces a node actually supports, so Orca queries the right ones.

// --- org.a11y.atspi.Action -----------------------------------------------

func (n *atspiNode) GetNActions() (int32, *dbus.Error) {
	n.p.mu.Lock()
	defer n.p.mu.Unlock()
	return int32(len(n.actions)), nil
}

func (n *atspiNode) actionAt(i int32) (core.AccessibleAction, bool) {
	n.p.mu.Lock()
	defer n.p.mu.Unlock()
	if i < 0 || int(i) >= len(n.actions) {
		return core.AccessibleAction{}, false
	}
	return n.actions[i], true
}

func (n *atspiNode) GetName(i int32) (string, *dbus.Error) {
	if a, ok := n.actionAt(i); ok {
		return a.Name, nil
	}
	return "", nil
}

func (n *atspiNode) GetDescription(i int32) (string, *dbus.Error) {
	if a, ok := n.actionAt(i); ok {
		return a.Description, nil
	}
	return "", nil
}

func (n *atspiNode) GetKeyBinding(i int32) (string, *dbus.Error) { return "", nil }

// atspiActionTuple is one entry of Action.GetActions' a(sss): name,
// description, key-binding.
type atspiActionTuple struct {
	Name        string
	Description string
	KeyBinding  string
}

func (n *atspiNode) GetActions() ([]atspiActionTuple, *dbus.Error) {
	n.p.mu.Lock()
	defer n.p.mu.Unlock()
	out := make([]atspiActionTuple, len(n.actions))
	for i, a := range n.actions {
		out[i] = atspiActionTuple{Name: a.Name, Description: a.Description}
	}
	return out, nil
}

func (n *atspiNode) DoAction(i int32) (bool, *dbus.Error) {
	a, ok := n.actionAt(i)
	if !ok {
		return false, nil
	}
	n.p.mu.Lock()
	obj := n.obj
	n.p.mu.Unlock()
	act, isActioner := obj.(core.AccessibleActioner)
	if !isActioner {
		return false, nil
	}
	bellui.Do(func() { act.PerformAccessibleAction(a.Name) })
	return true, nil
}

// --- org.a11y.atspi.Text (read; served from the reconcile snapshot) -------

// granToUnit maps an AT-SPI TextGranularity to the shared text-index unit.
func granToUnit(gran uint32) int {
	switch gran {
	case 0: // ATSPI_TEXT_GRANULARITY_CHAR
		return accessibility.UnitCharacter
	case 1: // WORD
		return accessibility.UnitWord
	case 3: // LINE
		return accessibility.UnitLine
	case 4: // PARAGRAPH
		return accessibility.UnitParagraph
	default: // SENTENCE (2) and anything else → line, the closest unit we model
		return accessibility.UnitLine
	}
}

func (n *atspiNode) GetText(start, end int32) (string, *dbus.Error) {
	n.p.mu.Lock()
	defer n.p.mu.Unlock()
	if !n.isText {
		return "", nil
	}
	if end < 0 {
		end = int32(n.text.RuneLen())
	}
	return n.text.Substring(int(start), int(end)), nil
}

func (n *atspiNode) GetCharacterCount() (int32, *dbus.Error) {
	n.p.mu.Lock()
	defer n.p.mu.Unlock()
	return int32(n.text.RuneLen()), nil
}

func (n *atspiNode) GetCaretOffset() (int32, *dbus.Error) {
	n.p.mu.Lock()
	defer n.p.mu.Unlock()
	return int32(n.caret), nil
}

func (n *atspiNode) SetCaretOffset(offset int32) (bool, *dbus.Error) {
	n.p.mu.Lock()
	obj, isText := n.obj, n.isText
	n.p.mu.Unlock()
	if !isText {
		return false, nil
	}
	if tx, ok := obj.(core.AccessibleTexter); ok {
		off := int(offset)
		bellui.Do(func() { tx.SetAccessibleSelection(off, off) })
		return true, nil
	}
	return false, nil
}

func (n *atspiNode) GetStringAtOffset(offset int32, gran uint32) (string, int32, int32, *dbus.Error) {
	n.p.mu.Lock()
	defer n.p.mu.Unlock()
	if !n.isText {
		return "", 0, 0, nil
	}
	s, e := n.text.UnitBounds(granToUnit(gran), int(offset))
	return n.text.Substring(s, e), int32(s), int32(e), nil
}

func (n *atspiNode) GetCharacterAtOffset(offset int32) (int32, *dbus.Error) {
	n.p.mu.Lock()
	defer n.p.mu.Unlock()
	if !n.isText || int(offset) < 0 || int(offset) >= n.text.RuneLen() {
		return 0, nil
	}
	return int32(n.text.RuneAt(int(offset))), nil
}

func (n *atspiNode) GetNSelections() (int32, *dbus.Error) {
	n.p.mu.Lock()
	defer n.p.mu.Unlock()
	if n.isText && n.selEnd > n.selStart {
		return 1, nil
	}
	return 0, nil
}

func (n *atspiNode) GetSelection(num int32) (int32, int32, *dbus.Error) {
	n.p.mu.Lock()
	defer n.p.mu.Unlock()
	if num != 0 || !n.isText || n.selEnd <= n.selStart {
		return 0, 0, nil
	}
	return int32(n.selStart), int32(n.selEnd), nil
}

func (n *atspiNode) setSelectionRange(start, end int32) bool {
	n.p.mu.Lock()
	obj, isText := n.obj, n.isText
	n.p.mu.Unlock()
	if !isText {
		return false
	}
	if tx, ok := obj.(core.AccessibleTexter); ok {
		s, e := int(start), int(end)
		bellui.Do(func() { tx.SetAccessibleSelection(s, e) })
		return true
	}
	return false
}

func (n *atspiNode) AddSelection(start, end int32) (bool, *dbus.Error) {
	return n.setSelectionRange(start, end), nil
}

func (n *atspiNode) SetSelection(num, start, end int32) (bool, *dbus.Error) {
	if num != 0 {
		return false, nil
	}
	return n.setSelectionRange(start, end), nil
}

func (n *atspiNode) RemoveSelection(num int32) (bool, *dbus.Error) {
	n.p.mu.Lock()
	caret := n.caret
	n.p.mu.Unlock()
	return n.setSelectionRange(int32(caret), int32(caret)), nil
}

// GetCharacterExtents / GetRangeExtents are best-effort: per-glyph geometry is
// not tracked, so a caret-height rect at the node origin is returned. Orca uses
// these for braille/magnifier follow, not speech. (Refinement tracked in #986.)
func (n *atspiNode) GetCharacterExtents(offset int32, coord uint32) (int32, int32, int32, int32, *dbus.Error) {
	n.p.mu.Lock()
	defer n.p.mu.Unlock()
	r := n.extentsFor(coord)
	return r.X, r.Y, 0, r.H, nil
}

func (n *atspiNode) GetRangeExtents(start, end int32, coord uint32) (int32, int32, int32, int32, *dbus.Error) {
	n.p.mu.Lock()
	defer n.p.mu.Unlock()
	r := n.extentsFor(coord)
	return r.X, r.Y, r.W, r.H, nil
}

func (n *atspiNode) GetOffsetAtPoint(x, y int32, coord uint32) (int32, *dbus.Error) {
	return 0, nil
}

// --- org.a11y.atspi.EditableText -----------------------------------------

func (n *atspiNode) texter() (core.AccessibleTexter, bool) {
	n.p.mu.Lock()
	obj, isText, editable := n.obj, n.isText, n.editable
	n.p.mu.Unlock()
	if !isText || !editable {
		return nil, false
	}
	tx, ok := obj.(core.AccessibleTexter)
	return tx, ok
}

func (n *atspiNode) SetTextContents(contents string) (bool, *dbus.Error) {
	tx, ok := n.texter()
	if !ok {
		return false, nil
	}
	n.p.mu.Lock()
	count := n.text.RuneLen()
	n.p.mu.Unlock()
	bellui.Do(func() {
		tx.SetAccessibleSelection(0, count)
		tx.ReplaceAccessibleSelection(contents)
	})
	return true, nil
}

func (n *atspiNode) InsertText(position int32, text string, length int32) (bool, *dbus.Error) {
	tx, ok := n.texter()
	if !ok {
		return false, nil
	}
	pos := int(position)
	bellui.Do(func() {
		tx.SetAccessibleSelection(pos, pos)
		tx.ReplaceAccessibleSelection(text)
	})
	return true, nil
}

func (n *atspiNode) DeleteText(start, end int32) (bool, *dbus.Error) {
	tx, ok := n.texter()
	if !ok {
		return false, nil
	}
	s, e := int(start), int(end)
	bellui.Do(func() {
		tx.SetAccessibleSelection(s, e)
		tx.ReplaceAccessibleSelection("")
	})
	return true, nil
}

// --- snapshot + change events (main goroutine, called from applyNodeLocked) --

// snapshotP1Locked captures the Action/Value/Text state a node exposes and
// rebuilds its advertised interface list, emitting text-changed / caret-moved
// events when the text or caret moved since the last frame. Caller holds p.mu.
func (p *atspiProvider) snapshotP1Locked(n *atspiNode, sn *accessibility.Node, events *[]func()) {
	ifaces := []string{ifaceAccessible, ifaceComponent}

	if _, ok := sn.Object.(core.AccessibleActioner); ok && len(sn.Actions) > 0 {
		n.actions = sn.Actions
		ifaces = append(ifaces, ifaceAction)
	} else {
		n.actions = nil
	}

	n.hasRange = sn.HasRange
	if sn.HasRange {
		n.valMin, n.valMax, n.valCur = sn.RangeMin, sn.RangeMax, sn.RangeCurrent
		n.valStr = sn.Value
		ifaces = append(ifaces, ifaceValue)
	}

	if tx, ok := sn.Object.(core.AccessibleTexter); ok {
		n.isText = true
		n.editable = !sn.State.Has(core.StateReadOnly) && !sn.State.Has(core.StateDisabled)
		cur := tx.AccessibleText()
		selStart, selEnd := tx.AccessibleSelection()
		caret := tx.AccessibleCaret()

		// Rebuild the rune index only when the text actually changed; the rope
		// memo returns the same string object when unchanged, so the compare is
		// a pointer-equal fast path for a static field.
		if n.text == nil || cur != n.prevText {
			deleted, inserted := textRuneDiff(n.prevText, cur)
			idx := accessibility.BuildTextIndex(cur)
			n.text = &idx
			path := n.path
			if deleted.length > 0 {
				d := deleted
				*events = append(*events, func() { p.emitTextChanged(path, false, d) })
			}
			if inserted.length > 0 {
				ins := inserted
				*events = append(*events, func() { p.emitTextChanged(path, true, ins) })
			}
			n.prevText = cur
		}
		if caret != n.caret {
			path := n.path
			*events = append(*events, func() { p.emitCaretMoved(path, caret) })
		}
		n.selStart, n.selEnd, n.caret = selStart, selEnd, caret

		ifaces = append(ifaces, ifaceText)
		if n.editable {
			ifaces = append(ifaces, ifaceEditText)
		}
	} else {
		n.isText = false
		n.editable = false
		n.text = nil
		n.prevText = ""
	}

	n.ifaces = ifaces
}

// textEdit is one insert/delete region: rune offset, length and the text.
type textEdit struct {
	offset int
	length int
	str    string
}

// textRuneDiff strips the common rune prefix and suffix and returns the removed
// and inserted regions (rune offsets/lengths), so a keystroke costs O(edit).
func textRuneDiff(prev, cur string) (deleted, inserted textEdit) {
	pr := []rune(prev)
	cu := []rune(cur)
	p := 0
	for p < len(pr) && p < len(cu) && pr[p] == cu[p] {
		p++
	}
	s := 0
	for s < len(pr)-p && s < len(cu)-p && pr[len(pr)-1-s] == cu[len(cu)-1-s] {
		s++
	}
	deleted = textEdit{offset: p, length: len(pr) - s - p, str: string(pr[p : len(pr)-s])}
	inserted = textEdit{offset: p, length: len(cu) - s - p, str: string(cu[p : len(cu)-s])}
	return deleted, inserted
}

func (p *atspiProvider) emitTextChanged(path dbus.ObjectPath, insert bool, e textEdit) {
	detail := "delete"
	if insert {
		detail = "insert"
	}
	_ = p.conn.Emit(path, ifaceEventObject+".TextChanged",
		detail, int32(e.offset), int32(e.length), dbus.MakeVariant(e.str), map[string]dbus.Variant{})
}

func (p *atspiProvider) emitCaretMoved(path dbus.ObjectPath, caret int) {
	_ = p.conn.Emit(path, ifaceEventObject+".TextCaretMoved",
		"", int32(caret), int32(0), dbus.MakeVariant(""), map[string]dbus.Variant{})
}
