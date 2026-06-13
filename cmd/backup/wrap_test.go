// regression coverage — no open roadmap item (wrapVCalendar already correct as of
// handoff 2026-06-13; these tests guard against regressions during refactors).
package main

import (
	"strings"
	"testing"
)

// TestWrapVCalendarPassthrough — content that already has a VCALENDAR envelope is
// returned unchanged.
func TestWrapVCalendarPassthrough(t *testing.T) {
	in := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VEVENT\r\nUID:x\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	if got := wrapVCalendar(in); got != in {
		t.Errorf("wrapVCalendar passthrough changed input:\n got %q\nwant %q", got, in)
	}
}

// TestWrapVCalendarWrapsVEvent — a bare VEVENT body gets a VCALENDAR envelope with
// VERSION:2.0 and the proton-moresync PRODID, exactly one VCALENDAR.
func TestWrapVCalendarWrapsVEvent(t *testing.T) {
	got := wrapVCalendar("BEGIN:VEVENT\r\nUID:x\r\nEND:VEVENT\r\n")
	if strings.Count(got, "BEGIN:VCALENDAR") != 1 {
		t.Errorf("want exactly one BEGIN:VCALENDAR, got %d in %q", strings.Count(got, "BEGIN:VCALENDAR"), got)
	}
	for _, want := range []string{"VERSION:2.0", "PRODID:-//proton-moresync", "BEGIN:VEVENT", "END:VCALENDAR"} {
		if !strings.Contains(got, want) {
			t.Errorf("wrapped output missing %q:\n%q", want, got)
		}
	}
}

// TestWrapVCalendarWrapsBareContent — content that is neither a VCALENDAR nor a VEVENT
// is wrapped in BEGIN:VEVENT/END:VEVENT and then a VCALENDAR envelope.
func TestWrapVCalendarWrapsBareContent(t *testing.T) {
	got := wrapVCalendar("SUMMARY:lonely line\r\n")
	if !strings.Contains(got, "BEGIN:VEVENT") || !strings.Contains(got, "END:VEVENT") {
		t.Errorf("bare content must be wrapped in VEVENT:\n%q", got)
	}
	if strings.Count(got, "BEGIN:VCALENDAR") != 1 {
		t.Errorf("want exactly one BEGIN:VCALENDAR, got %d", strings.Count(got, "BEGIN:VCALENDAR"))
	}
}
