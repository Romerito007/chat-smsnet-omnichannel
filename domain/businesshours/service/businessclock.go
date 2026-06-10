package service

import (
	"context"
	"sort"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/businesshours/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// maxDaysWalk bounds the day-by-day walk so a misconfigured schedule (e.g. a
// sector that is never open) can never loop forever.
const maxDaysWalk = 4000

// resolved bundles everything needed to reason about a sector's open time.
type resolved struct {
	sched    entity.Schedule
	loc      *time.Location
	holidays []*entity.Holiday
	sectorID string
}

func (s *BusinessHoursService) resolve(ctx context.Context, sectorID string) (resolved, error) {
	sector, err := s.sectors.FindByID(ctx, sectorID)
	if err != nil {
		return resolved{}, err
	}
	sched := entity.ParseSchedule(sector.BusinessHours)
	loc, lerr := time.LoadLocation(sched.Timezone)
	if lerr != nil {
		loc = time.UTC
		sched.Timezone = "UTC"
	}
	holidays, err := s.holidays.ListAll(ctx)
	if err != nil {
		return resolved{}, err
	}
	return resolved{sched: sched, loc: loc, holidays: holidays, sectorID: sectorID}, nil
}

func (r resolved) isHoliday(localDay time.Time) bool {
	for _, h := range r.holidays {
		if h.AppliesTo(r.sectorID) && h.FallsOn(localDay) {
			return true
		}
	}
	return false
}

// dayIntervals returns the sorted open instants for a given local day.
func (r resolved) dayIntervals(localDay time.Time) [][2]time.Time {
	ivs := append([]entity.Interval(nil), r.sched.IntervalsOn(localDay.Weekday())...)
	sort.Slice(ivs, func(i, j int) bool { return ivs[i].StartMin < ivs[j].StartMin })
	y, m, d := localDay.Date()
	out := make([][2]time.Time, 0, len(ivs))
	for _, iv := range ivs {
		start := time.Date(y, m, d, iv.StartMin/60, iv.StartMin%60, 0, 0, r.loc)
		end := time.Date(y, m, d, iv.EndMin/60, iv.EndMin%60, 0, 0, r.loc)
		out = append(out, [2]time.Time{start, end})
	}
	return out
}

// BusinessDurationBetween returns the open business time within [from, to].
func (s *BusinessHoursService) BusinessDurationBetween(ctx context.Context, sectorID string, from, to time.Time) (time.Duration, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return 0, err
	}
	if !to.After(from) {
		return 0, nil
	}
	r, err := s.resolve(ctx, sectorID)
	if err != nil {
		return 0, err
	}
	if !r.sched.Configured {
		return to.Sub(from), nil
	}

	fromL := from.In(r.loc)
	toL := to.In(r.loc)
	var total time.Duration
	day := startOfDay(fromL)
	for i := 0; i < maxDaysWalk && !day.After(toL); i++ {
		if !r.isHoliday(day) {
			for _, iv := range r.dayIntervals(day) {
				segStart := maxTime(iv[0], fromL)
				segEnd := minTime(iv[1], toL)
				if segEnd.After(segStart) {
					total += segEnd.Sub(segStart)
				}
			}
		}
		day = startOfDay(day.AddDate(0, 0, 1))
	}
	return total, nil
}

// AddBusinessDuration advances `from` by d of open business time.
func (s *BusinessHoursService) AddBusinessDuration(ctx context.Context, sectorID string, from time.Time, d time.Duration) (time.Time, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return time.Time{}, err
	}
	if d <= 0 {
		return from, nil
	}
	r, err := s.resolve(ctx, sectorID)
	if err != nil {
		return time.Time{}, err
	}
	if !r.sched.Configured {
		return from.Add(d), nil
	}

	remaining := d
	cursor := from.In(r.loc)
	for i := 0; i < maxDaysWalk; i++ {
		day := startOfDay(cursor)
		if !r.isHoliday(day) {
			for _, iv := range r.dayIntervals(day) {
				if !iv[1].After(cursor) {
					continue // interval already passed
				}
				segStart := maxTime(iv[0], cursor)
				avail := iv[1].Sub(segStart)
				if avail >= remaining {
					return segStart.Add(remaining), nil
				}
				remaining -= avail
			}
		}
		cursor = startOfDay(day.AddDate(0, 0, 1))
	}
	// Schedule never accrues enough open time within the cap; fall back.
	return from.Add(d), nil
}

func startOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

var _ shared.BusinessClock = (*BusinessHoursService)(nil)
