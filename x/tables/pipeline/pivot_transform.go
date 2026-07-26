package pipeline

import "fmt"

// PivotConfig configures a pivot operation.
type PivotConfig struct {
	// RowGroupField is the field used to group rows (e.g. "region").
	RowGroupField  string
	RowGroupGetter func(rowData any) any

	// PivotField is the field whose unique values become new columns (e.g. "product").
	PivotField  string
	PivotGetter func(rowData any) any

	// ValueField is the field to aggregate (e.g. "revenue").
	ValueField  string
	ValueGetter func(rowData any) float64

	// AggFunc is the aggregation function to apply.
	AggFunc AggFunc
}

// PivotResult holds the output of a pivot operation.
type PivotResult struct {
	// PivotColumns are the dynamically-generated column names from unique pivot values.
	PivotColumns []string
	// Rows are the pivoted row data, keyed by row group value.
	// Each row is a map: {"_rowGroup": groupVal, pivotCol1: aggVal, pivotCol2: aggVal, ...}
	Rows []map[string]any
}

// PivotTransform pivots the data, producing a new set of virtual rows with
// dynamically-generated columns.
type PivotTransform struct {
	Config PivotConfig
	Result *PivotResult
}

// NewPivotTransform creates a pivot transform.
func NewPivotTransform(config PivotConfig) *PivotTransform {
	return &PivotTransform{Config: config}
}

func (t *PivotTransform) Apply(indices []int, source DataSource) []int {
	cfg := t.Config
	if cfg.RowGroupGetter == nil || cfg.PivotGetter == nil || cfg.ValueGetter == nil {
		return indices
	}

	// Collect unique pivot values and group data
	type cellAcc struct {
		values []float64
	}

	// Map: rowGroupKey -> (pivotKey -> accumulated values)
	type rowData struct {
		groupKey any
		cells    map[string]*cellAcc
	}

	pivotKeysSet := make(map[string]bool)
	var pivotKeysOrder []string
	rowGroups := make(map[string]*rowData)
	var rowGroupOrder []string

	for _, idx := range indices {
		if idx < 0 {
			continue
		}
		row := source.Row(idx)
		rgKey := cfg.RowGroupGetter(row)
		pvKey := cfg.PivotGetter(row)
		val := cfg.ValueGetter(row)

		rgStr := fmt.Sprint(rgKey)
		pvStr := fmt.Sprint(pvKey)

		if !pivotKeysSet[pvStr] {
			pivotKeysSet[pvStr] = true
			pivotKeysOrder = append(pivotKeysOrder, pvStr)
		}

		rg, ok := rowGroups[rgStr]
		if !ok {
			rg = &rowData{groupKey: rgKey, cells: make(map[string]*cellAcc)}
			rowGroups[rgStr] = rg
			rowGroupOrder = append(rowGroupOrder, rgStr)
		}
		cell, ok := rg.cells[pvStr]
		if !ok {
			cell = &cellAcc{}
			rg.cells[pvStr] = cell
		}
		cell.values = append(cell.values, val)
	}

	// Build pivot columns
	pivotCols := make([]string, len(pivotKeysOrder))
	for i, pk := range pivotKeysOrder {
		pivotCols[i] = pk + "_" + cfg.ValueField
	}

	// Build pivoted rows
	rows := make([]map[string]any, 0, len(rowGroupOrder))
	for _, rgStr := range rowGroupOrder {
		rg := rowGroups[rgStr]
		rowMap := map[string]any{
			"_rowGroup":       rg.groupKey,
			cfg.RowGroupField: rg.groupKey,
		}
		for i, pk := range pivotKeysOrder {
			colName := pivotCols[i]
			cell := rg.cells[pk]
			if cell != nil {
				rowMap[colName] = computeAgg(cfg.AggFunc, cell.values)
			} else {
				rowMap[colName] = 0.0
			}
		}
		rows = append(rows, rowMap)
	}

	t.Result = &PivotResult{
		PivotColumns: pivotCols,
		Rows:         rows,
	}

	// Return indices into the pivot result (0..len-1)
	result := make([]int, len(rows))
	for i := range result {
		result[i] = i
	}
	return result
}

// PivotDataSource wraps a PivotResult as a DataSource so the grid can display it.
type PivotDataSource struct {
	Result *PivotResult
}

func (s *PivotDataSource) Length() int {
	if s.Result == nil {
		return 0
	}
	return len(s.Result.Rows)
}

func (s *PivotDataSource) Row(index int) any {
	if s.Result == nil || index < 0 || index >= len(s.Result.Rows) {
		return nil
	}
	return s.Result.Rows[index]
}
