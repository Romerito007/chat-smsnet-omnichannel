// Package entity holds the businesshours domain entities: the weekly schedule
// (parsed from a sector's free-form business_hours document) and holidays.
package entity

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

// Interval is an open period within a day, expressed in minutes from midnight in
// the sector's local timezone (e.g. 09:00 → 540).
type Interval struct {
	StartMin int
	EndMin   int
}

// Contains reports whether minute-of-day m falls within [StartMin, EndMin).
func (i Interval) Contains(m int) bool {
	return m >= i.StartMin && m < i.EndMin
}

// Schedule is a sector's weekly business hours plus its timezone. Configured is
// false when the sector has no business hours set, in which case it is treated
// as always open (no restriction).
type Schedule struct {
	Timezone   string
	Weekly     map[time.Weekday][]Interval
	Configured bool
}

// weekdayNames maps the canonical lowercase day names to time.Weekday.
var weekdayNames = map[string]time.Weekday{
	"sunday":    time.Sunday,
	"monday":    time.Monday,
	"tuesday":   time.Tuesday,
	"wednesday": time.Wednesday,
	"thursday":  time.Thursday,
	"friday":    time.Friday,
	"saturday":  time.Saturday,
}

// rawSchedule is the JSON shape of a sector's business_hours document. Parsing
// goes through JSON so the source map can come from any decoder (the Mongo
// driver decodes nested documents/arrays as its own named types, not plain
// map[string]any/[]any).
type rawSchedule struct {
	Timezone string                   `json:"timezone"`
	Weekly   map[string][]rawInterval `json:"weekly"`
}

type rawInterval struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

// ParseSchedule reads a sector's free-form business_hours document into a
// Schedule. The expected shape is:
//
//	{
//	  "timezone": "America/Sao_Paulo",
//	  "weekly": { "monday": [{"start":"09:00","end":"18:00"}], ... }
//	}
//
// Missing/blank timezone defaults to UTC. An empty document yields an
// unconfigured schedule (always open).
func ParseSchedule(doc map[string]any) Schedule {
	s := Schedule{Timezone: "UTC", Weekly: map[time.Weekday][]Interval{}}
	if len(doc) == 0 {
		return s
	}
	b, err := json.Marshal(doc)
	if err != nil {
		return s
	}
	var raw rawSchedule
	if err := json.Unmarshal(b, &raw); err != nil {
		return s
	}
	if strings.TrimSpace(raw.Timezone) != "" {
		s.Timezone = strings.TrimSpace(raw.Timezone)
	}
	for day, intervals := range raw.Weekly {
		wd, ok := weekdayNames[strings.ToLower(strings.TrimSpace(day))]
		if !ok {
			continue
		}
		var out []Interval
		for _, iv := range intervals {
			start, sok := parseHHMM(iv.Start)
			end, eok := parseHHMM(iv.End)
			if !sok || !eok || end <= start {
				continue
			}
			out = append(out, Interval{StartMin: start, EndMin: end})
		}
		if len(out) > 0 {
			s.Weekly[wd] = out
			s.Configured = true
		}
	}
	return s
}

// IsOpenAt reports whether the schedule is open at the given local time. An
// unconfigured schedule is always open.
func (s Schedule) IsOpenAt(local time.Time) bool {
	if !s.Configured {
		return true
	}
	minute := local.Hour()*60 + local.Minute()
	for _, iv := range s.Weekly[local.Weekday()] {
		if iv.Contains(minute) {
			return true
		}
	}
	return false
}

// IntervalsOn returns the configured intervals for a weekday.
func (s Schedule) IntervalsOn(wd time.Weekday) []Interval { return s.Weekly[wd] }

// parseHHMM parses an "HH:MM" string into minutes from midnight.
func parseHHMM(str string) (int, bool) {
	parts := strings.SplitN(strings.TrimSpace(str), ":", 2)
	if len(parts) != 2 {
		return 0, false
	}
	h, err1 := strconv.Atoi(parts[0])
	m, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil || h < 0 || h > 24 || m < 0 || m > 59 {
		return 0, false
	}
	total := h*60 + m
	if total > 24*60 {
		return 0, false
	}
	return total, true
}
