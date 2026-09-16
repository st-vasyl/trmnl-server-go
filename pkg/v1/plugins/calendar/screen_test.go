package calendar

import (
	"strings"
	"testing"
	"time"
	"trmnl-server-go/pkg/v1/render"
)

func TestTimeLabel(t *testing.T) {
	from, to := wednesday()
	cases := []struct {
		name string
		e    Event
		want string
	}{
		{"all day", Event{AllDay: true, Start: from, End: to}, "All day"},
		{"inside day", Event{Start: at(2026, 9, 16, 9, 0), End: at(2026, 9, 16, 10, 30)}, "09:00-10:30"},
		{"started yesterday", Event{Start: at(2026, 9, 15, 23, 0), End: at(2026, 9, 16, 1, 0)}, "00:00-01:00"},
		{"ends tomorrow", Event{Start: at(2026, 9, 16, 22, 0), End: at(2026, 9, 17, 2, 0)}, "22:00-24:00"},
		{"ends at midnight", Event{Start: at(2026, 9, 16, 22, 0), End: to}, "22:00-24:00"},
		{"zero length", Event{Start: at(2026, 9, 16, 12, 0), End: at(2026, 9, 16, 12, 0)}, "12:00"},
	}
	for _, c := range cases {
		if got := timeLabel(c.e, from, to); got != c.want {
			t.Errorf("%s: timeLabel = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestHeaderDate(t *testing.T) {
	day := time.Date(2026, 9, 16, 0, 0, 0, 0, kyiv)
	if got, want := headerDate(day), "Wednesday, 16 September"; got != want {
		t.Errorf("headerDate = %q, want %q", got, want)
	}
}

func TestTruncate(t *testing.T) {
	loadTestFont(t)

	short, err := truncate("Standup", rowSize, 400)
	if err != nil {
		t.Fatalf("truncate: %v", err)
	}
	if short != "Standup" {
		t.Errorf("short title changed to %q", short)
	}

	long := "A very long meeting title that will not fit on one row of the screen"
	got, err := truncate(long, rowSize, 300)
	if err != nil {
		t.Fatalf("truncate: %v", err)
	}
	if !strings.HasSuffix(got, "...") || len(got) >= len(long) {
		t.Errorf("long title = %q, want a shortened string ending in ...", got)
	}
	if w, _ := render.TextWidth(got, rowSize); w > 300 {
		t.Errorf("truncated width = %d, want <= 300", w)
	}

	none, err := truncate(long, rowSize, 1)
	if err != nil {
		t.Fatalf("truncate: %v", err)
	}
	if none != "" {
		t.Errorf("nothing fits: got %q, want empty", none)
	}
}
