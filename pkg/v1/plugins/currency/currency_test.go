package currency

import (
	"image/png"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"trmnl-server-go/pkg/v1/icons"
	"trmnl-server-go/pkg/v1/render"
)

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func withBaseURL(t *testing.T, srv *httptest.Server) {
	t.Helper()
	orig := baseURL
	baseURL = srv.URL
	t.Cleanup(func() { baseURL = orig })
}

// seriesFixture mirrors a real Frankfurter time-series response for the
// currencies used by the first example screen.
const seriesFixture = `{
	"amount": 1.0,
	"base": "EUR",
	"start_date": "2026-09-11",
	"end_date": "2026-09-15",
	"rates": {
		"2026-09-11": {"CHF": 0.9402, "GBP": 0.8511, "PLN": 4.325, "USD": 1.1610},
		"2026-09-14": {"CHF": 0.9430, "GBP": 0.8549, "PLN": 4.3418, "USD": 1.1560},
		"2026-09-15": {"CHF": 0.9441, "GBP": 0.8558, "PLN": 4.34, "USD": 1.1539}
	}
}`

func TestNew_NamesScreensByPosition(t *testing.T) {
	p, err := New([][]string{{"EUR/PLN"}, {"USD/PLN", "GBP/PLN"}})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if p.Name() != "currency" {
		t.Errorf("Name = %q, want currency", p.Name())
	}
	got := p.Screens()
	want := []string{"currency_1", "currency_2"}
	if len(got) != len(want) {
		t.Fatalf("Screens = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Screens[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestNew_AcceptsFourLowercasePairs(t *testing.T) {
	if _, err := New([][]string{{"eur/pln", "usd/pln", "gbp/pln", "chf/pln"}}); err != nil {
		t.Fatalf("New: %v", err)
	}
}

func TestNew_RejectsInvalidScreens(t *testing.T) {
	tests := []struct {
		name    string
		screens [][]string
	}{
		{"no screens", nil},
		{"empty screen", [][]string{{}}},
		{"five pairs", [][]string{{"EUR/PLN", "USD/PLN", "GBP/PLN", "CHF/PLN", "JPY/PLN"}}},
		{"missing slash", [][]string{{"EURPLN"}}},
		{"unknown code", [][]string{{"EUR/XXX"}}},
		{"same code twice", [][]string{{"EUR/EUR"}}},
		{"bad pair on second screen", [][]string{{"EUR/PLN"}, {"EUR/PLN", "bogus"}}},
	}
	for _, tc := range tests {
		if _, err := New(tc.screens); err == nil {
			t.Errorf("%s: expected error, got nil", tc.name)
		}
	}
}

func TestParsePair_NormalizesCase(t *testing.T) {
	got, err := parsePair(" eur/pln ")
	if err != nil {
		t.Fatalf("parsePair: %v", err)
	}
	if got.Base != "EUR" || got.Quote != "PLN" {
		t.Errorf("parsePair = %+v, want Base EUR Quote PLN", got)
	}
}

func TestCrossRate_DerivesAnyPairFromEURRates(t *testing.T) {
	rates := map[string]float64{"PLN": 4.0, "USD": 2.0}
	tests := []struct {
		pair Pair
		want float64
	}{
		{Pair{"EUR", "PLN"}, 4.0},
		{Pair{"PLN", "EUR"}, 0.25},
		{Pair{"USD", "PLN"}, 2.0},
		{Pair{"PLN", "USD"}, 0.5},
		{Pair{"USD", "EUR"}, 0.5},
	}
	for _, tc := range tests {
		got, err := crossRate(rates, tc.pair)
		if err != nil {
			t.Errorf("crossRate(%s): %v", tc.pair, err)
			continue
		}
		if !near(got, tc.want) {
			t.Errorf("crossRate(%s) = %v, want %v", tc.pair, got, tc.want)
		}
	}
}

func TestCrossRate_MissingCodeIsAnError(t *testing.T) {
	if _, err := crossRate(map[string]float64{"PLN": 4.0}, Pair{"USD", "PLN"}); err == nil {
		t.Fatal("expected error when a code is missing from the rates")
	}
}

func TestComputeStats_OrdersDatesAndDerivesChange(t *testing.T) {
	s := series{Rates: map[string]map[string]float64{
		"2026-09-03": {"PLN": 4.41},
		"2026-09-01": {"PLN": 4.0},
		"2026-09-02": {"PLN": 4.2},
	}}

	got, err := computeStats(s, Pair{"EUR", "PLN"})
	if err != nil {
		t.Fatalf("computeStats: %v", err)
	}
	if !near(got.Rate, 4.41) {
		t.Errorf("Rate = %v, want 4.41", got.Rate)
	}
	// 4.41 / 4.2 = 1.05 -> +5%
	if !near(got.Change, 5.0) {
		t.Errorf("Change = %v, want 5", got.Change)
	}
	want := []float64{4.0, 4.2, 4.41}
	if len(got.History) != len(want) {
		t.Fatalf("History = %v, want %v", got.History, want)
	}
	for i := range want {
		if !near(got.History[i], want[i]) {
			t.Errorf("History[%d] = %v, want %v", i, got.History[i], want[i])
		}
	}
}

func TestComputeStats_CrossPair(t *testing.T) {
	s := series{Rates: map[string]map[string]float64{
		"2026-09-01": {"USD": 2.0, "PLN": 4.0},
		"2026-09-02": {"USD": 2.0, "PLN": 5.0},
	}}

	got, err := computeStats(s, Pair{"USD", "PLN"})
	if err != nil {
		t.Fatalf("computeStats: %v", err)
	}
	if !near(got.Rate, 2.5) {
		t.Errorf("Rate = %v, want 2.5", got.Rate)
	}
	if !near(got.Change, 25.0) {
		t.Errorf("Change = %v, want 25", got.Change)
	}
}

func TestComputeStats_SingleDateHasZeroChange(t *testing.T) {
	s := series{Rates: map[string]map[string]float64{"2026-09-01": {"PLN": 4.0}}}

	got, err := computeStats(s, Pair{"EUR", "PLN"})
	if err != nil {
		t.Fatalf("computeStats: %v", err)
	}
	if !near(got.Rate, 4.0) || !near(got.Change, 0) || len(got.History) != 1 {
		t.Errorf("stats = %+v, want Rate 4 Change 0 History len 1", got)
	}
}

func TestComputeStats_Errors(t *testing.T) {
	if _, err := computeStats(series{}, Pair{"EUR", "PLN"}); err == nil {
		t.Error("expected error for a series with no dates")
	}
	missing := series{Rates: map[string]map[string]float64{
		"2026-09-01": {"PLN": 4.0},
		"2026-09-02": {},
	}}
	if _, err := computeStats(missing, Pair{"EUR", "PLN"}); err == nil {
		t.Error("expected error when a date lacks the requested code")
	}
}

func TestFormatRate_UsesFourDecimalsBelow100AndTwoAbove(t *testing.T) {
	tests := []struct {
		in   float64
		want string
	}{
		{4.34, "4.3400"},
		{0.8558, "0.8558"},
		{99.5, "99.5000"},
		{100, "100.00"},
		{157.31, "157.31"},
	}
	for _, tc := range tests {
		if got := formatRate(tc.in); got != tc.want {
			t.Errorf("formatRate(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestFormatChange_SignedPercentWithTwoDecimals(t *testing.T) {
	tests := []struct {
		in   float64
		want string
	}{
		{0.13, "+0.13%"},
		{5, "+5.00%"},
		{-0.4249, "-0.42%"},
		{0, "0.00%"},
		{0.004, "0.00%"},
		{-0.004, "0.00%"},
	}
	for _, tc := range tests {
		if got := formatChange(tc.in); got != tc.want {
			t.Errorf("formatChange(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestTrendIcon(t *testing.T) {
	tests := []struct {
		in   float64
		want string
	}{
		{0.13, icons.TrendUp},
		{0.005, icons.TrendUp},
		{-0.13, icons.TrendDown},
		{-0.005, icons.TrendDown},
		{0, icons.TrendFlat},
		{0.004, icons.TrendFlat},
		{-0.004, icons.TrendFlat},
	}
	for _, tc := range tests {
		if got := trendIcon(tc.in); got != tc.want {
			t.Errorf("trendIcon(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestScreenIndex(t *testing.T) {
	good := map[string]int{"currency_1": 0, "currency_12": 11}
	for in, want := range good {
		got, err := screenIndex(in)
		if err != nil {
			t.Errorf("screenIndex(%q): %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("screenIndex(%q) = %d, want %d", in, got, want)
		}
	}
	for _, in := range []string{"currency_0", "currency_", "currency_x", "weather", "currency_-1"} {
		if _, err := screenIndex(in); err == nil {
			t.Errorf("screenIndex(%q): expected error", in)
		}
	}
}

func TestScreenCurrencies_SortedUniqueWithoutEUR(t *testing.T) {
	pairs := []Pair{{"EUR", "PLN"}, {"USD", "PLN"}, {"PLN", "CHF"}}

	got := screenCurrencies(pairs)

	want := []string{"CHF", "PLN", "USD"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("screenCurrencies = %v, want %v", got, want)
	}
}

func TestFetchSeries_RequestsEURBasedRangeAndParsesRates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/2026-08-17..2026-09-16" {
			t.Errorf("path = %q, want /v1/2026-08-17..2026-09-16", r.URL.Path)
		}
		if got := r.URL.Query().Get("base"); got != "EUR" {
			t.Errorf("base = %q, want EUR", got)
		}
		if got := r.URL.Query().Get("symbols"); got != "PLN,USD" {
			t.Errorf("symbols = %q, want PLN,USD", got)
		}
		w.Write([]byte(seriesFixture))
	}))
	defer srv.Close()
	withBaseURL(t, srv)

	from := time.Date(2026, 8, 17, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 9, 16, 12, 30, 0, 0, time.UTC)
	s, err := fetchSeries([]string{"PLN", "USD"}, from, to)
	if err != nil {
		t.Fatalf("fetchSeries: %v", err)
	}
	if len(s.Rates) != 3 {
		t.Fatalf("len(Rates) = %d, want 3", len(s.Rates))
	}
	if got := s.Rates["2026-09-15"]["PLN"]; !near(got, 4.34) {
		t.Errorf("Rates[2026-09-15][PLN] = %v, want 4.34", got)
	}
}

func TestFetchSeries_Errors(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"invalid json", "not json"},
		{"api error message", `{"message": "not found"}`},
		{"no rates", `{"base": "EUR", "rates": {}}`},
	}
	for _, tc := range tests {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(tc.body))
		}))
		withBaseURL(t, srv)

		_, err := fetchSeries([]string{"PLN"}, time.Now().AddDate(0, 0, -30), time.Now())
		srv.Close()
		if err == nil {
			t.Errorf("%s: expected error, got nil", tc.name)
		}
	}
}

func TestRender_UnknownScreenDoesNotHitNetwork(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request %s", r.URL)
	}))
	defer srv.Close()
	withBaseURL(t, srv)

	p, err := New([][]string{{"EUR/PLN"}})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := p.Render("currency_2", filepath.Join(t.TempDir(), "out.png"), 4.0); err == nil {
		t.Fatal("expected error for a screen index past the configured screens")
	}
}

