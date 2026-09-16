package calendar

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func timed(uid string, sh, sm, eh, em int) Event {
	return Event{UID: uid, Title: uid, Start: at(2026, 9, 16, sh, sm), End: at(2026, 9, 16, eh, em)}
}

func TestTimelineWindow_DefaultsToSevenToNineteen(t *testing.T) {
	day, _ := wednesday()
	start, end := timelineWindow(nil, day)
	if !start.Equal(at(2026, 9, 16, 7, 0)) || !end.Equal(at(2026, 9, 16, 19, 0)) {
		t.Errorf("window = %v - %v, want 07:00-19:00", start, end)
	}
}

func TestTimelineWindow_StretchesToWholeHoursAroundEvents(t *testing.T) {
	day, _ := wednesday()
	events := []Event{timed("early", 6, 15, 6, 45), timed("late", 18, 30, 20, 10)}
	start, end := timelineWindow(events, day)
	if !start.Equal(at(2026, 9, 16, 6, 0)) {
		t.Errorf("start = %v, want 06:00", start)
	}
	if !end.Equal(at(2026, 9, 16, 21, 0)) {
		t.Errorf("end = %v, want 21:00", end)
	}
}

func TestTimelineWindow_ClampsToTheDayAndIgnoresAllDay(t *testing.T) {
	day, next := wednesday()
	events := []Event{
		{UID: "night", Start: at(2026, 9, 15, 23, 0), End: at(2026, 9, 17, 1, 0)},
		{UID: "allday", AllDay: true, Start: day, End: next},
	}
	start, end := timelineWindow(events, day)
	if !start.Equal(day) || !end.Equal(next) {
		t.Errorf("window = %v - %v, want the whole day", start, end)
	}
	start, end = timelineWindow(events[1:], day)
	if !start.Equal(at(2026, 9, 16, 7, 0)) || !end.Equal(at(2026, 9, 16, 19, 0)) {
		t.Errorf("all-day only: window = %v - %v, want the default", start, end)
	}
}

func cols(t *testing.T, blocks []block, uid string) (col, n int) {
	t.Helper()
	for _, b := range blocks {
		if b.UID == uid {
			return b.col, b.cols
		}
	}
	t.Fatalf("block %q missing from %+v", uid, blocks)
	return 0, 0
}

func TestLayoutColumns_TouchingEventsShareNoColumns(t *testing.T) {
	blocks := layoutColumns([]Event{timed("a", 9, 0, 10, 0), timed("b", 10, 0, 11, 0)}, 45*time.Minute)
	for _, uid := range []string{"a", "b"} {
		if col, n := cols(t, blocks, uid); col != 0 || n != 1 {
			t.Errorf("%s: col %d of %d, want 0 of 1", uid, col, n)
		}
	}
}

func TestLayoutColumns_OverlappingEventsSplitTheWidth(t *testing.T) {
	blocks := layoutColumns([]Event{timed("b", 9, 30, 10, 30), timed("a", 9, 0, 10, 0)}, time.Minute)
	if col, n := cols(t, blocks, "a"); col != 0 || n != 2 {
		t.Errorf("a: col %d of %d, want 0 of 2", col, n)
	}
	if col, n := cols(t, blocks, "b"); col != 1 || n != 2 {
		t.Errorf("b: col %d of %d, want 1 of 2", col, n)
	}
}

func TestLayoutColumns_ReusesFreedColumnWithinCluster(t *testing.T) {
	blocks := layoutColumns([]Event{timed("a", 9, 0, 10, 0), timed("b", 9, 30, 11, 0), timed("c", 10, 0, 10, 30)}, time.Minute)
	if col, n := cols(t, blocks, "c"); col != 0 || n != 2 {
		t.Errorf("c: col %d of %d, want 0 of 2 (a's column is free)", col, n)
	}
}

