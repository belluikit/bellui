package filter

import (
	"testing"
	"time"
)

func TestDateFilter_Equals(t *testing.T) {
	d := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)
	f := &DateFilter{Mode: DateEquals, Date: d}

	if !f.Matches(d) {
		t.Error("expected same date to match")
	}
	if f.Matches(d.AddDate(0, 0, 1)) {
		t.Error("expected different date to not match")
	}
}

func TestDateFilter_Before(t *testing.T) {
	d := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)
	f := &DateFilter{Mode: DateBefore, Date: d}

	if !f.Matches(d.AddDate(0, 0, -1)) {
		t.Error("expected earlier date to match")
	}
	if f.Matches(d.AddDate(0, 0, 1)) {
		t.Error("expected later date to not match")
	}
}

func TestDateFilter_After(t *testing.T) {
	d := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)
	f := &DateFilter{Mode: DateAfter, Date: d}

	if !f.Matches(d.AddDate(0, 0, 1)) {
		t.Error("expected later date to match")
	}
	if f.Matches(d.AddDate(0, 0, -1)) {
		t.Error("expected earlier date to not match")
	}
}

func TestDateFilter_InRange(t *testing.T) {
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)
	f := &DateFilter{Mode: DateInRange, Date: start, ToDate: end}

	if !f.Matches(time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)) {
		t.Error("expected mid-year to be in range")
	}
	if f.Matches(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Error("expected next year to be out of range")
	}
}

func TestDateFilter_StringInput(t *testing.T) {
	d := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)
	f := &DateFilter{Mode: DateEquals, Date: d}

	if !f.Matches("2024-06-15") {
		t.Error("expected string date to match")
	}
}
