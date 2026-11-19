package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Term describes one academic term: its class dates and its all-day events.
// Events marked NoClasses are skipped by every course in the term.
type Term struct {
	ID           string        `json:"id"`    // e.g. "2027-sp"; prefixes every generated file name
	Label        string        `json:"label"` // e.g. "SP27"; prefixes calendar display names
	Name         string        `json:"name"`  // e.g. "Spring 2027"
	Timezone     string        `json:"timezone"`
	ClassesStart Date          `json:"classesStart"`
	ClassesEnd   Date          `json:"classesEnd"` // last day of regular class meetings (before finals)
	Calendar     TermCalendar  `json:"calendar"`
	Events       []AllDayEvent `json:"events"`
}

// TermCalendar names the term's academic calendar file.
type TermCalendar struct {
	File   string `json:"file"`
	ProdID string `json:"prodid"`
	Name   string `json:"name"`
}

// AllDayEvent is a holiday, break, or other date range. Start and End are inclusive.
// UID and DTStamp are set for events that were published before the generator existed,
// so subscribers keep seeing the same events.
type AllDayEvent struct {
	Summary   string `json:"summary"`
	Start     Date   `json:"start"`
	End       Date   `json:"end"`
	NoClasses bool   `json:"noClasses"`
	UID       string `json:"uid"`
	DTStamp   string `json:"dtstamp"`
}

// Course is one section that gets its own calendar file.
type Course struct {
	Term            string    `json:"term"`
	Subject         string    `json:"subject"` // e.g. "CPSC"
	Number          string    `json:"number"`  // e.g. "44000"
	Section         string    `json:"section"` // e.g. "001"
	CRN             string    `json:"crn"`
	Title           string    `json:"title"`
	DTStamp         string    `json:"dtstamp"`
	ReminderMinutes int       `json:"reminderMinutes"`
	Meetings        []Meeting `json:"meetings"`
	Final           *Final    `json:"final"`
}

// Meeting is a weekly class meeting pattern.
type Meeting struct {
	Days     []string `json:"days"` // iCalendar day codes: MO TU WE TH FR SA SU
	Start    Clock    `json:"start"`
	End      Clock    `json:"end"`
	Location string   `json:"location"`
}

// Final is a course's final exam.
type Final struct {
	Date     Date   `json:"date"`
	Start    Clock  `json:"start"`
	End      Clock  `json:"end"`
	Location string `json:"location"`
}

// Code returns the course code used in names and file names, e.g. "CPSC-44000-001".
func (c Course) Code() string {
	return fmt.Sprintf("%s-%s-%s", c.Subject, c.Number, c.Section)
}

// FileName returns the generated calendar file name, e.g. "2027-sp-cpsc-44000-001.ics".
func (c Course) FileName() string {
	return fmt.Sprintf("%s-%s.ics", c.Term, strings.ToLower(c.Code()))
}

// Date is a calendar date with no time or zone, written as "2006-01-02" in JSON.
// It is stored as midnight UTC so that adding days never crosses a DST change.
type Date struct{ time.Time }

func (d *Date) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		return fmt.Errorf("date %q: want YYYY-MM-DD", s)
	}
	d.Time = t
	return nil
}

// Clock is a time of day, written as "15:04" (24-hour) in JSON.
type Clock struct{ Hour, Minute int }

func (c *Clock) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	t, err := time.Parse("15:04", s)
	if err != nil {
		return fmt.Errorf("time %q: want HH:MM (24-hour)", s)
	}
	c.Hour, c.Minute = t.Hour(), t.Minute()
	return nil
}

func (c Clock) minutes() int { return c.Hour*60 + c.Minute }

// loadJSON decodes one file strictly: unknown fields are errors, which catches typos.
func loadJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

