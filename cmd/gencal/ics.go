package main

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

const chicagoTZID = "America/Chicago"

// chicagoVTIMEZONE defines America/Chicago with the US daylight saving rules in effect since 2007.
// RFC 5545 requires a VTIMEZONE for every TZID that a calendar references.
var chicagoVTIMEZONE = []string{
	"BEGIN:VTIMEZONE",
	"TZID:America/Chicago",
	"BEGIN:DAYLIGHT",
	"TZOFFSETFROM:-0600",
	"TZOFFSETTO:-0500",
	"TZNAME:CDT",
	"DTSTART:19700308T020000",
	"RRULE:FREQ=YEARLY;BYMONTH=3;BYDAY=2SU",
	"END:DAYLIGHT",
	"BEGIN:STANDARD",
	"TZOFFSETFROM:-0500",
	"TZOFFSETTO:-0600",
	"TZNAME:CST",
	"DTSTART:19701101T020000",
	"RRULE:FREQ=YEARLY;BYMONTH=11;BYDAY=1SU",
	"END:STANDARD",
	"END:VTIMEZONE",
}

// writer collects content lines and renders them with CRLF endings and RFC 5545 line folding.
type writer struct{ lines []string }

func (w *writer) add(lines ...string) { w.lines = append(w.lines, lines...) }

func (w *writer) String() string {
	var b strings.Builder
	for _, l := range w.lines {
		b.WriteString(fold(l))
		b.WriteString("\r\n")
	}
	return b.String()
}

// fold splits a content line longer than 75 octets. Each continuation line starts with a
// space, as RFC 5545 section 3.1 requires. Multi-byte UTF-8 characters are never split.
func fold(line string) string {
	const limit = 75
	if len(line) <= limit {
		return line
	}
	var b strings.Builder
	width := limit // the first line has 75 octets; continuation lines have 74 plus the space
	for len(line) > width {
		cut := width
		for cut > 0 && !utf8.RuneStart(line[cut]) {
			cut--
		}
		b.WriteString(line[:cut])
		b.WriteString("\r\n ")
		line = line[cut:]
		width = limit - 1
	}
	b.WriteString(line)
	return b.String()
}

