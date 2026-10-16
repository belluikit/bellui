package widget

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/data/binding"
	"github.com/belluikit/bellui/data/model"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/layout"
)

// CellEditor describes how an inline editor exchanges a string with a model item
// for the editing delegates ([BindListEditing] / [BindTableEditing]). Values are
// edited through a single-line [Entry]: Text seeds the editor when editing begins
// and Commit turns the edited text back into an item.
//
// Since: 1.0
type CellEditor[T any] struct {
	// Text returns the string to seed the editor with when editing begins.
	Text func(item T) string

	// Commit parses the edited text back into an updated item. Returning a
	// non-nil error rejects the edit as invalid: the editor stays open and the
	// model is not written, backing validation. When Commit is nil the item is
	// not editable (editing that cell/row is a no-op).
	Commit func(item T, text string) (T, error)

	// Validator, if set, is attached to the editor Entry for live in-field
	// validation feedback while typing. It does not gate the commit — only
	// Commit's returned error does.
	Validator bellui.StringValidator
}

// editEntry is the inline editor: a single-line Entry that commits on Enter or
// focus loss and cancels on Escape, routed back to the owning controller.
type editEntry struct {
	Entry
	onCommit func()
	onCancel func()
}

func newEditEntry() *editEntry {
	e := &editEntry{}
	e.ExtendBaseWidget(e)
	return e
}

// HandlesEscape reports true: Escape cancels the in-place edit, which is the
// innermost thing the user can back out of.
func (e *editEntry) HandlesEscape() bool { return true }

func (e *editEntry) TypedKey(key *event.KeyEvent) {
	if key.Name == event.KeyEscape {
		if e.onCancel != nil {
			e.onCancel()
		}
		return
	}
	e.Entry.TypedKey(key)
}

func (e *editEntry) FocusLost() {
	e.Entry.FocusLost()
	if e.onCommit != nil {
		e.onCommit()
	}
}

// newEditTemplate builds a cell/row template that stacks a display object over a
// (hidden) inline editor; the delegate shows exactly one at a time.
func newEditTemplate(createDisplay func() core.CanvasObject) *bellui.Container {
	display := createDisplay()
	entry := newEditEntry()
	entry.Hide()
	return &bellui.Container{
		Layout:  layout.NewStackLayout(),
		Objects: []core.CanvasObject{display, entry},
	}
}

func templateParts(o core.CanvasObject) (display core.CanvasObject, entry *editEntry, ok bool) {
	c, ok := o.(*bellui.Container)
	if !ok || len(c.Objects) != 2 {
		return nil, nil, false
	}
	entry, ok = c.Objects[1].(*editEntry)
	if !ok {
		return nil, nil, false
	}
	return c.Objects[0], entry, true
}

// focusEditor best-effort moves keyboard focus to the editor once it is on a
// canvas. Outside a running driver (unit tests) it is a harmless no-op.
func focusEditor(e *editEntry) {
	if app := bellui.CurrentApp(); app != nil {
		if d := app.Driver(); d != nil {
			if c := d.CanvasForObject(e); c != nil {
				c.Focus(e)
			}
		}
	}
}

// ListEditor is the controller returned by [BindListEditing]. It drives an inline
// editor over the rows of a [List]: begin editing a row, commit the edit back
// through the model as one gated write, or cancel it.
//
// Call all methods on the main goroutine, like every widget mutation.
//
// Since: 1.0
type ListEditor[T any] struct {
	// EditOnActivate begins editing a row when it is selected (the default). Set
	// it to false to drive editing yourself — for example from a
	// [model.SelectionModel], by calling [ListEditor.Edit] with the selected row.
	EditOnActivate bool

	list   *List
	m      model.MutableModel[T]
	editor CellEditor[T]

	editing int        // display index being edited, or -1 when idle
	item    T          // snapshot of the item captured when editing began
	active  *editEntry // the on-screen editor for the editing row, or nil
	busy    bool       // guards reentrancy from Refresh-driven focus changes
	unbindL func()     // detaches the model change listener
}

