// Package entity holds the businesshours domain entities: the weekly schedule
// (parsed from a channel's free-form business_hours document) and holidays.
package entity

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Interval is an open period within a day, expressed in minutes from midnight in
// the channel's local timezone (e.g. 09:00 → 540). Intervals never cross midnight
// (EndMin > StartMin); model an overnight shift as two days.
type Interval struct {
	StartMin int
	EndMin   int
}

// Contains reports whether minute-of-day m falls within [StartMin, EndMin).
func (i Interval) Contains(m int) bool {
	return m >= i.StartMin && m < i.EndMin
}

// Schedule is a channel's weekly business hours plus its timezone. Configured is
// false when the channel has no business hours set, in which case it is treated
// as always open (24/7, no restriction).
type Schedule struct {
	Timezone   string
	Weekly     map[time.Weekday][]Interval
	Configured bool
}

// rawSchedule is the JSON shape of a channel's business_hours document:
//
//	{
//	  "timezone": "America/Sao_Paulo",
//	  "weekly": [
//	    { "day": 1, "intervals": [ {"start":"08:00","end":"12:00"}, {"start":"13:00","end":"18:00"} ] },
//	    { "day": 0, "intervals": [] }
//	  ]
//	}
//
// day is 0=Sunday..6=Saturday; a day with no intervals (or absent) is closed.
// Parsing goes through JSON so the source map can come from any decoder (the Mongo
// driver decodes nested documents/arrays as its own named types).
type rawSchedule struct {
	Timezone string   `json:"timezone"`
	Weekly   []rawDay `json:"weekly"`
}

type rawDay struct {
	Day       int           `json:"day"`
	Intervals []rawInterval `json:"intervals"`
}

type rawInterval struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

// ParseSchedule reads a channel's free-form business_hours document into a
// Schedule, leniently (invalid days/intervals are skipped). Missing/blank
// timezone defaults to UTC. An empty/invalid document yields an unconfigured
// schedule (always open). Use ValidateSchedule for the write path.
func ParseSchedule(doc map[string]any) Schedule {
	s := Schedule{Timezone: "UTC", Weekly: map[time.Weekday][]Interval{}}
	raw, ok := decodeRaw(doc)
	if !ok {
		return s
	}
	if tz := strings.TrimSpace(raw.Timezone); tz != "" {
		s.Timezone = tz
	}
	for _, day := range raw.Weekly {
		if day.Day < 0 || day.Day > 6 {
			continue
		}
		var out []Interval
		for _, iv := range day.Intervals {
			start, sok := parseHHMM(iv.Start)
			end, eok := parseHHMM(iv.End)
			if !sok || !eok || end <= start {
				continue
			}
			out = append(out, Interval{StartMin: start, EndMin: end})
		}
		if len(out) > 0 {
			s.Weekly[time.Weekday(day.Day)] = out
			s.Configured = true
		}
	}
	return s
}

// ValidateSchedule strictly validates a business_hours document for the write
// path: timezone must be loadable; each day in [0,6]; each interval "HH:MM" with
// end > start (no overnight crossing); intervals within a day must not overlap. An
// empty document is valid (means 24/7).
func ValidateSchedule(doc map[string]any) error {
	if len(doc) == 0 {
		return nil
	}
	raw, ok := decodeRaw(doc)
	if !ok {
		return fmt.Errorf("business_hours: malformed document")
	}
	tz := strings.TrimSpace(raw.Timezone)
	if tz == "" {
		tz = "UTC"
	}
	if _, err := time.LoadLocation(tz); err != nil {
		return fmt.Errorf("business_hours: unknown timezone %q", tz)
	}
	seen := map[int]bool{}
	for _, day := range raw.Weekly {
		if day.Day < 0 || day.Day > 6 {
			return fmt.Errorf("business_hours: day must be 0..6, got %d", day.Day)
		}
		if seen[day.Day] {
			return fmt.Errorf("business_hours: day %d repeated", day.Day)
		}
		seen[day.Day] = true
		ivs := make([]Interval, 0, len(day.Intervals))
		for _, iv := range day.Intervals {
			start, sok := parseHHMM(iv.Start)
			end, eok := parseHHMM(iv.End)
			if !sok {
				return fmt.Errorf("business_hours: day %d invalid start %q", day.Day, iv.Start)
			}
			if !eok {
				return fmt.Errorf("business_hours: day %d invalid end %q", day.Day, iv.End)
			}
			if end <= start {
				return fmt.Errorf("business_hours: day %d interval end must be after start (no overnight crossing)", day.Day)
			}
			ivs = append(ivs, Interval{StartMin: start, EndMin: end})
		}
		sort.Slice(ivs, func(i, j int) bool { return ivs[i].StartMin < ivs[j].StartMin })
		for i := 1; i < len(ivs); i++ {
			if ivs[i].StartMin < ivs[i-1].EndMin {
				return fmt.Errorf("business_hours: day %d intervals overlap", day.Day)
			}
		}
	}
	return nil
}

func decodeRaw(doc map[string]any) (rawSchedule, bool) {
	if len(doc) == 0 {
		return rawSchedule{}, false
	}
	b, err := json.Marshal(doc)
	if err != nil {
		return rawSchedule{}, false
	}
	var raw rawSchedule
	if err := json.Unmarshal(b, &raw); err != nil {
		return rawSchedule{}, false
	}
	return raw, true
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
