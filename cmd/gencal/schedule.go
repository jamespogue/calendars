package main

import (
	"sort"
	"time"
)

// dayCodes maps iCalendar BYDAY codes to Go weekdays.
var dayCodes = map[string]time.Weekday{
	"SU": time.Sunday,
	"MO": time.Monday,
	"TU": time.Tuesday,
	"WE": time.Wednesday,
	"TH": time.Thursday,
	"FR": time.Friday,
	"SA": time.Saturday,
}

// weekOrder returns the day codes sorted Monday first, so RRULEs are written consistently.
func weekOrder(days []string) []string {
	order := func(d string) int { return (int(dayCodes[d]) + 6) % 7 } // Monday=0 … Sunday=6
	out := append([]string(nil), days...)
	sort.Slice(out, func(i, j int) bool { return order(out[i]) < order(out[j]) })
	return out
}

// meetsOn reports whether a meeting pattern includes the date's weekday.
func meetsOn(days []string, d time.Time) bool {
	for _, code := range days {
		if dayCodes[code] == d.Weekday() {
			return true
		}
	}
	return false
}

// firstMeeting returns the first date on or after start that falls on one of the days.
// It returns false if there is none on or before end.
func firstMeeting(days []string, start, end time.Time) (time.Time, bool) {
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		if meetsOn(days, d) {
			return d, true
		}
	}
	return time.Time{}, false
}

// skippedDates returns, in order, every date from start through end that the meeting
// pattern would meet on but that falls inside a no-class event.
func skippedDates(days []string, start, end time.Time, events []AllDayEvent) []time.Time {
	var out []time.Time
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		if meetsOn(days, d) && noClassesOn(events, d) {
			out = append(out, d)
		}
	}
	return out
}

func noClassesOn(events []AllDayEvent, d time.Time) bool {
	for _, e := range events {
		if e.NoClasses && !d.Before(e.Start.Time) && !d.After(e.End.Time) {
			return true
		}
	}
	return false
}

// untilUTC returns the RRULE UNTIL value for the last moment of a local date.
// RFC 5545 requires UNTIL in UTC when DTSTART has a time zone.
func untilUTC(last time.Time, loc *time.Location) string {
	t := time.Date(last.Year(), last.Month(), last.Day(), 23, 59, 59, 0, loc)
	return t.UTC().Format("20060102T150405Z")
}
