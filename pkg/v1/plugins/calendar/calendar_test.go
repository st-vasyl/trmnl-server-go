package calendar

import (
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"trmnl-server-go/pkg/v1/render"
)

func okSource(name string) Source {
	return Source{Name: name, URL: "https://example.com/" + name + ".ics"}
}

func TestNew_RejectsEmptyConfig(t *testing.T) {
	if _, err := New("Europe/Kyiv", "list", nil); err == nil {
		t.Fatal("expected error for no calendars")
	}
}

func TestNew_RejectsInvalidSources(t *testing.T) {
	cases := []struct {
		name string
		src  Source
	}{
		{"missing name", Source{Name: "  ", URL: "https://example.com/a.ics"}},
		{"missing url", Source{Name: "Work", URL: ""}},
		{"unsupported scheme", Source{Name: "Work", URL: "ftp://example.com/a.ics"}},
		{"no host", Source{Name: "Work", URL: "https:///a.ics"}},
		{"garbage", Source{Name: "Work", URL: "not a url"}},
	}
	for _, c := range cases {
		if _, err := New("", "list", []Source{c.src}); err == nil {
			t.Errorf("%s: expected error for %+v", c.name, c.src)
		}
	}
}

func TestNew_RewritesWebcalToHTTPS(t *testing.T) {
	p, err := New("", "list", []Source{{Name: "Home", URL: " webcal://p44-caldav.icloud.com/published/2/abc "}})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got, want := p.sources[0].URL, "https://p44-caldav.icloud.com/published/2/abc"; got != want {
		t.Errorf("URL = %q, want %q", got, want)
	}
}

func TestNew_Timezone(t *testing.T) {
	if _, err := New("Mars/Olympus", "list", []Source{okSource("a")}); err == nil {
		t.Error("expected error for unknown timezone")
	}
	p, err := New("", "list", []Source{okSource("a")})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if p.loc != time.Local {
		t.Errorf("empty timezone: loc = %v, want time.Local", p.loc)
	}
	p, err = New("Europe/Kyiv", "list", []Source{okSource("a")})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if p.loc.String() != "Europe/Kyiv" {
		t.Errorf("loc = %v, want Europe/Kyiv", p.loc)
	}
}

func TestNameAndScreens(t *testing.T) {
	p, err := New("", "list", []Source{okSource("a"), okSource("b")})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if p.Name() != "calendar" {
		t.Errorf("Name = %q, want calendar", p.Name())
	}
	if got := p.Screens(); len(got) != 1 || got[0] != "calendar" {
		t.Errorf("Screens = %v, want [calendar]", got)
	}
}

// feedServer serves body for any path, or the given status with no body.
func feedServer(t *testing.T, status int, body string, gotHeaders *http.Header) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if gotHeaders != nil {
			*gotHeaders = r.Header.Clone()
		}
		w.WriteHeader(status)
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestFetchFeed_ParsesCalendarAndSendsAccept(t *testing.T) {
	var headers http.Header
	srv := feedServer(t, http.StatusOK, ics(vevent("UID:1", "DTSTART;VALUE=DATE:20260916", "SUMMARY:X")), &headers)

	cal, err := fetchFeed(srv.URL)
	if err != nil {
		t.Fatalf("fetchFeed: %v", err)
	}
	if n := len(cal.Events()); n != 1 {
		t.Errorf("events = %d, want 1", n)
	}
	if got := headers.Get("Accept"); !strings.Contains(got, "text/calendar") {
		t.Errorf("Accept = %q, want text/calendar", got)
	}
}

func TestFetchFeed_Errors(t *testing.T) {
	notFound := feedServer(t, http.StatusNotFound, "", nil)
	if _, err := fetchFeed(notFound.URL); err == nil {
		t.Error("expected error for 404")
	}
	garbage := feedServer(t, http.StatusOK, "<html>not a calendar</html>", nil)
	if _, err := fetchFeed(garbage.URL); err == nil {
		t.Error("expected error for a non-ICS body")
	}
	closed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := closed.URL
	closed.Close()
	if _, err := fetchFeed(url); err == nil {
		t.Error("expected network error")
	}
}

func fixedNow(p *Plugin, t time.Time) { p.now = func() time.Time { return t } }

func TestCollect_MergesFeedsAndReportsFailures(t *testing.T) {
	work := feedServer(t, http.StatusOK, ics(
		vevent("UID:w", "DTSTART;TZID=Europe/Kyiv:20260916T100000", "DTEND;TZID=Europe/Kyiv:20260916T110000", "SUMMARY:Review"),
		vevent("UID:old", "DTSTART;TZID=Europe/Kyiv:20260901T100000", "SUMMARY:Past"),
	), nil)
	home := feedServer(t, http.StatusOK, ics(
		vevent("UID:h", "DTSTART;VALUE=DATE:20260916", "SUMMARY:Birthday"),
	), nil)
	broken := feedServer(t, http.StatusInternalServerError, "", nil)

	p, err := New("Europe/Kyiv", "list", []Source{
		{Name: "Work", URL: work.URL}, {Name: "Home", URL: home.URL}, {Name: "Broken", URL: broken.URL},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	from, to := wednesday()
	events, failed, err := p.collect(from, to)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	if len(failed) != 1 || failed[0] != "Broken" {
		t.Errorf("failed = %v, want [Broken]", failed)
	}
	if len(events) != 2 {
		t.Fatalf("events = %+v, want 2", events)
	}
	if events[0].Title != "Birthday" || events[0].Calendar != "Home" || !events[0].AllDay {
		t.Errorf("events[0] = %+v, want the all-day Home event first", events[0])
	}
	if events[1].Title != "Review" || events[1].Calendar != "Work" {
		t.Errorf("events[1] = %+v, want the Work event", events[1])
	}
}

func TestCollect_AllFeedsFailingIsAnError(t *testing.T) {
	broken := feedServer(t, http.StatusBadGateway, "", nil)
	p, err := New("Europe/Kyiv", "list", []Source{{Name: "A", URL: broken.URL}, {Name: "B", URL: broken.URL}})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	from, to := wednesday()
	if _, _, err := p.collect(from, to); err == nil {
		t.Fatal("expected error when every feed fails")
	}
}

func TestRender_UnknownScreenDoesNotHitNetwork(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request %s", r.URL)
	}))
	defer srv.Close()
	p, err := New("", "list", []Source{{Name: "A", URL: srv.URL}})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := p.Render("calendar_2", filepath.Join(t.TempDir(), "out.png"), 4.0); err == nil {
		t.Fatal("expected error for an unknown screen")
	}
}

