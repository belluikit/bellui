package tables

// SelectionMode determines how cells/rows can be selected.
type SelectionMode int

const (
	// SelectionNone disables selection.
	SelectionNone SelectionMode = iota
	// SelectionSingleRow allows selecting a single row.
	SelectionSingleRow
	// SelectionMultiRow allows selecting multiple rows via shift/ctrl click.
	SelectionMultiRow
	// SelectionSingleCell allows selecting a single cell.
	SelectionSingleCell
	// SelectionMultiCell allows selecting multiple cells: a click selects one,
	// Ctrl/Cmd-click adds or removes one, and Shift-click selects the
	// rectangle from the last clicked cell. GridAPI.SelectedCells lists them,
	// OnSelectionChanged reports the rows that hold one, and copy copies them
	// (#1401).
	SelectionMultiCell
)
