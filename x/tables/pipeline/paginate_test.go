package pipeline

import "testing"

func TestPaginateTransform_Basic(t *testing.T) {
	indices := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	src := NewSliceSource(nil) // not used by paginate

	pt := NewPaginateTransform(3)

	// Page 0: [0, 1, 2]
	result := pt.Apply(indices, src)
	if len(result) != 3 {
		t.Fatalf("expected 3 rows on page 0, got %d", len(result))
	}
	if result[0] != 0 || result[2] != 2 {
		t.Errorf("expected [0,1,2], got %v", result)
	}

	// Page 1: [3, 4, 5]
	pt.SetPage(1)
	result = pt.Apply(indices, src)
	if len(result) != 3 || result[0] != 3 {
		t.Errorf("page 1: expected [3,4,5], got %v", result)
	}

	// Page 3 (last): [9]
	pt.SetPage(3)
	result = pt.Apply(indices, src)
	if len(result) != 1 || result[0] != 9 {
		t.Errorf("page 3: expected [9], got %v", result)
	}
}

func TestPaginateTransform_PageCount(t *testing.T) {
	pt := NewPaginateTransform(3)

	if pt.PageCount(10) != 4 {
		t.Errorf("expected 4 pages for 10 rows with pageSize 3, got %d", pt.PageCount(10))
	}
	if pt.PageCount(9) != 3 {
		t.Errorf("expected 3 pages for 9 rows, got %d", pt.PageCount(9))
	}
	if pt.PageCount(0) != 0 {
		t.Errorf("expected 0 pages for 0 rows, got %d", pt.PageCount(0))
	}
}

func TestPaginateTransform_OutOfBounds(t *testing.T) {
	indices := []int{0, 1, 2}
	src := NewSliceSource(nil)

	pt := NewPaginateTransform(10)
	pt.SetPage(5) // way beyond

	result := pt.Apply(indices, src)
	// Should clamp to page 0
	if len(result) != 3 {
		t.Errorf("expected all 3 rows on clamped page, got %d", len(result))
	}
}

func TestPaginateTransform_ZeroPageSize(t *testing.T) {
	indices := []int{0, 1, 2}
	src := NewSliceSource(nil)

	pt := NewPaginateTransform(0)
	result := pt.Apply(indices, src)
	if len(result) != 3 {
		t.Errorf("zero page size should return all rows, got %d", len(result))
	}
}

func TestPaginateTransform_NegativePage(t *testing.T) {
	pt := NewPaginateTransform(5)
	pt.SetPage(-3)
	if pt.CurrentPage != 0 {
		t.Errorf("expected page clamped to 0, got %d", pt.CurrentPage)
	}
}
