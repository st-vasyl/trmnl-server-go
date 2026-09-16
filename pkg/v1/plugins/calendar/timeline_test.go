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

func assertWindow(t *testing.T, events []Event, sh, eh int) {
	t.Helper()
	day, _ := wednesday()
	start, end := timelineWindow(events, day)
	if !start.Equal(at(2026, 9, 16, sh, 0)) || !end.Equal(at(2026, 9, 16, eh, 0)) {
		t.Errorf("window = %s - %s, want %02d:00-%02d:00", start.Format("15:04"), end.Format("15:04"), sh, eh)
	}
}

func TestTimelineWindow_DefaultsToSevenToNineteen(t *testing.T) {
	assertWindow(t, nil, 7, 19)
}

func TestTimelineWindow_CoversAllEventsOnWholeHours(t *testing.T) {
	assertWindow(t, []Event{timed("early", 6, 15, 6, 45), timed("late", 18, 30, 20, 10)}, 6, 21)
}

func TestTimelineWindow_FitsTheDayWithAnEightHourMinimum(t *testing.T) {
	// An afternoon of meetings: the window grows backwards from the evening edge.
	assertWindow(t, []Event{timed("a", 15, 30, 16, 30), timed("b", 17, 30, 18, 20)}, 11, 19)
	// A morning: it grows towards the evening first.
	assertWindow(t, []Event{timed("a", 7, 0, 9, 0)}, 7, 15)
	// Late events: nothing to grow after them, so it grows backwards.
	assertWindow(t, []Event{timed("a", 20, 0, 21, 0)}, 13, 21)
	// One midday event: evening first, then the rest backwards.
	assertWindow(t, []Event{timed("a", 12, 0, 12, 30)}, 11, 19)
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
	assertWindow(t, events[1:], 7, 19)
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
	blocks := layoutColumns([]Event{timed("a", 9, 0, 10, 0), timed("b", 10, 0, 11, 0)})
	for _, uid := range []string{"a", "b"} {
		if col, n := cols(t, blocks, uid); col != 0 || n != 1 {
			t.Errorf("%s: col %d of %d, want 0 of 1", uid, col, n)
		}
	}
}

func TestLayoutColumns_OverlappingEventsSplitTheWidth(t *testing.T) {
	blocks := layoutColumns([]Event{timed("b", 9, 30, 10, 30), timed("a", 9, 0, 10, 0)})
	if col, n := cols(t, blocks, "a"); col != 0 || n != 2 {
		t.Errorf("a: col %d of %d, want 0 of 2", col, n)
	}
	if col, n := cols(t, blocks, "b"); col != 1 || n != 2 {
		t.Errorf("b: col %d of %d, want 1 of 2", col, n)
	}
}

func TestLayoutColumns_ReusesFreedColumnWithinCluster(t *testing.T) {
	blocks := layoutColumns([]Event{timed("a", 9, 0, 10, 0), timed("b", 9, 30, 11, 0), timed("c", 10, 0, 10, 30)})
	if col, n := cols(t, blocks, "c"); col != 0 || n != 2 {
		t.Errorf("c: col %d of %d, want 0 of 2 (a's column is free)", col, n)
	}
}

func TestLayoutColumns_SeparateClustersAreIndependent(t *testing.T) {
	blocks := layoutColumns([]Event{timed("a", 9, 0, 10, 0), timed("b", 9, 0, 10, 0), timed("c", 14, 0, 15, 0)})
	if _, n := cols(t, blocks, "a"); n != 2 {
		t.Errorf("a: %d columns, want 2", n)
	}
	if col, n := cols(t, blocks, "c"); col != 0 || n != 1 {
		t.Errorf("c: col %d of %d, want 0 of 1", col, n)
	}
}

func TestLayoutColumns_ZeroLengthEventStillOccupiesAColumn(t *testing.T) {
	blocks := layoutColumns([]Event{timed("a", 9, 0, 10, 0), timed("ping", 9, 30, 9, 30)})
	if _, n := cols(t, blocks, "ping"); n != 2 {
		t.Errorf("ping: %d columns, want 2", n)
	}
}

