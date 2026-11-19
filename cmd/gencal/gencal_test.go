package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func date(s string) time.Time {
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		panic(err)
	}
	return t
}

func dates(ts []time.Time) []string {
	out := make([]string, len(ts))
	for i, t := range ts {
		out[i] = t.Format(time.DateOnly)
	}
	return out
}

// The spring 2027 no-class events, as in data/terms/2027-sp.json.
var spring2027 = []AllDayEvent{
	{Summary: "MLK", Start: Date{date("2027-01-18")}, End: Date{date("2027-01-18")}, NoClasses: true},
	{Summary: "Start", Start: Date{date("2027-01-19")}, End: Date{date("2027-01-19")}},
	{Summary: "Spring Break", Start: Date{date("2027-03-15")}, End: Date{date("2027-03-19")}, NoClasses: true},
	{Summary: "Easter", Start: Date{date("2027-03-25")}, End: Date{date("2027-03-29")}, NoClasses: true},
}

func TestSkippedDates(t *testing.T) {
	tests := []struct {
		name string
		days []string
		want []string
	}{
		{"tuesday-thursday", []string{"TU", "TH"}, []string{"2027-03-16", "2027-03-18", "2027-03-25"}},
		{"monday-wednesday-friday", []string{"MO", "WE", "FR"}, []string{"2027-03-15", "2027-03-17", "2027-03-19", "2027-03-26", "2027-03-29"}},
		{"wednesday only", []string{"WE"}, []string{"2027-03-17"}},
		{"saturday only", []string{"SA"}, []string{"2027-03-27"}}, // Easter break runs Thursday–Monday
		{"sunday only", []string{"SU"}, []string{"2027-03-28"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dates(skippedDates(tt.days, date("2027-01-19"), date("2027-05-08"), spring2027))
			if strings.Join(got, ",") != strings.Join(tt.want, ",") {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFirstMeeting(t *testing.T) {
	tests := []struct {
		days []string
		want string
	}{
		{[]string{"TU", "TH"}, "2027-01-19"}, // classes start on a Tuesday
		{[]string{"MO", "WE"}, "2027-01-20"},
		{[]string{"MO"}, "2027-01-25"},
	}
	for _, tt := range tests {
		got, ok := firstMeeting(tt.days, date("2027-01-19"), date("2027-05-08"))
		if !ok || got.Format(time.DateOnly) != tt.want {
			t.Errorf("firstMeeting(%v) = %v, %v; want %s", tt.days, got.Format(time.DateOnly), ok, tt.want)
		}
	}
	if _, ok := firstMeeting([]string{"MO"}, date("2027-01-19"), date("2027-01-20")); ok {
		t.Error("firstMeeting found a Monday in a Tuesday–Wednesday range")
	}
}

func TestUntilUTC(t *testing.T) {
	loc, err := time.LoadLocation(chicagoTZID)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct{ last, want string }{
		{"2027-05-08", "20270509T045959Z"}, // CDT, UTC-5
		{"2026-12-05", "20261206T055959Z"}, // CST, UTC-6
	}
	for _, tt := range tests {
		if got := untilUTC(date(tt.last), loc); got != tt.want {
			t.Errorf("untilUTC(%s) = %s, want %s", tt.last, got, tt.want)
		}
	}
}

func TestWeekOrder(t *testing.T) {
	got := strings.Join(weekOrder([]string{"SU", "TH", "MO", "TU"}), ",")
	if want := "MO,TU,TH,SU"; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestFold(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"short line unchanged", "SUMMARY:Short", "SUMMARY:Short"},
		{"exactly 75 octets unchanged", strings.Repeat("a", 75), strings.Repeat("a", 75)},
		{"76 octets folds", strings.Repeat("a", 76), strings.Repeat("a", 75) + "\r\n a"},
		{"continuations hold 74", strings.Repeat("a", 75+74+1), strings.Repeat("a", 75) + "\r\n " + strings.Repeat("a", 74) + "\r\n a"},
		{"never splits a multi-byte rune", strings.Repeat("a", 74) + "é", strings.Repeat("a", 74) + "\r\n é"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fold(tt.in); got != tt.want {
				t.Errorf("fold() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEscapeText(t *testing.T) {
	got := escapeText("Room 101, Science; A\\B\nnext")
	if want := `Room 101\, Science\; A\\B\nnext`; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSlug(t *testing.T) {
	if got := slug("Easter Holiday Break!"); got != "easter-holiday-break" {
		t.Errorf("got %q", got)
	}
}

// TestPublishedTermCalendarsUnchanged regenerates every term calendar and compares it with the
// published file byte for byte, so subscribers never see unintended changes.
func TestPublishedTermCalendarsUnchanged(t *testing.T) {
	terms, _, err := loadAll("../../data")
	if err != nil {
		t.Fatal(err)
	}
	for _, term := range terms {
		published, err := os.ReadFile(filepath.Join("../..", term.Calendar.File))
		if err != nil {
			t.Fatal(err)
		}
		if got := termCalendar(term, ""); got != string(published) {
			t.Errorf("%s: generated output differs from the published file", term.Calendar.File)
		}
	}
}

func TestCourseCalendar(t *testing.T) {
	loc, err := time.LoadLocation(chicagoTZID)
	if err != nil {
		t.Fatal(err)
	}
	term := Term{ID: "2027-sp", Label: "SP27", ClassesStart: Date{date("2027-01-19")}, ClassesEnd: Date{date("2027-05-08")}, Events: spring2027}
	course := Course{
		Term: "2027-sp", Subject: "CPSC", Number: "44000", Section: "001", CRN: "12345",
		Title: "Software Engineering", DTStamp: "20261119T000000Z", ReminderMinutes: 15,
		Meetings: []Meeting{{Days: []string{"TH", "TU"}, Start: Clock{9, 30}, End: Clock{10, 45}, Location: "AS-101, Romeoville"}},
		Final:    &Final{Date: Date{date("2027-05-11")}, Start: Clock{8, 0}, End: Clock{10, 0}},
	}
	got, err := courseCalendar(course, term, loc)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"X-WR-CALNAME:SP27 CPSC-44000-001 Software Engineering\r\n",
		"BEGIN:VTIMEZONE\r\n",
		"UID:2027-sp-cpsc-44000-001-class@lewisu.edu\r\n",
		"DTSTART;TZID=America/Chicago:20270119T093000\r\n",
		"DTEND;TZID=America/Chicago:20270119T104500\r\n",
		"RRULE:FREQ=WEEKLY;BYDAY=TU,TH;UNTIL=20270509T045959Z\r\n",
		"EXDATE;TZID=America/Chicago:20270316T093000,20270318T093000,20270325T093000\r\n",
		"DESCRIPTION:CRN 12345\r\n",
		`LOCATION:AS-101\, Romeoville` + "\r\n",
		"TRIGGER:-PT15M\r\n",
		"UID:2027-sp-cpsc-44000-001-final@lewisu.edu\r\n",
		"DTSTART;TZID=America/Chicago:20270511T080000\r\n",
		"SUMMARY:Software Engineering Final Exam\r\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("calendar is missing %q", strings.TrimSpace(want))
		}
	}
	if strings.Contains(strings.ReplaceAll(got, "\r\n", ""), "\n") {
		t.Error("calendar contains a bare LF; every line must end in CRLF")
	}
}

func TestCourseValidation(t *testing.T) {
	term := Term{ClassesStart: Date{date("2027-01-19")}}
	bad := Course{
		Meetings: []Meeting{{Days: []string{"XX"}, Start: Clock{10, 0}, End: Clock{9, 0}}},
		Final:    &Final{Date: Date{date("2026-12-01")}, Start: Clock{8, 0}, End: Clock{10, 0}},
	}
	err := bad.validate(term)
	if err == nil {
		t.Fatal("expected validation errors")
	}
	for _, want := range []string{"title are required", "dtstamp is required", `unknown day "XX"`, "end must be after start", "before the term's classes start"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

func TestRunCheckReportsStaleFiles(t *testing.T) {
	out := t.TempDir()
	var stdout bytes.Buffer
	if err := run([]string{"-data", "../../data", "-out", out, "-check"}, &stdout); err == nil {
		t.Fatal("-check passed with an empty output directory")
	}
	if err := run([]string{"-data", "../../data", "-out", out}, &stdout); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"-data", "../../data", "-out", out, "-check"}, &stdout); err != nil {
		t.Fatalf("-check failed right after generating: %v", err)
	}
}
