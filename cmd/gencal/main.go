// Command gencal generates the repository's .ics calendar files from the JSON files in data/.
//
// Usage:
//
//	go run ./cmd/gencal          # write calendars
//	go run ./cmd/gencal -check   # report calendars that are out of date; exit 1 if any
//
// Term files (data/terms/*.json) hold a term's class dates and all-day events, and produce the
// term's academic calendar. Course files (data/courses/*.json) produce one calendar per section;
// their meetings automatically skip every term event marked "noClasses".
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "gencal:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("gencal", flag.ContinueOnError)
	dataDir := fs.String("data", "data", "directory containing terms/ and courses/")
	outDir := fs.String("out", ".", "directory to write .ics files to")
	check := fs.Bool("check", false, "don't write; report out-of-date calendars and exit 1 if any")
	if err := fs.Parse(args); err != nil {
		return err
	}

	loc, err := time.LoadLocation(chicagoTZID)
	if err != nil {
		return fmt.Errorf("loading time zone: %w", err)
	}
	terms, courses, err := loadAll(*dataDir)
	if err != nil {
		return err
	}

	outputs := make(map[string]string) // file name → contents
	for _, t := range terms {
		outputs[t.Calendar.File] = termCalendar(t, "")
	}
	for _, c := range courses {
		ics, err := courseCalendar(c, terms[c.Term], loc)
		if err != nil {
			return err
		}
		if _, dup := outputs[c.FileName()]; dup {
			return fmt.Errorf("two data files generate %s", c.FileName())
		}
		outputs[c.FileName()] = ics
	}

	names := make([]string, 0, len(outputs))
	for name := range outputs {
		names = append(names, name)
	}
	sort.Strings(names) // deterministic output order

	var stale []string
	for _, name := range names {
		contents := outputs[name]
		path := filepath.Join(*outDir, name)
		current, err := os.ReadFile(path)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if bytes.Equal(current, []byte(contents)) {
			continue
		}
		if *check {
			stale = append(stale, name)
			continue
		}
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			return err
		}
		fmt.Fprintln(stdout, "wrote", path)
	}
	if len(stale) > 0 {
		return fmt.Errorf("out of date (run go run ./cmd/gencal): %v", stale)
	}
	return nil
}