// BindListEditing drives an editable [List] from a [model.MutableModel]: rows
// render through createDisplay/updateDisplay, and activating a row swaps its
// display for an inline editor described by editor. Committing writes the edited
// value back through the model as one gated edit (a single [model.MutableModel.SetItem],
// so it fires one change notification); an invalid edit (Commit returns an error)
// is rejected and the editor stays open; Escape cancels and discards.
//
// Because m may be a [model.SortProxy]/[model.FilterProxy] (or a chain), the write
// is mapped from the displayed row back to the correct source row automatically.
//
// The returned [ListEditor] is the controller; the returned function detaches the
// binding. Mutate the model on the main goroutine (use [bellui.Do] from a
// background goroutine), the same rule the widgets already follow.
//
// Since: 1.0
func BindListEditing[T any](list *List, m model.MutableModel[T],
	createDisplay func() core.CanvasObject,
	updateDisplay func(item T, display core.CanvasObject),
	editor CellEditor[T],
) (*ListEditor[T], func()) {
	ed := &ListEditor[T]{
		EditOnActivate: true,
		list:           list,
		m:              m,
		editor:         editor,
		editing:        -1,
	}

	list.Length = m.Len
	list.CreateItem = func() core.CanvasObject { return newEditTemplate(createDisplay) }
	list.UpdateItem = func(id ListItemID, o core.CanvasObject) {
		display, entry, ok := templateParts(o)
		if !ok || id < 0 || id >= m.Len() {
			return
		}
		item := m.Item(id)
		if ed.editing == id && editor.Commit != nil {
			display.Hide()
			entry.Show()
			if ed.active != entry {
				entry.Validator = editor.Validator
				if editor.Text != nil {
					entry.SetText(editor.Text(item))
				} else {
					entry.SetText("")
				}
				entry.OnSubmitted = func(string) { _ = ed.Commit() }
				entry.onCommit = func() { _ = ed.Commit() }
				entry.onCancel = ed.Cancel
				ed.active = entry
				focusEditor(entry)
			}
			return
		}
		entry.Hide()
		display.Show()
		updateDisplay(item, display)
	}

	prev := list.OnSelected
	list.OnSelected = func(id ListItemID) {
		if prev != nil {
			prev(id)
		}
		if ed.EditOnActivate {
			ed.Edit(id)
		}
	}

	l := binding.NewDataListener(list.Refresh)
	m.AddListener(l) // fires immediately, rendering current contents
	ed.unbindL = func() { m.RemoveListener(l) }

	return ed, func() {
		ed.unbindL()
		list.OnSelected = prev
	}
}

// Edit begins inline editing of the row at display index row (a no-op if the row
// is out of range or the editor has no Commit). It replaces any edit in progress.
func (ed *ListEditor[T]) Edit(row int) {
	if ed.editor.Commit == nil || row < 0 || row >= ed.m.Len() {
		return
	}
	ed.editing = row
	ed.item = ed.m.Item(row)
	ed.active = nil // force the next UpdateItem to (re)seed the editor
	ed.list.Refresh()
}

// Commit validates the current edit and, if it is accepted, writes it back
// through the model as one gated edit and closes the editor. It returns the
// validation error from the editor's Commit when the edit is rejected (the editor
// stays open in that case), or nil when there was nothing to commit or the write
// succeeded.
func (ed *ListEditor[T]) Commit() error {
	if ed.busy || ed.editing < 0 || ed.active == nil || ed.editor.Commit == nil {
		return nil
	}
	row := ed.editing
	newItem, err := ed.editor.Commit(ed.item, ed.active.Text)
	if err != nil {
		return err // rejected: keep editing
	}
	ed.busy = true
	ed.editing = -1
	ed.active = nil
	ed.m.SetItem(row, newItem) // one write + one notification -> Refresh
	ed.busy = false
	ed.list.Refresh() // ensure teardown even if the model write was a no-op
	return nil
}

// Cancel discards the current edit and closes the editor.
func (ed *ListEditor[T]) Cancel() {
	if ed.editing < 0 {
		return
	}
	ed.editing = -1
	ed.active = nil
	ed.list.Refresh()
}

// Editing reports the row currently being edited, and whether an edit is active.
func (ed *ListEditor[T]) Editing() (row int, ok bool) {
	return ed.editing, ed.editing >= 0
}

// TableEditor is the controller returned by [BindTableEditing]. It drives an
// inline editor over the cells of a [Table]. See [ListEditor] for the semantics;
// the only difference is that a cell is addressed by [TableCellID] and each
// column carries its own [CellEditor].
//
// Call all methods on the main goroutine.
//
// Since: 1.0
type TableEditor[T any] struct {
	// EditOnActivate begins editing a cell when it is selected (the default). Set
	// it to false to drive editing yourself via [TableEditor.Edit].
	EditOnActivate bool

	table   *Table
	m       model.MutableModel[T]
	cols    int
	editors []CellEditor[T]

	editing TableCellID
	active2 bool // whether an edit is in progress (editing is otherwise ambiguous)
	item    T
	entry   *editEntry
	busy    bool
	unbindL func()
}