func TestLayoutColumns_SeparateClustersAreIndependent(t *testing.T) {
	blocks := layoutColumns([]Event{timed("a", 9, 0, 10, 0), timed("b", 9, 0, 10, 0), timed("c", 14, 0, 15, 0)}, time.Minute)
	if _, n := cols(t, blocks, "a"); n != 2 {
		t.Errorf("a: %d columns, want 2", n)
	}
	if col, n := cols(t, blocks, "c"); col != 0 || n != 1 {
		t.Errorf("c: col %d of %d, want 0 of 1", col, n)
	}
}

func TestLayoutColumns_ZeroLengthEventStillOccupiesAColumn(t *testing.T) {
	blocks := layoutColumns([]Event{timed("a", 9, 0, 10, 0), timed("ping", 9, 30, 9, 30)}, 0)
	if _, n := cols(t, blocks, "ping"); n != 2 {
		t.Errorf("ping: %d columns, want 2", n)
	}
}

func TestLayoutColumns_ShortEventKeepsItsMinimumHeightClear(t *testing.T) {
	// A 15-minute standup drawn at its minimum block height would run into a
	// meeting starting 15 minutes later, so the two must share the width.
	blocks := layoutColumns([]Event{timed("standup", 9, 30, 9, 45), timed("meeting", 10, 0, 11, 0)}, 45*time.Minute)
	if col, n := cols(t, blocks, "standup"); col != 0 || n != 2 {
		t.Errorf("standup: col %d of %d, want 0 of 2", col, n)
	}
	if col, n := cols(t, blocks, "meeting"); col != 1 || n != 2 {
		t.Errorf("meeting: col %d of %d, want 1 of 2", col, n)
	}
	// With room to spare they stack as usual.
	blocks = layoutColumns([]Event{timed("standup", 9, 30, 9, 45), timed("meeting", 10, 0, 11, 0)}, 10*time.Minute)
	if _, n := cols(t, blocks, "meeting"); n != 1 {
		t.Errorf("meeting: %d columns, want 1", n)
	}
}

func TestBlockMetrics_ScaleWithTheHourGrid(t *testing.T) {
	// Twelve visible hours: a 30-minute slot is about 14px, so single-line
	// text shrinks to 11px and the minimum block stays inside the slot.
	size, minH := blockMetrics(float64(tlGridBottom-tlGridTop) / 12)
	if size != 11 || minH != 14 {
		t.Errorf("12h grid: size %v, minH %d, want 11 and 14", size, minH)
	}
	// A short window leaves room for the full-size text.
	size, minH = blockMetrics(float64(tlGridBottom-tlGridTop) / 6)
	if size != 16 || minH != 19 {
		t.Errorf("6h grid: size %v, minH %d, want 16 and 19", size, minH)
	}
}

func TestLineSizeFor_GrowsWithBlockHeight(t *testing.T) {
	cases := map[int]float64{14: 11, 10: 11, 17: 14, 28: 16, 60: 16}
	for height, want := range cases {
		if got := lineSizeFor(height); got != want {
			t.Errorf("lineSizeFor(%d) = %v, want %v", height, got, want)
		}
	}
}

func TestMinBlockDuration_LetsShortConsecutiveMeetingsStack(t *testing.T) {
	pxPerHour := float64(tlGridBottom-tlGridTop) / 12
	if d := minBlockDuration(pxPerHour); d >= 30*time.Minute {
		t.Errorf("minimum block spans %v on a 12h grid, want under 30m so half-hour slots stack", d)
	}
	// A 20-minute standup followed ten minutes later by the next meeting
	// must stack, not sit side by side.
	blocks := layoutColumns([]Event{timed("standup", 17, 30, 17, 50), timed("leads", 18, 0, 18, 20)}, minBlockDuration(pxPerHour))
	if _, n := cols(t, blocks, "leads"); n != 1 {
		t.Errorf("leads: %d columns, want 1", n)
	}
}