// loadAll reads data/terms/*.json and data/courses/*.json and validates them.
func loadAll(dataDir string) (map[string]Term, []Course, error) {
	termPaths, err := filepath.Glob(filepath.Join(dataDir, "terms", "*.json"))
	if err != nil {
		return nil, nil, err
	}
	terms := make(map[string]Term)
	for _, p := range termPaths {
		var t Term
		if err := loadJSON(p, &t); err != nil {
			return nil, nil, err
		}
		if err := t.validate(); err != nil {
			return nil, nil, fmt.Errorf("%s: %w", p, err)
		}
		if _, dup := terms[t.ID]; dup {
			return nil, nil, fmt.Errorf("%s: duplicate term id %q", p, t.ID)
		}
		terms[t.ID] = t
	}

	coursePaths, err := filepath.Glob(filepath.Join(dataDir, "courses", "*.json"))
	if err != nil {
		return nil, nil, err
	}
	var courses []Course
	for _, p := range coursePaths {
		var c Course
		if err := loadJSON(p, &c); err != nil {
			return nil, nil, err
		}
		if _, ok := terms[c.Term]; !ok {
			return nil, nil, fmt.Errorf("%s: unknown term %q", p, c.Term)
		}
		if err := c.validate(terms[c.Term]); err != nil {
			return nil, nil, fmt.Errorf("%s: %w", p, err)
		}
		if want := strings.TrimSuffix(c.FileName(), ".ics") + ".json"; filepath.Base(p) != want {
			return nil, nil, fmt.Errorf("%s: file should be named %s to match its contents", p, want)
		}
		courses = append(courses, c)
	}
	sort.Slice(courses, func(i, j int) bool { return courses[i].FileName() < courses[j].FileName() })
	return terms, courses, nil
}

func (t Term) validate() error {
	var errs []error
	if t.ID == "" || t.Label == "" || t.Name == "" {
		errs = append(errs, errors.New("id, label, and name are required"))
	}
	if t.Timezone != chicagoTZID {
		errs = append(errs, fmt.Errorf("timezone must be %q (the only zone with a built-in definition)", chicagoTZID))
	}
	if t.ClassesStart.IsZero() || t.ClassesEnd.Before(t.ClassesStart.Time) {
		errs = append(errs, errors.New("classesStart is required and must not be after classesEnd"))
	}
	if t.Calendar.File == "" || t.Calendar.ProdID == "" || t.Calendar.Name == "" {
		errs = append(errs, errors.New("calendar.file, calendar.prodid, and calendar.name are required"))
	}
	for i, e := range t.Events {
		if e.Summary == "" || e.Start.IsZero() || e.End.Before(e.Start.Time) {
			errs = append(errs, fmt.Errorf("events[%d]: summary and start are required, and end must not be before start", i))
		}
	}
	return errors.Join(errs...)
}

func (c Course) validate(t Term) error {
	var errs []error
	if c.Subject == "" || c.Number == "" || c.Section == "" || c.Title == "" {
		errs = append(errs, errors.New("subject, number, section, and title are required"))
	}
	if c.DTStamp == "" {
		errs = append(errs, errors.New("dtstamp is required (UTC, e.g. 20261119T000000Z)"))
	}
	if c.ReminderMinutes < 0 {
		errs = append(errs, errors.New("reminderMinutes must not be negative"))
	}
	if len(c.Meetings) == 0 {
		errs = append(errs, errors.New("at least one meeting is required"))
	}
	for i, m := range c.Meetings {
		if len(m.Days) == 0 {
			errs = append(errs, fmt.Errorf("meetings[%d]: days are required", i))
		}
		for _, d := range m.Days {
			if _, ok := dayCodes[d]; !ok {
				errs = append(errs, fmt.Errorf("meetings[%d]: unknown day %q (use MO TU WE TH FR SA SU)", i, d))
			}
		}
		if m.End.minutes() <= m.Start.minutes() {
			errs = append(errs, fmt.Errorf("meetings[%d]: end must be after start", i))
		}
	}
	if f := c.Final; f != nil {
		if f.Date.IsZero() || f.End.minutes() <= f.Start.minutes() {
			errs = append(errs, errors.New("final: date is required and end must be after start"))
		}
		if f.Date.Before(t.ClassesStart.Time) {
			errs = append(errs, errors.New("final: date is before the term's classes start"))
		}
	}
	return errors.Join(errs...)
}
