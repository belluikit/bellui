package editor

import (
	"fmt"
	"strings"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/data/model"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
)

// matchPos records the location and length of a single find match. It is fully
// comparable, so it doubles as its own identity key for the [model.SelectionModel]
// that keeps the active match stable across a re-search.
type matchPos struct {
	row, col, length int
}

// findBar provides search functionality for the editor.
type findBar struct {
	bar        *bellui.Container
	entry      *findEntry
	caseSwitch *widget.Switch
	matchLabel *widget.Label

	// matches holds the current match set as a data/model source (fyne#539);
	// selection tracks the active match by identity (matchPos value) rather than
	// by row index, so re-running the search — a re-typed query, a case toggle,
	// or FindNext with a stale set — keeps the highlight on the same match
	// instead of snapping back to the first (#165).
	matches   *model.SliceModel[matchPos]
	selection *model.SelectionModel[matchPos, matchPos]

	// getText returns the current editor text.
	getText func() string
	// onSelect highlights a match in the editor: (row, col, endRow, endCol).
	onSelect func(startRow, startCol, endRow, endCol int)
	// onHide is called when the find bar requests to close.
	onHide func()
	// onMatches reports all match ranges plus the active match index whenever
	// the match set or active match changes. active is -1 when there are no
	// matches. The editor feeds these into a SearchMatchSpanDecorator.
	onMatches func(matches []TextRange, active int)
}

func newFindBar(getText func() string, onSelect func(int, int, int, int), onHide func(), onMatches func([]TextRange, int)) *findBar {
	f := &findBar{
		getText:   getText,
		onSelect:  onSelect,
		onHide:    onHide,
		onMatches: onMatches,
	}
	f.matches = model.NewSliceModel[matchPos]()
	f.selection = model.NewSelectionModel[matchPos, matchPos](f.matches, func(m matchPos) matchPos { return m })

	f.entry = newFindEntry()
	f.entry.OnChanged = func(_ string) { f.updateMatches() }
	f.entry.OnSubmitted = func(_ string) { f.FindNext() }
	f.entry.onEscape = func() { f.hide() }

	f.caseSwitch = widget.NewSwitch(func(_ bool) { f.updateMatches() })
	f.matchLabel = widget.NewLabel("")
	f.matchLabel.TextStyle = bellui.TextStyle{Monospace: true}

	prevBtn := widget.NewButtonWithIcon("", theme.MoveUpIcon(), func() { f.FindPrev() })
	prevBtn.Style = widget.ButtonStyleFilledTonal
	nextBtn := widget.NewButtonWithIcon("", theme.MoveDownIcon(), func() { f.FindNext() })
	nextBtn.Style = widget.ButtonStyleFilledTonal
	closeBtn := widget.NewButtonWithIcon("", theme.CancelIcon(), func() { f.hide() })
	closeBtn.Style = widget.ButtonStyleOutlined

	caseLabel := widget.NewLabel("Aa")
	caseLabel.TextStyle = bellui.TextStyle{Monospace: true}

	left := container.NewHBox(widget.NewIcon(theme.SearchIcon()))
	right := container.NewHBox(
		container.NewHBox(f.caseSwitch, caseLabel),
		prevBtn, nextBtn,
		f.matchLabel,
		closeBtn,
	)
	f.bar = container.NewBorder(nil, nil, left, right, f.entry)

	return f
}

func (f *findBar) hide() {
	if f.onHide != nil {
		f.onHide()
	}
}

// Content returns the find bar's container.
func (f *findBar) Content() *bellui.Container {
	return f.bar
}

// Focus sets focus to the find bar's search entry.
// If selectedText is non-empty and single-line, it pre-fills the search field.
func (f *findBar) Focus(c core.Canvas, selectedText string) {
	if selectedText != "" && !strings.Contains(selectedText, "\n") {
		f.entry.SetText(selectedText)
	}
	if c != nil {
		c.Focus(f.entry)
	}
	f.entry.TypedShortcut(&bellui.ShortcutSelectAll{})
	f.updateMatches()
}

// ClearLabel clears the match count label.
func (f *findBar) ClearLabel() {
	f.matchLabel.SetText("")
}

// currentIndex returns the display index of the active match, or -1 when no
// match is selected in the current set (empty set, or the previously active
// match filtered out by a re-search).
func (f *findBar) currentIndex() int {
	idx := f.selection.SelectedIndices()
	if len(idx) == 0 {
		return -1
	}
	return idx[0]
}

