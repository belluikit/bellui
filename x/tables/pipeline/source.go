package pipeline

// DataSource provides row data to the pipeline.
type DataSource interface {
	// Length returns the total number of rows.
	Length() int
	// Row returns the data for the given row index.
	Row(index int) any
}
