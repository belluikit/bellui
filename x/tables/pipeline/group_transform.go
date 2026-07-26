package pipeline

// GroupRow represents either a group header or a data row in the grouped output.
type GroupRow struct {
	// IsGroup is true for group header rows.
	IsGroup bool
	// GroupKey is the value the rows are grouped by (for group headers).
	GroupKey any
	// GroupField is the field name used for grouping.
	GroupField string
	// ChildCount is the number of child rows in this group.
	ChildCount int
	// Depth is the nesting depth (0 = top-level group).
	Depth int
	// Expanded is true if the group is expanded (children visible).
	Expanded bool
	// SourceIndex is the original source row index (for data rows).
	SourceIndex int
	// AggValues holds computed aggregations for group rows.
	AggValues map[string]any
}

// GroupTransform groups rows by a key extracted via ValueGetter.
// It inserts group header rows and optionally hides children of collapsed groups.
type GroupTransform struct {
	// ValueGetter extracts the group key from a row.
	ValueGetter func(rowData any) any
	// Field is the field name being grouped.
	Field string
	// Collapsed tracks which group keys are collapsed.
	Collapsed map[string]bool
	// GroupRows is the output list of group/data rows, used by the renderer
	// to distinguish group headers from data rows.
	GroupRows []GroupRow
}

// NewGroupTransform creates a group transform.
func NewGroupTransform(field string, valueGetter func(rowData any) any) *GroupTransform {
	return &GroupTransform{
		Field:       field,
		ValueGetter: valueGetter,
		Collapsed:   make(map[string]bool),
	}
}

func (t *GroupTransform) Apply(indices []int, source DataSource) []int {
	if t.ValueGetter == nil {
		return indices
	}

	// Group rows by key, preserving order of first occurrence
	type group struct {
		key     any
		keyStr  string
		indices []int
	}
	var groups []*group
	groupMap := make(map[string]*group)

	for _, idx := range indices {
		row := source.Row(idx)
		key := t.ValueGetter(row)
		keyStr := formatGroupKey(key)

		g, ok := groupMap[keyStr]
		if !ok {
			g = &group{key: key, keyStr: keyStr}
			groups = append(groups, g)
			groupMap[keyStr] = g
		}
		g.indices = append(g.indices, idx)
	}

	// Build output: group header + (expanded children)
	var result []int
	t.GroupRows = nil

	for _, g := range groups {
		expanded := !t.Collapsed[g.keyStr]

		// Group header row (uses -1 - groupIndex as a sentinel)
		groupIdx := -(len(t.GroupRows) + 1)
		t.GroupRows = append(t.GroupRows, GroupRow{
			IsGroup:    true,
			GroupKey:   g.key,
			GroupField: t.Field,
			ChildCount: len(g.indices),
			Depth:      0,
			Expanded:   expanded,
		})
		result = append(result, groupIdx)

		if expanded {
			for _, idx := range g.indices {
				t.GroupRows = append(t.GroupRows, GroupRow{
					IsGroup:     false,
					SourceIndex: idx,
					Depth:       1,
				})
				result = append(result, idx)
			}
		}
	}

	return result
}

// ToggleGroup toggles the collapsed state of a group.
func (t *GroupTransform) ToggleGroup(keyStr string) {
	t.Collapsed[keyStr] = !t.Collapsed[keyStr]
}

// IsCollapsed returns true if the given group is collapsed.
func (t *GroupTransform) IsCollapsed(keyStr string) bool {
	return t.Collapsed[keyStr]
}

// ExpandAll expands all groups.
func (t *GroupTransform) ExpandAll() {
	t.Collapsed = make(map[string]bool)
}

// CollapseAll collapses all groups.
func (t *GroupTransform) CollapseAll(source DataSource, indices []int) {
	if t.ValueGetter == nil {
		return
	}
	seen := make(map[string]bool)
	for _, idx := range indices {
		row := source.Row(idx)
		key := t.ValueGetter(row)
		keyStr := formatGroupKey(key)
		if !seen[keyStr] {
			t.Collapsed[keyStr] = true
			seen[keyStr] = true
		}
	}
}

func formatGroupKey(key any) string {
	if key == nil {
		return "<nil>"
	}
	if s, ok := key.(string); ok {
		return s
	}
	return stringOf(key)
}

func stringOf(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case int:
		return intToStr(val)
	case float64:
		return floatToStr(val)
	case bool:
		if val {
			return "true"
		}
		return "false"
	}
	return "<unknown>"
}

func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf) - 1
	for n > 0 {
		buf[i] = byte('0' + n%10)
		n /= 10
		i--
	}
	if neg {
		buf[i] = '-'
		i--
	}
	return string(buf[i+1:])
}

func floatToStr(f float64) string {
	// Simple formatting — adequate for group keys
	if f == float64(int(f)) {
		return intToStr(int(f))
	}
	// Fallback
	return stringOf(int(f*100)/100) + "." + intToStr(int(f*100)%100)
}
