package calendar

import (
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-ical"
)

// kyiv is the configured zone for most tests; it is UTC+3 in September 2026.
var kyiv = mustZone("Europe/Kyiv")

func mustZone(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return loc
}

// wednesday is the test "today": 2026-09-16 in Kyiv.
func wednesday() (from, to time.Time) {
	from = time.Date(2026, 9, 16, 0, 0, 0, 0, kyiv)
	return from, from.AddDate(0, 0, 1)
}

// ics wraps VEVENT bodies in a minimal VCALENDAR.
func ics(events ...string) string {
	return "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//test//EN\r\n" +
		strings.Join(events, "") + "END:VCALENDAR\r\n"
}

// vevent builds one VEVENT from property lines.
func vevent(lines ...string) string {
	return "BEGIN:VEVENT\r\n" + strings.Join(lines, "\r\n") + "\r\nEND:VEVENT\r\n"
}

func parseICS(t *testing.T, body string) *ical.Calendar {
	t.Helper()
	cal, err := ical.NewDecoder(strings.NewReader(body)).Decode()
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	return cal
}

// today parses the fixture and returns the events overlapping the test day.
func today(t *testing.T, body string) []Event {
	t.Helper()
	from, to := wednesday()
	return eventsForDay(parseICS(t, body), "Work", from, to, kyiv)
}

func at(y int, m time.Month, d, hh, mm int) time.Time {
	return time.Date(y, m, d, hh, mm, 0, 0, kyiv)
}

func TestDayWindow_UsesConfiguredZone(t *testing.T) {
	// 23:30 UTC on the 16th is already 02:30 on the 17th in Kyiv.
	now := time.Date(2026, 9, 16, 23, 30, 0, 0, time.UTC)
	from, to := dayWindow(now, kyiv)
	if want := at(2026, 9, 17, 0, 0); !from.Equal(want) {
		t.Errorf("from = %v, want %v", from, want)
	}
	if want := at(2026, 9, 18, 0, 0); !to.Equal(want) {
		t.Errorf("to = %v, want %v", to, want)
	}
	if from.Location() != kyiv {
		t.Errorf("from location = %v, want Kyiv", from.Location())
	}
}

func TestEventsForDay_TimedEventInsideDay(t *testing.T) {
	got := today(t, ics(vevent(
		"UID:a1",
		"DTSTART;TZID=Europe/Kyiv:20260916T090000",
		"DTEND;TZID=Europe/Kyiv:20260916T103000",
		"SUMMARY:Standup",
	)))
	if len(got) != 1 {
		t.Fatalf("got %d events, want 1: %+v", len(got), got)
	}
	e := got[0]
	if e.Title != "Standup" || e.UID != "a1" || e.Calendar != "Work" || e.AllDay {
		t.Errorf("event = %+v", e)
	}
	if !e.Start.Equal(at(2026, 9, 16, 9, 0)) || !e.End.Equal(at(2026, 9, 16, 10, 30)) {
		t.Errorf("times = %v - %v", e.Start, e.End)
	}
}

func TestEventsForDay_ExcludesOtherDays(t *testing.T) {
	got := today(t, ics(
		vevent("UID:y", "DTSTART;TZID=Europe/Kyiv:20260915T090000", "DTEND;TZID=Europe/Kyiv:20260915T100000", "SUMMARY:Yesterday"),
		vevent("UID:t", "DTSTART;TZID=Europe/Kyiv:20260917T090000", "DTEND;TZID=Europe/Kyiv:20260917T100000", "SUMMARY:Tomorrow"),
	))
	if len(got) != 0 {
		t.Fatalf("got %d events, want 0: %+v", len(got), got)
	}
}

func TestEventsForDay_AllDayEventWithoutEndLastsOneDay(t *testing.T) {
	got := today(t, ics(vevent("UID:d", "DTSTART;VALUE=DATE:20260916", "SUMMARY:Holiday")))
	if len(got) != 1 {
		t.Fatalf("got %d events, want 1", len(got))
	}
	from, to := wednesday()
	e := got[0]
	if !e.AllDay || !e.Start.Equal(from) || !e.End.Equal(to) {
		t.Errorf("event = %+v, want all-day %v - %v", e, from, to)
	}
}

