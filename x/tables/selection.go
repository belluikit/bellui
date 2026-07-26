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
	// SelectionMultiCell allows selecting multiple cells.
	SelectionMultiCell
)
