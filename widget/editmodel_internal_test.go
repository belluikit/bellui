package widget

import (
	"errors"
	"testing"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/data/binding"
	"github.com/belluikit/bellui/data/model"
	"github.com/belluikit/bellui/test"

	"github.com/stretchr/testify/assert"
)

func passThroughEditor() CellEditor[string] {
	return CellEditor[string]{
		Text:   func(s string) string { return s },
		Commit: func(_ string, text string) (string, error) { return text, nil },
	}
}

func newEditableList(t *testing.T, m model.MutableModel[string], editor CellEditor[string]) (*List, *ListEditor[string], func()) {
	t.Helper()
	list := NewList(nil, nil, nil)
	ed, unbind := BindListEditing[string](
		list, m,
		func() core.CanvasObject { return NewLabel("") },
		func(s string, o core.CanvasObject) { o.(*Label).SetText(s) },
		editor,
	)
	return list, ed, unbind
}

func TestBindListEditing_ActivateEditCommit(t *testing.T) {
	test.NewTempApp(t)
	m := model.NewSliceModel("a", "b", "c")
	list, ed, unbind := newEditableList(t, m, passThroughEditor())
	defer unbind()

	tmpl := list.CreateItem()

	// Activation (selection) begins editing.
	list.OnSelected(1)
	if _, editing := ed.Editing(); !assert.True(t, editing) {
		return
	}

	list.UpdateItem(1, tmpl) // renders the inline editor and seeds it
	display, entry, ok := templateParts(tmpl)
	assert.True(t, ok)
	assert.False(t, entry.Hidden, "editor shown for the editing row")
	assert.True(t, display.Visible() == false, "display hidden while editing")
	assert.Equal(t, "b", entry.Text, "editor seeded from the item")

	// User edits, then commits.
	entry.SetText("B")
	assert.NoError(t, ed.Commit())
	assert.Equal(t, "B", m.Item(1), "commit wrote the model")
	if _, editing := ed.Editing(); !assert.False(t, editing) {
		return
	}
}

func TestBindListEditing_CancelDiscards(t *testing.T) {
	test.NewTempApp(t)
	m := model.NewSliceModel("a", "b", "c")
	list, ed, unbind := newEditableList(t, m, passThroughEditor())
	defer unbind()

	tmpl := list.CreateItem()
	ed.Edit(0)
	list.UpdateItem(0, tmpl)
	_, entry, _ := templateParts(tmpl)
	entry.SetText("changed")

	ed.Cancel()
	assert.Equal(t, "a", m.Item(0), "cancel must not write the model")
	if _, editing := ed.Editing(); !assert.False(t, editing) {
		return
	}
}

func TestBindListEditing_ValidationRejectsAndKeepsOpen(t *testing.T) {
	test.NewTempApp(t)
	m := model.NewSliceModel("a", "b", "c")
	editor := CellEditor[string]{
		Text: func(s string) string { return s },
		Commit: func(_ string, text string) (string, error) {
			if text == "" {
				return "", errors.New("must not be empty")
			}
			return text, nil
		},
	}
	list, ed, unbind := newEditableList(t, m, editor)
	defer unbind()

	tmpl := list.CreateItem()
	ed.Edit(1)
	list.UpdateItem(1, tmpl)
	_, entry, _ := templateParts(tmpl)
	entry.SetText("")

	err := ed.Commit()
	assert.Error(t, err, "invalid edit is rejected")
	assert.Equal(t, "b", m.Item(1), "rejected edit must not write the model")
	if _, editing := ed.Editing(); !assert.True(t, editing, "editor stays open on rejection") {
		return
	}

	// A subsequent valid edit commits normally.
	entry.SetText("ok")
	assert.NoError(t, ed.Commit())
	assert.Equal(t, "ok", m.Item(1))
}