func TestEventsForDay_MultiDayAllDayEvent(t *testing.T) {
	spanning := today(t, ics(vevent("UID:s", "DTSTART;VALUE=DATE:20260915", "DTEND;VALUE=DATE:20260917", "SUMMARY:Trip")))
	if len(spanning) != 1 {
		t.Errorf("event spanning today: got %d, want 1", len(spanning))
	}
	// DTEND is exclusive: an event ending on the 16th does not touch the 16th.
	ended := today(t, ics(vevent("UID:e", "DTSTART;VALUE=DATE:20260915", "DTEND;VALUE=DATE:20260916", "SUMMARY:Gone")))
	if len(ended) != 0 {
		t.Errorf("event ending at midnight today: got %d, want 0", len(ended))
	}
}

func TestEventsForDay_OvernightEventFromYesterday(t *testing.T) {
	got := today(t, ics(vevent(
		"UID:n",
		"DTSTART;TZID=Europe/Kyiv:20260915T230000",
		"DTEND;TZID=Europe/Kyiv:20260916T010000",
		"SUMMARY:Night shift",
	)))
	if len(got) != 1 {
		t.Fatalf("got %d events, want 1", len(got))
	}
	if !got[0].Start.Equal(at(2026, 9, 15, 23, 0)) {
		t.Errorf("start = %v, want 23:00 on the 15th", got[0].Start)
	}
}

func TestEventsForDay_ZeroDurationEvent(t *testing.T) {
	inside := today(t, ics(vevent("UID:z", "DTSTART;TZID=Europe/Kyiv:20260916T120000", "SUMMARY:Ping")))
	if len(inside) != 1 {
		t.Fatalf("zero-length inside day: got %d, want 1", len(inside))
	}
	if !inside[0].End.Equal(inside[0].Start) {
		t.Errorf("end = %v, want equal to start %v", inside[0].End, inside[0].Start)
	}
	atStart := today(t, ics(vevent("UID:z", "DTSTART;TZID=Europe/Kyiv:20260916T000000", "SUMMARY:Midnight")))
	if len(atStart) != 1 {
		t.Errorf("zero-length at day start: got %d, want 1", len(atStart))
	}
	atEnd := today(t, ics(vevent("UID:z", "DTSTART;TZID=Europe/Kyiv:20260917T000000", "SUMMARY:Next midnight")))
	if len(atEnd) != 0 {
		t.Errorf("zero-length at day end: got %d, want 0", len(atEnd))
	}
}

func TestEventsForDay_TimedEventCoveringWholeDayShowsAsAllDay(t *testing.T) {
	got := today(t, ics(vevent(
		"UID:c",
		"DTSTART;TZID=Europe/Kyiv:20260915T090000",
		"DTEND;TZID=Europe/Kyiv:20260917T170000",
		"SUMMARY:Conference",
	)))
	if len(got) != 1 || !got[0].AllDay {
		t.Fatalf("got %+v, want one all-day event", got)
	}
}

const weeklyMaster = "UID:w1\r\n" +
	"DTSTART;TZID=Europe/Kyiv:20260805T090000\r\n" +
	"DTEND;TZID=Europe/Kyiv:20260805T100000\r\n" +
	"RRULE:FREQ=WEEKLY;BYDAY=WE\r\n" +
	"SUMMARY:Weekly sync"

func TestEventsForDay_WeeklyRecurrenceExpandsToToday(t *testing.T) {
	got := today(t, ics(vevent(weeklyMaster)))
	if len(got) != 1 {
		t.Fatalf("got %d events, want 1: %+v", len(got), got)
	}
	e := got[0]
	if !e.Start.Equal(at(2026, 9, 16, 9, 0)) || !e.End.Equal(at(2026, 9, 16, 10, 0)) {
		t.Errorf("times = %v - %v, want 09:00-10:00 today", e.Start, e.End)
	}
	if e.UID != "w1" || e.Title != "Weekly sync" {
		t.Errorf("event = %+v", e)
	}
}

func TestEventsForDay_RecurrenceNotDueTodayIsExcluded(t *testing.T) {
	got := today(t, ics(vevent(
		"UID:w2",
		"DTSTART;TZID=Europe/Kyiv:20260803T090000",
		"DTEND;TZID=Europe/Kyiv:20260803T100000",
		"RRULE:FREQ=WEEKLY;BYDAY=MO",
		"SUMMARY:Monday only",
	)))
	if len(got) != 0 {
		t.Fatalf("got %d events, want 0", len(got))
	}
}