func TestRender_FetchFailureIsReturned(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"message": "not found"}`))
	}))
	defer srv.Close()
	withBaseURL(t, srv)

	p, err := New([][]string{{"EUR/PLN"}})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	out := filepath.Join(t.TempDir(), "out.png")
	if err := p.Render("currency_1", out, 4.0); err == nil {
		t.Fatal("expected fetch error from Render")
	}
	if _, statErr := os.Stat(out); statErr == nil {
		t.Error("no PNG should be written when the fetch fails")
	}
}

// TestRender_WritesFullScreenPNG renders a real screen against a fake API.
// It needs the repo-root font.ttf for text; without it the test skips. The
// icon font cache is seeded with the same TTF so no network is hit.
func TestRender_WritesFullScreenPNG(t *testing.T) {
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

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(seriesFixture))
	}))
	defer srv.Close()
	withBaseURL(t, srv)

	p, err := New([][]string{{"EUR/PLN", "USD/PLN", "GBP/PLN", "CHF/PLN"}})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	out := filepath.Join(t.TempDir(), "currency_1.png")
	if err := p.Render("currency_1", out, 4.0); err != nil {
		t.Fatalf("Render: %v", err)
	}

	f, err := os.Open(out)
	if err != nil {
		t.Fatalf("open output: %v", err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if b := img.Bounds(); b.Dx() != 800 || b.Dy() != 480 {
		t.Fatalf("bounds = %v, want 800x480", b)
	}
	// Every quadrant must contain drawn (dark) pixels: text or sparkline.
	for _, q := range [][2]int{{0, 0}, {400, 0}, {0, 240}, {400, 240}} {
		dark := 0
		for y := q[1]; y < q[1]+240; y++ {
			for x := q[0]; x < q[0]+400; x++ {
				r, _, _, _ := img.At(x, y).RGBA()
				if r < 0x8000 {
					dark++
				}
			}
		}
		if dark == 0 {
			t.Errorf("quadrant at %v has no dark pixels", q)
		}
	}
}