// twelveHourGrid is the scale of a 07:00-19:00 window on the real grid.
func twelveHourGrid() gridScale {
	return gridScale{winStart: at(2026, 9, 16, 7, 0), pxPerHour: float64(tlGridBottom-tlGridTop) / 12, top: tlGridTop, bottom: tlGridBottom}
}

func TestPlaceBlocks_UsesNaturalPositionsWhenThereIsRoom(t *testing.T) {
	g := gridScale{winStart: at(2026, 9, 16, 7, 0), pxPerHour: 40, top: 100, bottom: 500}
	got := placeBlocks(layoutColumns([]Event{timed("a", 9, 0, 10, 0), timed("b", 10, 0, 11, 0)}), g, 19, 80, 700)
	if len(got) != 2 {
		t.Fatalf("placed %d blocks, want 2", len(got))
	}
	a, b := got[0].rect, got[1].rect
	if a.Min.Y != 180 || a.Max.Y != 220 || b.Min.Y != 220 || b.Max.Y != 260 {
		t.Errorf("rects a=%v b=%v, want 180-220 and 220-260", a, b)
	}
	if a.Min.X != 80+tlBlockGap || a.Max.X != 780-tlBlockGap {
		t.Errorf("a spans x %d-%d, want the full width minus the gap", a.Min.X, a.Max.X)
	}
}

func TestPlaceBlocks_PushesTheNextBlockBelowAShortOne(t *testing.T) {
	// On a 12-hour grid a 20-minute standup is 10px tall; drawn at the 19px
	// minimum it runs into the meeting ten minutes later, which moves down.
	g := twelveHourGrid()
	got := placeBlocks(layoutColumns([]Event{timed("standup", 17, 30, 17, 50), timed("leads", 18, 0, 18, 20)}), g, 19, 82, 698)
	standup, leads := got[0].rect, got[1].rect
	if standup.Min.Y != g.y(at(2026, 9, 16, 17, 30)) || standup.Dy() != 19 {
		t.Errorf("standup rect = %v, want 19px tall at its natural top", standup)
	}
	if leads.Min.Y != standup.Max.Y || leads.Dy() != 19 {
		t.Errorf("leads rect = %v, want 19px tall starting at the standup's bottom %d", leads, standup.Max.Y)
	}
}

func TestPlaceBlocks_ChainOfShortMeetingsStacksInOrder(t *testing.T) {
	g := twelveHourGrid()
	events := []Event{timed("a", 9, 0, 9, 15), timed("b", 9, 15, 9, 30), timed("c", 9, 30, 9, 45)}
	got := placeBlocks(layoutColumns(events), g, 19, 82, 698)
	for i := 1; i < len(got); i++ {
		if got[i].rect.Min.Y != got[i-1].rect.Max.Y {
			t.Errorf("block %s starts at %d, want %d (bottom of the previous one)", got[i].UID, got[i].rect.Min.Y, got[i-1].rect.Max.Y)
		}
	}
}

func TestPlaceBlocks_SideBySideBlocksDoNotPushEachOther(t *testing.T) {
	g := twelveHourGrid()
	got := placeBlocks(layoutColumns([]Event{timed("a", 9, 0, 9, 10), timed("b", 9, 5, 10, 0)}), g, 19, 82, 698)
	a, b := got[0].rect, got[1].rect
	if a.Max.X > b.Min.X {
		t.Errorf("a %v and b %v should sit side by side", a, b)
	}
	if b.Min.Y != g.y(at(2026, 9, 16, 9, 5)) {
		t.Errorf("b top = %d, want its natural position %d", b.Min.Y, g.y(at(2026, 9, 16, 9, 5)))
	}
}

func TestPlaceBlocks_ClampsToTheGridBottom(t *testing.T) {
	g := twelveHourGrid() // bottom row is 19:00
	got := placeBlocks(layoutColumns([]Event{timed("late", 18, 50, 19, 10)}), g, 19, 82, 698)
	r := got[0].rect
	if r.Max.Y != g.bottom || r.Dy() != 19 {
		t.Errorf("rect = %v, want 19px ending at the grid bottom %d", r, g.bottom)
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
