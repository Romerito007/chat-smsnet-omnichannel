package entity

import (
	"testing"
	"time"
)

// namedMap / namedSlice mimic the Mongo driver's bson.M / bson.A, which are
// named types (not plain map[string]any/[]any). ParseSchedule must handle them,
// which it does by going through JSON.
type namedMap map[string]any
type namedSlice []any

func TestParseSchedule_HandlesNamedNestedTypes(t *testing.T) {
	doc := map[string]any{
		"timezone": "America/Sao_Paulo",
		"weekly": namedMap{
			"monday": namedSlice{
				namedMap{"start": "09:00", "end": "12:00"},
				namedMap{"start": "13:00", "end": "18:00"},
			},
		},
	}
	s := ParseSchedule(doc)
	if !s.Configured {
		t.Fatalf("expected configured schedule from named nested types")
	}
	if s.Timezone != "America/Sao_Paulo" {
		t.Errorf("timezone = %q", s.Timezone)
	}
	if got := len(s.Weekly[time.Monday]); got != 2 {
		t.Fatalf("expected 2 monday intervals, got %d", got)
	}
}

func TestSchedule_IsOpenAt_Boundaries(t *testing.T) {
	doc := map[string]any{
		"timezone": "UTC",
		"weekly":   map[string]any{"monday": []any{map[string]any{"start": "09:00", "end": "18:00"}}},
	}
	s := ParseSchedule(doc)
	// 2025-01-06 is Monday.
	open := func(hh, mm int) bool {
		return s.IsOpenAt(time.Date(2025, 1, 6, hh, mm, 0, 0, time.UTC))
	}
	if !open(9, 0) {
		t.Errorf("09:00 should be open (inclusive start)")
	}
	if open(18, 0) {
		t.Errorf("18:00 should be closed (exclusive end)")
	}
	if !open(17, 59) {
		t.Errorf("17:59 should be open")
	}
	if open(8, 59) {
		t.Errorf("08:59 should be closed")
	}
	// Tuesday has no window.
	if s.IsOpenAt(time.Date(2025, 1, 7, 12, 0, 0, 0, time.UTC)) {
		t.Errorf("Tuesday should be closed (no window)")
	}
}

func TestParseSchedule_EmptyIsUnconfigured(t *testing.T) {
	s := ParseSchedule(nil)
	if s.Configured {
		t.Errorf("empty doc should be unconfigured")
	}
	if !s.IsOpenAt(time.Now()) {
		t.Errorf("unconfigured schedule should always be open")
	}
}