// BindTableEditing drives an editable [Table] from a [model.MutableModel] whose
// rows are the model items and whose columns are fixed at cols. Cells render
// through createDisplay/updateDisplay; editors is indexed by column, and a column
// is editable only when its [CellEditor] has a non-nil Commit (columns past the
// end of editors are not editable). Committing writes the edited value back
// through the model as one gated edit; validation and cancel behave as in
// [BindListEditing].
//
// The returned [TableEditor] is the controller; the returned function detaches
// the binding.
//
// Since: 1.0
func BindTableEditing[T any](t *Table, m model.MutableModel[T], cols int,
	createDisplay func() core.CanvasObject,
	updateDisplay func(item T, col int, display core.CanvasObject),
	editors []CellEditor[T],
) (*TableEditor[T], func()) {
	ed := &TableEditor[T]{
		EditOnActivate: true,
		table:          t,
		m:              m,
		cols:           cols,
		editors:        editors,
	}

	t.Length = func() (int, int) { return m.Len(), cols }
	t.CreateCell = func() core.CanvasObject { return newEditTemplate(createDisplay) }
	t.UpdateCell = func(id TableCellID, o core.CanvasObject) {
		display, entry, ok := templateParts(o)
		if !ok || id.Row < 0 || id.Row >= m.Len() || id.Col < 0 || id.Col >= cols {
			return
		}
		item := m.Item(id.Row)
		editor := ed.editorFor(id.Col)
		if ed.active2 && ed.editing == id && editor != nil {
			display.Hide()
			entry.Show()
			if ed.entry != entry {
				entry.Validator = editor.Validator
				if editor.Text != nil {
					entry.SetText(editor.Text(item))
				} else {
					entry.SetText("")
				}
				entry.OnSubmitted = func(string) { _ = ed.Commit() }
				entry.onCommit = func() { _ = ed.Commit() }
				entry.onCancel = ed.Cancel
				ed.entry = entry
				focusEditor(entry)
			}
			return
		}
		entry.Hide()
		display.Show()
		updateDisplay(item, id.Col, display)
	}

	prev := t.OnSelected
	t.OnSelected = func(id TableCellID) {
		if prev != nil {
			prev(id)
		}
		if ed.EditOnActivate {
			ed.Edit(id)
		}
	}

	l := binding.NewDataListener(t.Refresh)
	m.AddListener(l)
	ed.unbindL = func() { m.RemoveListener(l) }

	return ed, func() {
		ed.unbindL()
		t.OnSelected = prev
	}
}

func (ed *TableEditor[T]) editorFor(col int) *CellEditor[T] {
	if col < 0 || col >= len(ed.editors) || ed.editors[col].Commit == nil {
		return nil
	}
	return &ed.editors[col]
}

// Edit begins inline editing of the given cell (a no-op if the cell is out of
// range or its column is not editable). It replaces any edit in progress.
func (ed *TableEditor[T]) Edit(id TableCellID) {
	if ed.editorFor(id.Col) == nil || id.Row < 0 || id.Row >= ed.m.Len() {
		return
	}
	ed.editing = id
	ed.active2 = true
	ed.item = ed.m.Item(id.Row)
	ed.entry = nil
	ed.table.Refresh()
}

// Commit validates the current edit and, if accepted, writes it back through the
// model as one gated edit and closes the editor. See [ListEditor.Commit].
func (ed *TableEditor[T]) Commit() error {
	if ed.busy || !ed.active2 || ed.entry == nil {
		return nil
	}
	editor := ed.editorFor(ed.editing.Col)
	if editor == nil {
		return nil
	}
	row := ed.editing.Row
	newItem, err := editor.Commit(ed.item, ed.entry.Text)
	if err != nil {
		return err
	}
	ed.busy = true
	ed.active2 = false
	ed.entry = nil
	ed.m.SetItem(row, newItem)
	ed.busy = false
	ed.table.Refresh()
	return nil
}

// Cancel discards the current edit and closes the editor.
func (ed *TableEditor[T]) Cancel() {
	if !ed.active2 {
		return
	}
	ed.active2 = false
	ed.entry = nil
	ed.table.Refresh()
}

// Editing reports the cell currently being edited, and whether an edit is active.
func (ed *TableEditor[T]) Editing() (id TableCellID, ok bool) {
	return ed.editing, ed.active2
}