func TestRender_AllFeedsFailingWritesNothing(t *testing.T) {
	broken := feedServer(t, http.StatusInternalServerError, "", nil)
	p, err := New("", "list", []Source{{Name: "A", URL: broken.URL}})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	out := filepath.Join(t.TempDir(), "out.png")
	if err := p.Render("calendar", out, 4.0); err == nil {
		t.Fatal("expected error from Render")
	}
	if _, statErr := os.Stat(out); statErr == nil {
		t.Error("no PNG should be written when every feed fails")
	}
}

// loadTestFont installs the repo-root font.ttf as both the text font and the
// icon font so rendering tests hit no network. Skips when it is missing.
func loadTestFont(t *testing.T) {
	t.Helper()
	ttf, err := os.ReadFile("../../../../font.ttf")
	if err != nil {
		t.Skipf("font.ttf not available: %v", err)
	}
	if err := render.SetFont(ttf); err != nil {
		t.Fatalf("SetFont: %v", err)
	}
	if err := os.MkdirAll("icons", 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join("icons", "MaterialSymbols.ttf"), ttf, 0644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll("icons") })
}

// decodePNG opens the rendered file and returns it with a count of dark pixels.
func decodePNG(t *testing.T, path string) (w, h, dark int) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open output: %v", err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatalf("decode output: %v", err)
	}
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, _ := img.At(x, y).RGBA()
			if r < 0x8000 && g < 0x8000 && bl < 0x8000 {
				dark++
			}
		}
	}
	return b.Dx(), b.Dy(), dark
}

func TestRender_WritesFullScreenPNG(t *testing.T) {
	loadTestFont(t)
	work := feedServer(t, http.StatusOK, ics(
		vevent("UID:1", "DTSTART;TZID=Europe/Kyiv:20260916T090000", "DTEND;TZID=Europe/Kyiv:20260916T093000", "SUMMARY:Standup"),
		vevent("UID:2", "DTSTART;TZID=Europe/Kyiv:20260916T140000", "DTEND;TZID=Europe/Kyiv:20260916T153000", "SUMMARY:A very long meeting title that will not fit on one row of the screen"),
		vevent("UID:3", "DTSTART;VALUE=DATE:20260916", "SUMMARY:Company holiday"),
		vevent("UID:4", "DTSTART;TZID=Europe/Kyiv:20260915T230000", "DTEND;TZID=Europe/Kyiv:20260916T010000", "SUMMARY:Overnight"),
	), nil)
	broken := feedServer(t, http.StatusInternalServerError, "", nil)
	p, err := New("Europe/Kyiv", "list", []Source{{Name: "Work", URL: work.URL}, {Name: "Home", URL: broken.URL}})
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
	if dark < 2000 {
		t.Errorf("dark pixels = %d, want text and lines on the screen", dark)
	}
}

func TestRender_EmptyDayStillWritesScreen(t *testing.T) {
	loadTestFont(t)
	empty := feedServer(t, http.StatusOK, ics(), nil)
	p, err := New("Europe/Kyiv", "list", []Source{{Name: "Work", URL: empty.URL}})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	fixedNow(p, time.Date(2026, 9, 16, 8, 15, 0, 0, kyiv))

	out := filepath.Join(t.TempDir(), "calendar.png")
	if err := p.Render("calendar", out, 4.0); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if w, h, dark := decodePNG(t, out); w != 800 || h != 480 || dark < 500 {
		t.Errorf("size = %dx%d dark = %d, want a full screen with the empty-state text", w, h, dark)
	}
}

func TestRender_ManyEventsFitOnScreen(t *testing.T) {
	loadTestFont(t)
	var events []string
	for i := 0; i < 20; i++ {
		hh := 6 + i%12
		events = append(events, vevent(
			"UID:m"+string(rune('a'+i)),
			"DTSTART;TZID=Europe/Kyiv:20260916T"+twoDigits(hh)+"0000",
			"DTEND;TZID=Europe/Kyiv:20260916T"+twoDigits(hh)+"3000",
			"SUMMARY:Meeting "+string(rune('A'+i)),
		))
	}
	srv := feedServer(t, http.StatusOK, ics(events...), nil)
	p, err := New("Europe/Kyiv", "list", []Source{{Name: "Work", URL: srv.URL}})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	fixedNow(p, time.Date(2026, 9, 16, 8, 15, 0, 0, kyiv))

	out := filepath.Join(t.TempDir(), "calendar.png")
	if err := p.Render("calendar", out, 4.0); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if w, h, _ := decodePNG(t, out); w != 800 || h != 480 {
		t.Errorf("size = %dx%d, want 800x480", w, h)
	}
}

func twoDigits(n int) string {
	if n < 10 {
		return "0" + string(rune('0'+n))
	}
	return string(rune('0'+n/10)) + string(rune('0'+n%10))
}
