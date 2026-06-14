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
		"weekly": namedSlice{
			namedMap{"day": 1, "intervals": namedSlice{
				namedMap{"start": "09:00", "end": "12:00"},
				namedMap{"start": "13:00", "end": "18:00"},
			}},
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
		"weekly": []any{map[string]any{
			"day":       1,
			"intervals": []any{map[string]any{"start": "09:00", "end": "18:00"}},
		}},
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

func TestValidateSchedule(t *testing.T) {
	day := func(d int, ivs ...map[string]any) map[string]any {
		anyIvs := make([]any, len(ivs))
		for i, iv := range ivs {
			anyIvs[i] = iv
		}
		return map[string]any{"day": d, "intervals": anyIvs}
	}
	iv := func(s, e string) map[string]any { return map[string]any{"start": s, "end": e} }

	cases := []struct {
		name    string
		doc     map[string]any
		wantErr bool
	}{
		{"empty is valid (24/7)", nil, false},
		{
			"valid mon with lunch",
			map[string]any{"timezone": "America/Sao_Paulo", "weekly": []any{
				day(1, iv("09:00", "12:00"), iv("13:00", "18:00")),
			}},
			false,
		},
		{
			"unknown timezone",
			map[string]any{"timezone": "Mars/Phobos", "weekly": []any{day(1, iv("09:00", "18:00"))}},
			true,
		},
		{
			"day out of range",
			map[string]any{"weekly": []any{day(7, iv("09:00", "18:00"))}},
			true,
		},
		{
			"repeated day",
			map[string]any{"weekly": []any{day(1, iv("09:00", "12:00")), day(1, iv("13:00", "18:00"))}},
			true,
		},
		{
			"invalid start",
			map[string]any{"weekly": []any{day(1, iv("9h", "18:00"))}},
			true,
		},
		{
			"end before start (overnight not supported)",
			map[string]any{"weekly": []any{day(1, iv("22:00", "02:00"))}},
			true,
		},
		{
			"overlapping intervals",
			map[string]any{"weekly": []any{day(1, iv("09:00", "13:00"), iv("12:00", "18:00"))}},
			true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateSchedule(c.doc)
			if (err != nil) != c.wantErr {
				t.Errorf("ValidateSchedule err=%v, wantErr=%v", err, c.wantErr)
			}
		})
	}
}