// updateMatches scans the editor text for all occurrences of the search term.
func (f *findBar) updateMatches() {
	query := f.entry.Text
	if query == "" {
		f.matches.Set(nil)
		f.selection.Clear()
		f.matchLabel.SetText("")
		f.reportMatches()
		return
	}

	text := f.getText()
	caseSensitive := f.caseSwitch.On

	var searchText, searchQuery string
	if caseSensitive {
		searchText = text
		searchQuery = query
	} else {
		searchText = strings.ToLower(text)
		searchQuery = strings.ToLower(query)
	}

	var matches []matchPos
	lines := strings.Split(text, "\n")
	searchLines := strings.Split(searchText, "\n")

	for row, sline := range searchLines {
		col := 0
		for {
			idx := strings.Index(sline[col:], searchQuery)
			if idx < 0 {
				break
			}
			runeCol := len([]rune(lines[row][:col+idx]))
			matches = append(matches, matchPos{
				row:    row,
				col:    runeCol,
				length: len([]rune(query)),
			})
			col += idx + len(searchQuery)
		}
	}

	// Swap in the new set. The selection is keyed by matchPos value, so an
	// active match that survives the re-search stays selected (and reports its
	// new index); one that vanished drops out and we fall back to the first.
	f.matches.Set(matches)
	if len(matches) == 0 {
		f.selection.Clear()
		f.matchLabel.SetText("0 results")
		f.reportMatches()
		return
	}

	if f.currentIndex() < 0 {
		f.selection.SelectIndex(0)
	}
	f.selectCurrentMatch()
}

func (f *findBar) selectCurrentMatch() {
	i := f.currentIndex()
	if i < 0 {
		return
	}
	m := f.matches.Item(i)
	f.onSelect(m.row, m.col, m.row, m.col+m.length)
	f.matchLabel.SetText(fmt.Sprintf("%d of %d", i+1, f.matches.Len()))
	f.reportMatches()
}

// matchRanges converts the current matches to half-open TextRanges (each match
// is single-line, so EndRow == StartRow).
func (f *findBar) matchRanges() []TextRange {
	n := f.matches.Len()
	if n == 0 {
		return nil
	}
	ranges := make([]TextRange, n)
	for i := 0; i < n; i++ {
		m := f.matches.Item(i)
		ranges[i] = TextRange{
			StartRow: m.row,
			StartCol: m.col,
			EndRow:   m.row,
			EndCol:   m.col + m.length,
		}
	}
	return ranges
}

// reportMatches pushes the current match ranges and active index to the editor
// so it can highlight them as spans. active is -1 when there are no matches.
func (f *findBar) reportMatches() {
	if f.onMatches == nil {
		return
	}
	f.onMatches(f.matchRanges(), f.currentIndex())
}

// FindNext moves to the next match.
func (f *findBar) FindNext() {
	if f.matches.Len() == 0 && f.entry.Text != "" {
		f.updateMatches()
	}
	n := f.matches.Len()
	if n == 0 {
		return
	}
	i := f.currentIndex() // -1 → start before the first, so we land on 0
	f.selection.SelectIndex((i + 1) % n)
	f.selectCurrentMatch()
}

// FindPrev moves to the previous match.
func (f *findBar) FindPrev() {
	if f.matches.Len() == 0 && f.entry.Text != "" {
		f.updateMatches()
	}
	n := f.matches.Len()
	if n == 0 {
		return
	}
	i := f.currentIndex()
	if i < 0 {
		i = 0
	}
	if i--; i < 0 {
		i = n - 1
	}
	f.selection.SelectIndex(i)
	f.selectCurrentMatch()
}

// HasQuery returns true if the find bar has a non-empty search term.
func (f *findBar) HasQuery() bool {
	return f.entry.Text != ""
}

// findEntry is a single-line entry with Escape and find shortcut support.
type findEntry struct {
	widget.Entry
	onEscape func()
}

func newFindEntry() *findEntry {
	e := &findEntry{}
	e.ExtendBaseWidget(e)
	return e
}

func (e *findEntry) TypedKey(ev *event.KeyEvent) {
	if ev.Name == event.KeyEscape && e.onEscape != nil {
		e.onEscape()
		return
	}
	e.Entry.TypedKey(ev)
}

// addFindShortcuts registers Cmd+F, Cmd+G, Shift+Cmd+G on this entry.
func (e *findEntry) addFindShortcuts(onFind, onFindNext, onFindPrev func()) {
	e.Entry.AddShortcut(&desktop.CustomShortcut{
		KeyName:  event.KeyF,
		Modifier: event.KeyModifierSuper,
	}, func(_ core.Shortcut) { onFind() })
	e.Entry.AddShortcut(&desktop.CustomShortcut{
		KeyName:  event.KeyG,
		Modifier: event.KeyModifierSuper,
	}, func(_ core.Shortcut) { onFindNext() })
	e.Entry.AddShortcut(&desktop.CustomShortcut{
		KeyName:  event.KeyG,
		Modifier: event.KeyModifierSuper | event.KeyModifierShift,
	}, func(_ core.Shortcut) { onFindPrev() })
}