// escapeText escapes a TEXT value (SUMMARY, DESCRIPTION, LOCATION) per RFC 5545 section 3.3.11.
func escapeText(s string) string {
	return strings.NewReplacer(`\`, `\\`, ";", `\;`, ",", `\,`, "\n", `\n`).Replace(s)
}

func dateValue(d time.Time) string { return d.Format("20060102") }

func localValue(d time.Time, c Clock) string {
	return fmt.Sprintf("%sT%02d%02d00", dateValue(d), c.Hour, c.Minute)
}

func calendarHeader(w *writer, prodID, name string) {
	w.add(
		"BEGIN:VCALENDAR",
		"VERSION:2.0",
		"PRODID:"+prodID,
		"CALSCALE:GREGORIAN",
		"METHOD:PUBLISH",
		"X-WR-CALNAME:"+escapeText(name),
	)
}

// termCalendar renders a term's all-day events.
func termCalendar(t Term, defaultStamp string) string {
	var w writer
	calendarHeader(&w, t.Calendar.ProdID, t.Calendar.Name)
	for _, e := range t.Events {
		uid, stamp := e.UID, e.DTStamp
		if uid == "" {
			uid = fmt.Sprintf("%s-%s-%s@lewisu-holidays", t.ID, dateValue(e.Start.Time), slug(e.Summary))
		}
		if stamp == "" {
			stamp = defaultStamp
		}
		w.add(
			"BEGIN:VEVENT",
			"UID:"+uid,
			"DTSTAMP:"+stamp,
			"DTSTART;VALUE=DATE:"+dateValue(e.Start.Time),
			"DTEND;VALUE=DATE:"+dateValue(e.End.AddDate(0, 0, 1)), // DTEND is exclusive
			"SUMMARY:"+escapeText(e.Summary),
			"TRANSP:TRANSPARENT",
			"STATUS:CONFIRMED",
			"END:VEVENT",
		)
	}
	w.add("END:VCALENDAR")
	return w.String()
}

// courseCalendar renders a course's weekly meetings (skipping no-class days) and its final exam.
func courseCalendar(c Course, t Term, loc *time.Location) (string, error) {
	var w writer
	calendarHeader(&w, fmt.Sprintf("-//Lewis University//%s %s-%s %s//EN", c.Subject, c.Number, c.Section, c.Title),
		fmt.Sprintf("%s %s %s", t.Label, c.Code(), c.Title))
	w.add("X-WR-TIMEZONE:" + chicagoTZID)
	w.add(chicagoVTIMEZONE...)

	base := strings.ToLower(fmt.Sprintf("%s-%s", t.ID, c.Code()))
	description := ""
	if c.CRN != "" {
		description = "CRN " + c.CRN
	}

	for i, m := range c.Meetings {
		first, ok := firstMeeting(m.Days, t.ClassesStart.Time, t.ClassesEnd.Time)
		if !ok {
			return "", fmt.Errorf("%s: meeting %d never occurs between classesStart and classesEnd", c.Code(), i+1)
		}
		uid := base + "-class"
		if i > 0 {
			uid = fmt.Sprintf("%s-class-%d", base, i+1)
		}
		w.add(
			"BEGIN:VEVENT",
			"UID:"+uid+"@lewisu.edu",
			"DTSTAMP:"+c.DTStamp,
			fmt.Sprintf("DTSTART;TZID=%s:%s", chicagoTZID, localValue(first, m.Start)),
			fmt.Sprintf("DTEND;TZID=%s:%s", chicagoTZID, localValue(first, m.End)),
			fmt.Sprintf("RRULE:FREQ=WEEKLY;BYDAY=%s;UNTIL=%s", strings.Join(weekOrder(m.Days), ","), untilUTC(t.ClassesEnd.Time, loc)),
		)
		if skipped := skippedDates(m.Days, first, t.ClassesEnd.Time, t.Events); len(skipped) > 0 {
			values := make([]string, len(skipped))
			for j, d := range skipped {
				values[j] = localValue(d, m.Start)
			}
			w.add(fmt.Sprintf("EXDATE;TZID=%s:%s", chicagoTZID, strings.Join(values, ",")))
		}
		addDetails(&w, c.Title, description, m.Location, c.ReminderMinutes)
		w.add("END:VEVENT")
	}

	if f := c.Final; f != nil {
		summary := c.Title + " Final Exam"
		w.add(
			"BEGIN:VEVENT",
			"UID:"+base+"-final@lewisu.edu",
			"DTSTAMP:"+c.DTStamp,
			fmt.Sprintf("DTSTART;TZID=%s:%s", chicagoTZID, localValue(f.Date.Time, f.Start)),
			fmt.Sprintf("DTEND;TZID=%s:%s", chicagoTZID, localValue(f.Date.Time, f.End)),
		)
		addDetails(&w, summary, description, f.Location, c.ReminderMinutes)
		w.add("END:VEVENT")
	}

	w.add("END:VCALENDAR")
	return w.String(), nil
}

// addDetails writes SUMMARY, optional DESCRIPTION and LOCATION, and the reminder alarm.
func addDetails(w *writer, summary, description, location string, reminderMinutes int) {
	w.add("SUMMARY:" + escapeText(summary))
	if description != "" {
		w.add("DESCRIPTION:" + escapeText(description))
	}
	if location != "" {
		w.add("LOCATION:" + escapeText(location))
	}
	if reminderMinutes > 0 {
		w.add(
			"BEGIN:VALARM",
			fmt.Sprintf("TRIGGER:-PT%dM", reminderMinutes),
			"ACTION:DISPLAY",
			fmt.Sprintf("DESCRIPTION:%s starts in %d minutes", escapeText(summary), reminderMinutes),
			"END:VALARM",
		)
	}
}

// slug turns "Spring Break" into "spring-break" for generated UIDs.
func slug(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			dash = false
		case !dash && b.Len() > 0:
			b.WriteByte('-')
			dash = true
		}
	}
	return strings.TrimSuffix(b.String(), "-")
}