func TestEventsForDay_ExdateRemovesInstance(t *testing.T) {
	got := today(t, ics(vevent(weeklyMaster, "EXDATE;TZID=Europe/Kyiv:20260916T090000")))
	if len(got) != 0 {
		t.Fatalf("got %d events, want 0: %+v", len(got), got)
	}
}

func TestEventsForDay_ExdateInUTCMatchesLocalInstance(t *testing.T) {
	// 06:00Z is 09:00 in Kyiv: the exclusion must match by instant.
	got := today(t, ics(vevent(weeklyMaster, "EXDATE:20260916T060000Z")))
	if len(got) != 0 {
		t.Fatalf("got %d events, want 0: %+v", len(got), got)
	}
}

func TestEventsForDay_OverrideReplacesGeneratedInstance(t *testing.T) {
	got := today(t, ics(
		vevent(weeklyMaster),
		vevent(
			"UID:w1",
			"RECURRENCE-ID;TZID=Europe/Kyiv:20260916T090000",
			"DTSTART;TZID=Europe/Kyiv:20260916T140000",
			"DTEND;TZID=Europe/Kyiv:20260916T150000",
			"SUMMARY:Weekly sync (moved)",
		),
	))
	if len(got) != 1 {
		t.Fatalf("got %d events, want 1: %+v", len(got), got)
	}
	if got[0].Title != "Weekly sync (moved)" || !got[0].Start.Equal(at(2026, 9, 16, 14, 0)) {
		t.Errorf("event = %+v, want the moved instance at 14:00", got[0])
	}
}

func TestEventsForDay_OverrideMovedToAnotherDayLeavesTodayEmpty(t *testing.T) {
	got := today(t, ics(
		vevent(weeklyMaster),
		vevent(
			"UID:w1",
			"RECURRENCE-ID;TZID=Europe/Kyiv:20260916T090000",
			"DTSTART;TZID=Europe/Kyiv:20260917T090000",
			"DTEND;TZID=Europe/Kyiv:20260917T100000",
			"SUMMARY:Weekly sync",
		),
	))
	if len(got) != 0 {
		t.Fatalf("got %d events, want 0: %+v", len(got), got)
	}
}

func TestEventsForDay_OverrideMovedIntoTodayAppears(t *testing.T) {
	got := today(t, ics(
		vevent(weeklyMaster),
		vevent(
			"UID:w1",
			"RECURRENCE-ID;TZID=Europe/Kyiv:20260923T090000",
			"DTSTART;TZID=Europe/Kyiv:20260916T160000",
			"DTEND;TZID=Europe/Kyiv:20260916T170000",
			"SUMMARY:Weekly sync",
		),
	))
	if len(got) != 2 {
		t.Fatalf("got %d events, want 2 (regular 09:00 and moved 16:00): %+v", len(got), got)
	}
}

func TestEventsForDay_CancelledInstanceIsDropped(t *testing.T) {
	got := today(t, ics(
		vevent(weeklyMaster),
		vevent(
			"UID:w1",
			"RECURRENCE-ID;TZID=Europe/Kyiv:20260916T090000",
			"DTSTART;TZID=Europe/Kyiv:20260916T090000",
			"DTEND;TZID=Europe/Kyiv:20260916T100000",
			"STATUS:CANCELLED",
			"SUMMARY:Weekly sync",
		),
	))
	if len(got) != 0 {
		t.Fatalf("got %d events, want 0: %+v", len(got), got)
	}
}

func TestEventsForDay_CancelledEventIsDropped(t *testing.T) {
	got := today(t, ics(vevent(
		"UID:x",
		"DTSTART;TZID=Europe/Kyiv:20260916T090000",
		"DTEND;TZID=Europe/Kyiv:20260916T100000",
		"STATUS:CANCELLED",
		"SUMMARY:Nope",
	)))
	if len(got) != 0 {
		t.Fatalf("got %d events, want 0", len(got))
	}
}

func TestEventsForDay_ConvertsForeignTZIDToConfiguredZone(t *testing.T) {
	got := today(t, ics(vevent(
		"UID:ny",
		"DTSTART;TZID=America/New_York:20260916T090000",
		"DTEND;TZID=America/New_York:20260916T100000",
		"SUMMARY:NY call",
	)))
	if len(got) != 1 {
		t.Fatalf("got %d events, want 1", len(got))
	}
	if got[0].Start.Location() != kyiv || got[0].Start.Hour() != 16 {
		t.Errorf("start = %v, want 16:00 Kyiv", got[0].Start)
	}
}