func TestBindListEditing_CommitNotifiesOnce(t *testing.T) {
	test.NewTempApp(t)
	m := model.NewSliceModel("a", "b", "c")
	list, ed, unbind := newEditableList(t, m, passThroughEditor())
	defer unbind()

	count, fired := 0, false
	m.AddListener(binding.NewDataListener(func() {
		if fired {
			count++
		}
		fired = true
	}))

	tmpl := list.CreateItem()
	ed.Edit(0)
	list.UpdateItem(0, tmpl)
	_, entry, _ := templateParts(tmpl)
	entry.SetText("A")
	assert.NoError(t, ed.Commit())

	assert.Equal(t, 1, count, "one gated edit fires exactly one model notification")
}

func TestBindListEditing_WritesThroughSortProxy(t *testing.T) {
	test.NewTempApp(t)
	src := model.NewSliceModel("banana", "apple", "cherry")
	sorted := model.NewSortProxy[string](src)
	sorted.SetLess(func(a, b string) bool { return a < b })

	list, ed, unbind := newEditableList(t, sorted, passThroughEditor())
	defer unbind()

	tmpl := list.CreateItem()
	ed.Edit(0) // displayed row 0 is "apple"
	list.UpdateItem(0, tmpl)
	_, entry, _ := templateParts(tmpl)
	assert.Equal(t, "apple", entry.Text)
	entry.SetText("apricot")
	assert.NoError(t, ed.Commit())

	// The correct source row (index 1, "apple") was rewritten.
	assert.Equal(t, "apricot", src.Item(1))
}

func TestBindTableEditing_ActivateEditCommit(t *testing.T) {
	test.NewTempApp(t)
	type row struct{ a, b string }
	m := model.NewSliceModel(row{"a0", "b0"}, row{"a1", "b1"})

	table := NewTable(nil, nil, nil)
	editors := []CellEditor[row]{
		{}, // column 0 not editable (nil Commit)
		{
			Text:   func(r row) string { return r.b },
			Commit: func(r row, text string) (row, error) { r.b = text; return r, nil },
		},
	}
	ed, unbind := BindTableEditing[row](
		table, m, 2,
		func() core.CanvasObject { return NewLabel("") },
		func(r row, col int, o core.CanvasObject) {
			if col == 0 {
				o.(*Label).SetText(r.a)
			} else {
				o.(*Label).SetText(r.b)
			}
		},
		editors,
	)
	defer unbind()

	tmpl := table.CreateCell()

	// Column 0 is not editable: activation there does not begin an edit.
	table.OnSelected(TableCellID{Row: 0, Col: 0})
	if _, editing := ed.Editing(); !assert.False(t, editing) {
		return
	}

	// Column 1 is editable.
	table.OnSelected(TableCellID{Row: 1, Col: 1})
	if _, editing := ed.Editing(); !assert.True(t, editing) {
		return
	}
	table.UpdateCell(TableCellID{Row: 1, Col: 1}, tmpl)
	_, entry, _ := templateParts(tmpl)
	assert.Equal(t, "b1", entry.Text)
	entry.SetText("edited")
	assert.NoError(t, ed.Commit())

	assert.Equal(t, "edited", m.Item(1).b)
	assert.Equal(t, "a1", m.Item(1).a, "other column untouched")
}

func TestBindTableEditing_CancelDiscards(t *testing.T) {
	test.NewTempApp(t)
	type row struct{ v string }
	m := model.NewSliceModel(row{"x"})
	table := NewTable(nil, nil, nil)
	editors := []CellEditor[row]{{
		Text:   func(r row) string { return r.v },
		Commit: func(r row, text string) (row, error) { r.v = text; return r, nil },
	}}
	ed, unbind := BindTableEditing[row](
		table, m, 1,
		func() core.CanvasObject { return NewLabel("") },
		func(r row, _ int, o core.CanvasObject) { o.(*Label).SetText(r.v) },
		editors,
	)
	defer unbind()

	tmpl := table.CreateCell()
	ed.Edit(TableCellID{Row: 0, Col: 0})
	table.UpdateCell(TableCellID{Row: 0, Col: 0}, tmpl)
	_, entry, _ := templateParts(tmpl)
	entry.SetText("changed")
	ed.Cancel()
	assert.Equal(t, "x", m.Item(0).v, "cancel must not write")
	if _, editing := ed.Editing(); !assert.False(t, editing) {
		return
	}
}
