package tables

import (
	"math"
	"reflect"
	"strconv"

	"github.com/belluikit/bellui/x/tables/cell"
	"github.com/belluikit/bellui/x/tables/pipeline"
	"github.com/belluikit/bellui/x/tables/render"
)

// groupAggColumns builds the group transform's aggregates from the columns
// that declare an AggFunc, keyed by column ID (#689). It is rebuilt with the
// pipeline, so it follows column-definition changes.
func (g *DataGrid) groupAggColumns() []pipeline.AggColumn {
	var cols []pipeline.AggColumn
	for _, rc := range g.resolvedCols {
		if rc.def.AggFunc == nil {
			continue
		}
		getter := rc.def.valueGetter()
		cols = append(cols, pipeline.AggColumn{
			Field:   rc.def.ID(),
			ValueOK: func(row any) (float64, bool) { return aggNumber(getter(row)) },
			Func:    *rc.def.AggFunc,
		})
	}
	return cols
}

// aggNumber reads a cell value as a number for aggregation: any Go integer or
// float kind (named types included). Anything else, nil and "" included, has
// no value (ok false), so the aggregate skips it (#747).
func aggNumber(v any) (float64, bool) {
	if v == nil {
		return 0, false
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(rv.Int()), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return float64(rv.Uint()), true
	case reflect.Float32, reflect.Float64:
		return rv.Float(), true
	}
	return 0, false
}

// aggFuncName is the word a screen reader hears for an aggregate.
func aggFuncName(fn pipeline.AggFunc) string {
	switch fn {
	case pipeline.AggSum:
		return "sum"
	case pipeline.AggAvg:
		return "average"
	case pipeline.AggMin:
		return "minimum"
	case pipeline.AggMax:
		return "maximum"
	case pipeline.AggCount:
		return "count"
	case pipeline.AggFirst:
		return "first"
	case pipeline.AggLast:
		return "last"
	}
	return "aggregate"
}

// formatAggregate renders an aggregate for display: a count as a whole number;
// otherwise through the column's ValueFormatter, else a NumberRenderer's Format,
// else rounded to two decimals with trailing zeros dropped.
func formatAggregate(rc *resolvedColumn, v float64) string {
	if *rc.def.AggFunc == pipeline.AggCount {
		return strconv.FormatInt(int64(v), 10)
	}
	if rc.def.ValueFormatter != nil {
		return rc.def.ValueFormatter(v)
	}
	if nr, ok := rc.renderer.(*cell.NumberRenderer); ok && nr.Format != "" {
		return nr.TextForValue(v)
	}
	if math.IsInf(v, 0) || math.IsNaN(v) {
		return strconv.FormatFloat(v, 'f', -1, 64)
	}
	return strconv.FormatFloat(math.Round(v*100)/100, 'f', -1, 64)
}

// GroupRowAggregates returns the aggregates to show on a group header row, one
// per visible column with an AggFunc, in display-column order (GridAccess). It
// returns nil for a data row or when no column aggregates.
func (g *DataGrid) GroupRowAggregates(row int) []render.GroupAggregate {
	if g.groupTransform == nil {
		return nil
	}
	row += g.pageRowOffset()
	if row < 0 || row >= len(g.groupTransform.GroupRows) {
		return nil
	}
	gr := g.groupTransform.GroupRows[row]
	if !gr.IsGroup || len(gr.AggValues) == 0 {
		return nil
	}
	var out []render.GroupAggregate
	for col, rc := range g.visibleLayout().all {
		if rc.def.AggFunc == nil {
			continue
		}
		v, ok := gr.AggValues[rc.def.ID()].(float64)
		if !ok {
			continue // no value (every cell blank): the cell stays empty
		}
		text := formatAggregate(rc, v)
		out = append(out, render.GroupAggregate{
			Col:   col,
			Text:  text,
			Label: rc.def.Header() + " " + aggFuncName(*rc.def.AggFunc) + " " + text,
		})
	}
	return out
}