func TestEventsForDay_UTCTimesConvertToConfiguredZone(t *testing.T) {
	got := today(t, ics(vevent("UID:u", "DTSTART:20260916T060000Z", "DTEND:20260916T070000Z", "SUMMARY:UTC")))
	if len(got) != 1 || got[0].Start.Hour() != 9 || got[0].Start.Location() != kyiv {
		t.Fatalf("got %+v, want one event at 09:00 Kyiv", got)
	}
}

func TestEventsForDay_FloatingTimeUsesConfiguredZone(t *testing.T) {
	got := today(t, ics(vevent("UID:f", "DTSTART:20260916T090000", "DTEND:20260916T100000", "SUMMARY:Floating")))
	if len(got) != 1 || !got[0].Start.Equal(at(2026, 9, 16, 9, 0)) {
		t.Fatalf("got %+v, want one event at 09:00 Kyiv", got)
	}
}

func TestEventsForDay_UnknownTZIDFallsBackToConfiguredZone(t *testing.T) {
	// Outlook exports Windows zone names that time.LoadLocation cannot resolve.
	got := today(t, ics(vevent(
		"UID:o",
		"DTSTART;TZID=Central European Standard Time:20260916T090000",
		"DTEND;TZID=Central European Standard Time:20260916T100000",
		"SUMMARY:Outlook",
	)))
	if len(got) != 1 || !got[0].Start.Equal(at(2026, 9, 16, 9, 0)) {
		t.Fatalf("got %+v, want one event at 09:00 in the configured zone", got)
	}
}

func TestEventsForDay_UnknownTZIDOnRecurringEventStillExpands(t *testing.T) {
	got := today(t, ics(vevent(
		"UID:or",
		"DTSTART;TZID=Central European Standard Time:20260805T090000",
		"DTEND;TZID=Central European Standard Time:20260805T100000",
		"RRULE:FREQ=WEEKLY;BYDAY=WE",
		"EXDATE;TZID=Central European Standard Time:20260909T090000",
		"SUMMARY:Outlook weekly",
	)))
	if len(got) != 1 || !got[0].Start.Equal(at(2026, 9, 16, 9, 0)) {
		t.Fatalf("got %+v, want one event at 09:00", got)
	}
}

func TestEventsForDay_RecurringKeepsWallClockAcrossDST(t *testing.T) {
	warsaw := mustZone("Europe/Warsaw")
	// Weekly at 09:00 since July (CEST); 4 November is after the switch to CET.
	from := time.Date(2026, 11, 4, 0, 0, 0, 0, warsaw)
	cal := parseICS(t, ics(vevent(
		"UID:dst",
		"DTSTART;TZID=Europe/Warsaw:20260701T090000",
		"DTEND;TZID=Europe/Warsaw:20260701T100000",
		"RRULE:FREQ=WEEKLY;BYDAY=WE",
		"SUMMARY:Weekly",
	)))
	got := eventsForDay(cal, "Work", from, from.AddDate(0, 0, 1), warsaw)
	if len(got) != 1 || got[0].Start.Hour() != 9 {
		t.Fatalf("got %+v, want one event at 09:00 local", got)
	}
}

func TestEventsForDay_RecurringAllDayEndsAtNextMidnightAcrossDST(t *testing.T) {
	warsaw := mustZone("Europe/Warsaw")
	// 25 October 2026 is the 25-hour day when CEST ends.
	from := time.Date(2026, 10, 25, 0, 0, 0, 0, warsaw)
	to := from.AddDate(0, 0, 1)
	cal := parseICS(t, ics(vevent(
		"UID:sun",
		"DTSTART;VALUE=DATE:20261004",
		"RRULE:FREQ=WEEKLY;BYDAY=SU",
		"SUMMARY:Sunday",
	)))
	got := eventsForDay(cal, "Home", from, to, warsaw)
	if len(got) != 1 {
		t.Fatalf("got %d events, want 1", len(got))
	}
	if !got[0].AllDay || !got[0].Start.Equal(from) || !got[0].End.Equal(to) {
		t.Errorf("event = %+v, want all-day %v - %v", got[0], from, to)
	}
}

