package tables

import (
	"fmt"
	"testing"
)

// With row grouping AND pagination both active, IsGroupRow/GroupRowInfo must read
// the group metadata for the row actually shown on the current page. GroupRows is
// built over the full pre-paginate output, so a page-local display row is offset
// into it by the page start; without that offset, pages after the first report a
// stale row's group state (#51).
//
// Oracle: pipeline.RowIndex.SourceRow(displayRow) is negative exactly for
// synthetic group-header rows, independent of the GroupRows-indexing path under
// test — so it is a trustworthy witness of which visible rows are headers.
func TestGroupPagination_MetadataAlignedAcrossPages(t *testing.T) {
	// Four regions, three rows each; grouped + expanded this yields 16 display
	// rows (4 headers + 12 data) interleaved as H,d,d,d per group.
	var data []map[string]any
	for _, region := range []string{"A", "B", "C", "D"} {
		for i := 0; i < 3; i++ {
			data = append(data, map[string]any{
				"region": region,
				"name":   fmt.Sprintf("%s%d", region, i),
			})
		}
	}
	g := newTestGrid(t, data, []*ColumnDef{{Field: "region"}, {Field: "name"}})

	g.API().SetRowGroupColumn("region")
	g.API().ExpandAllGroups()
	// Page size 5 deliberately straddles group boundaries, so every page after
	// the first starts on a different kind of row than page 0 does.
	g.API().EnablePagination(5)

	total := 0
	for page := 0; page < g.PageCount(); page++ {
		g.API().SetPage(page)
		for r := 0; r < g.RowCount(); r++ {
			wantHeader := g.sourceRow(r) < 0 // oracle
			if g.IsGroupRow(r) != wantHeader {
				t.Errorf("page %d row %d: IsGroupRow = %v, want %v",
					page, r, g.IsGroupRow(r), wantHeader)
			}
			if wantHeader {
				// A real header exposes its key and a non-zero child count.
				key, childCount, _, _ := g.GroupRowInfo(r)
				if key == "" || childCount == 0 {
					t.Errorf("page %d row %d: header info = (%q, %d), want non-empty key + count",
						page, r, key, childCount)
				}
			}
			total++
		}
	}
	// Sanity: we actually walked every display row (4 headers + 12 data).
	if total != 16 {
		t.Fatalf("walked %d display rows across pages, want 16", total)
	}
}