func TestOneLineText_PrefersTitleWhenSpanDoesNotFit(t *testing.T) {
	loadTestFont(t)
	wide, err := oneLineText("09:30-10:00", "Standup", tlBlockLineSize, 600)
	if err != nil {
		t.Fatalf("oneLineText: %v", err)
	}
	if wide != "09:30-10:00 · Standup" {
		t.Errorf("wide block = %q, want span and title", wide)
	}
	narrow, err := oneLineText("09:30-10:00", "Standup", tlBlockLineSize, 120)
	if err != nil {
		t.Fatalf("oneLineText: %v", err)
	}
	if narrow != "Standup" {
		t.Errorf("narrow block = %q, want the title alone", narrow)
	}
	tiny, err := oneLineText("09:30-10:00", "Standup", tlBlockLineSize, 50)
	if err != nil {
		t.Fatalf("oneLineText: %v", err)
	}
	if !strings.HasSuffix(tiny, "...") || strings.Contains(tiny, "09:30") {
		t.Errorf("tiny block = %q, want a shortened title", tiny)
	}
}

func TestNew_Layout(t *testing.T) {
	if _, err := New("", "grid", []Source{okSource("a")}); err == nil {
		t.Error("expected error for an unknown layout")
	}
	for _, name := range []string{"", "timeline", "list", " List "} {
		if _, err := New("", name, []Source{okSource("a")}); err != nil {
			t.Errorf("layout %q: %v", name, err)
		}
	}
	p, _ := New("", "", []Source{okSource("a")})
	if p.layout != layoutTimeline {
		t.Errorf("default layout = %q, want timeline", p.layout)
	}
}

func TestRender_TimelineWritesFullScreenPNG(t *testing.T) {
	loadTestFont(t)
	work := feedServer(t, http.StatusOK, ics(
		vevent("UID:1", "DTSTART;TZID=Europe/Kyiv:20260916T063000", "DTEND;TZID=Europe/Kyiv:20260916T071500", "SUMMARY:Morning run"),
		vevent("UID:2", "DTSTART;TZID=Europe/Kyiv:20260916T093000", "DTEND;TZID=Europe/Kyiv:20260916T094500", "SUMMARY:Team standup"),
		vevent("UID:3", "DTSTART;TZID=Europe/Kyiv:20260916T100000", "DTEND;TZID=Europe/Kyiv:20260916T110000", "SUMMARY:Monthly catchup with the dev team"),
		vevent("UID:4", "DTSTART;TZID=Europe/Kyiv:20260916T103000", "DTEND;TZID=Europe/Kyiv:20260916T120000", "SUMMARY:Overlapping review"),
		vevent("UID:5", "DTSTART;VALUE=DATE:20260916", "SUMMARY:Security audit"),
		vevent("UID:6", "DTSTART;TZID=Europe/Kyiv:20260916T200000", "DTEND;TZID=Europe/Kyiv:20260916T213000", "SUMMARY:Late dinner"),
	), nil)
	p, err := New("Europe/Kyiv", "timeline", []Source{{Name: "Work", URL: work.URL}})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	fixedNow(p, time.Date(2026, 9, 16, 8, 15, 0, 0, kyiv))

	out := filepath.Join(t.TempDir(), "calendar.png")
	if err := p.Render("calendar", out, 4.0); err != nil {
		t.Fatalf("Render: %v", err)
	}
	w, h, dark := decodePNG(t, out)
	if w != 800 || h != 480 {
		t.Errorf("size = %dx%d, want 800x480", w, h)
	}
	if dark < 3000 {
		t.Errorf("dark pixels = %d, want grid lines, labels and event blocks", dark)
	}
}

func TestRender_TimelineEmptyDayStillWritesScreen(t *testing.T) {
	loadTestFont(t)
	empty := feedServer(t, http.StatusOK, ics(), nil)
	p, err := New("Europe/Kyiv", "", []Source{{Name: "Work", URL: empty.URL}})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	fixedNow(p, time.Date(2026, 9, 16, 8, 15, 0, 0, kyiv))

	out := filepath.Join(t.TempDir(), "calendar.png")
	if err := p.Render("calendar", out, 4.0); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if w, h, dark := decodePNG(t, out); w != 800 || h != 480 || dark < 500 {
		t.Errorf("size = %dx%d dark = %d, want a full screen with the hour grid", w, h, dark)
	}
}