func TestEventsForDay_RecurringInstanceStartingYesterdayOverlapsToday(t *testing.T) {
	got := today(t, ics(vevent(
		"UID:late",
		"DTSTART;TZID=Europe/Kyiv:20260818T230000",
		"DTEND;TZID=Europe/Kyiv:20260819T010000",
		"RRULE:FREQ=WEEKLY;BYDAY=TU",
		"SUMMARY:Late show",
	)))
	if len(got) != 1 || !got[0].Start.Equal(at(2026, 9, 15, 23, 0)) {
		t.Fatalf("got %+v, want the Tuesday 23:00 instance", got)
	}
}

func TestEventsForDay_SummaryIsUnescapedAndDefaulted(t *testing.T) {
	got := today(t, ics(
		vevent("UID:e1", "DTSTART;TZID=Europe/Kyiv:20260916T090000", `SUMMARY:Lunch\, then coffee`),
		vevent("UID:e2", "DTSTART;TZID=Europe/Kyiv:20260916T100000"),
	))
	if len(got) != 2 {
		t.Fatalf("got %d events, want 2", len(got))
	}
	if got[0].Title != "Lunch, then coffee" {
		t.Errorf("title = %q, want unescaped comma", got[0].Title)
	}
	if got[1].Title != "(No title)" {
		t.Errorf("title = %q, want placeholder", got[1].Title)
	}
}

func TestEventsForDay_SkipsEventWithoutStart(t *testing.T) {
	got := today(t, ics(
		vevent("UID:bad", "SUMMARY:No start"),
		vevent("UID:ok", "DTSTART;TZID=Europe/Kyiv:20260916T090000", "SUMMARY:Fine"),
	))
	if len(got) != 1 || got[0].UID != "ok" {
		t.Fatalf("got %+v, want only the valid event", got)
	}
}

func TestMerge_AllDayFirstThenByStartThenTitle(t *testing.T) {
	in := []Event{
		{UID: "1", Title: "B", Start: at(2026, 9, 16, 10, 0), End: at(2026, 9, 16, 11, 0)},
		{UID: "2", Title: "Z", AllDay: true, Start: at(2026, 9, 16, 0, 0), End: at(2026, 9, 17, 0, 0)},
		{UID: "3", Title: "A0", Start: at(2026, 9, 16, 9, 0), End: at(2026, 9, 16, 10, 0)},
		{UID: "4", Title: "A", Start: at(2026, 9, 16, 9, 0), End: at(2026, 9, 16, 9, 30)},
	}
	got := merge(in)
	want := []string{"2", "4", "3", "1"}
	if len(got) != len(want) {
		t.Fatalf("got %d events, want %d", len(got), len(want))
	}
	for i, uid := range want {
		if got[i].UID != uid {
			t.Errorf("merge[%d].UID = %q, want %q", i, got[i].UID, uid)
		}
	}
}

func TestMerge_DedupesSameUIDAndStartAcrossCalendars(t *testing.T) {
	in := []Event{
		{UID: "same", Calendar: "Work", Title: "Dentist", Start: at(2026, 9, 16, 9, 0), End: at(2026, 9, 16, 10, 0)},
		{UID: "same", Calendar: "Home", Title: "Dentist", Start: at(2026, 9, 16, 9, 0), End: at(2026, 9, 16, 10, 0)},
	}
	got := merge(in)
	if len(got) != 1 || got[0].Calendar != "Work" {
		t.Fatalf("got %+v, want one event from the first calendar", got)
	}
}

func TestMerge_KeepsRecurringInstancesDistinct(t *testing.T) {
	in := []Event{
		{UID: "r", Title: "Daily", Start: at(2026, 9, 16, 9, 0), End: at(2026, 9, 16, 9, 30)},
		{UID: "r", Title: "Daily", Start: at(2026, 9, 16, 18, 0), End: at(2026, 9, 16, 18, 30)},
	}
	if got := merge(in); len(got) != 2 {
		t.Fatalf("got %d events, want 2", len(got))
	}
}

func TestMerge_DedupesByTitleAndTimesWithoutUID(t *testing.T) {
	in := []Event{
		{Title: "Gym", Start: at(2026, 9, 16, 7, 0), End: at(2026, 9, 16, 8, 0)},
		{Title: "gym", Start: at(2026, 9, 16, 7, 0), End: at(2026, 9, 16, 8, 0)},
		{Title: "Gym", Start: at(2026, 9, 16, 19, 0), End: at(2026, 9, 16, 20, 0)},
	}
	if got := merge(in); len(got) != 2 {
		t.Fatalf("got %d events, want 2", len(got))
	}
}
