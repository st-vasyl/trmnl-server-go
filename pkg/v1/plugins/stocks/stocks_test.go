package stocks

import (
	"image/png"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"trmnl-server-go/pkg/v1/icons"
	"trmnl-server-go/pkg/v1/render"
)

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestStocksPlugin_NameAndScreens(t *testing.T) {
	p := &StocksPlugin{Symbols: []string{"aapl", "nvda"}, ApiKey: "k"}
	if p.Name() != "twelvedata" {
		t.Errorf("Name = %q, want twelvedata", p.Name())
	}
	got := p.Screens()
	want := []string{"twelvedata_aapl", "twelvedata_nvda"}
	if len(got) != len(want) {
		t.Fatalf("Screens len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Screens[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func withBaseURL(t *testing.T, srv *httptest.Server) {
	t.Helper()
	orig := baseURL
	baseURL = srv.URL
	t.Cleanup(func() { baseURL = orig })
}

// quoteFixture mirrors a real TwelveData /quote response.
const quoteFixture = `{
	"symbol":"AAPL","name":"Apple Inc.","exchange":"NASDAQ","mic_code":"XNGS","currency":"USD",
	"datetime":"2026-09-15","timestamp":1789479000,"last_quote_at":1789502340,
	"open":"330.14001","high":"331.78000","low":"328.35001","close":"331.34000",
	"volume":"31694100","previous_close":"333.079987","change":"-1.73999","percent_change":"-0.52239411",
	"average_volume":"45658560","is_market_open":false,
	"fifty_two_week":{"low":"235.029999","high":"344.57001","low_change":"96.31000","high_change":"-13.23001",
	"low_change_percent":"40.97775","high_change_percent":"-3.83957","range":"235.029999 - 344.570007"}
}`

// historyFixture mirrors a real TwelveData /time_series response: newest bar
// first, two sessions of two 30-minute bars each.
const historyFixture = `{
	"meta":{"symbol":"AAPL","interval":"30min","currency":"USD","exchange_timezone":"America/New_York",
	"exchange":"NASDAQ","mic_code":"XNGS","type":"Common Stock"},
	"values":[
		{"datetime":"2024-01-02 10:00:00","open":"150.5","high":"151.2","low":"150.1","close":"151.0","volume":"1000"},
		{"datetime":"2024-01-02 09:30:00","open":"149.5","high":"150.4","low":"149.2","close":"150.0","volume":"1000"},
		{"datetime":"2024-01-01 10:00:00","open":"148.5","high":"149.3","low":"148.2","close":"149.0","volume":"1000"},
		{"datetime":"2024-01-01 09:30:00","open":"147.5","high":"148.4","low":"147.1","close":"148.0","volume":"1000"}
	],
	"status":"ok"
}`

func TestGetQuote_ParsesFieldsTheScreenNeeds(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/quote") {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		if r.URL.Query().Get("apikey") != "k" {
			t.Errorf("apikey = %q, want k", r.URL.Query().Get("apikey"))
		}
		if r.URL.Query().Get("symbol") != "AAPL" {
			t.Errorf("symbol = %q, want AAPL", r.URL.Query().Get("symbol"))
		}
		w.Write([]byte(quoteFixture))
	}))
	defer srv.Close()
	withBaseURL(t, srv)

	q, err := getQuote("AAPL", "k")
	if err != nil {
		t.Fatalf("getQuote: %v", err)
	}
	checks := []struct {
		name, got, want string
	}{
		{"Symbol", q.Symbol, "AAPL"},
		{"Name", q.Name, "Apple Inc."},
		{"Exchange", q.Exchange, "NASDAQ"},
		{"Currency", q.Currency, "USD"},
		{"Open", q.Open, "330.14001"},
		{"Close", q.Close, "331.34000"},
		{"PreviousClose", q.PreviousClose, "333.079987"},
		{"Change", q.Change, "-1.73999"},
		{"PercentChange", q.PercentChange, "-0.52239411"},
		{"Volume", q.Volume, "31694100"},
		{"FiftyTwoWeek.High", q.FiftyTwoWeek.High, "344.57001"},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", c.name, c.got, c.want)
		}
	}
	if q.LastQuoteAt != 1789502340 {
		t.Errorf("LastQuoteAt = %d, want 1789502340", q.LastQuoteAt)
	}
	if q.IsMarketOpen {
		t.Error("IsMarketOpen = true, want false")
	}
}

func TestGetQuote_APIErrorIsReturned(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"code":401,"message":"Invalid API key","status":"error"}`))
	}))
	defer srv.Close()
	withBaseURL(t, srv)

	if _, err := getQuote("AAPL", "bad"); err == nil {
		t.Fatal("expected error for an API error body, got nil")
	}
}

func TestGetHistory_ReturnsChronologicalClosesWithSessionSeparators(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/time_series") {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		if got := r.URL.Query().Get("interval"); got != "30min" {
			t.Errorf("interval = %q, want 30min", got)
		}
		w.Write([]byte(historyFixture))
	}))
	defer srv.Close()
	withBaseURL(t, srv)

	series, meta, err := getHistory("AAPL", "k")
	if err != nil {
		t.Fatalf("getHistory: %v", err)
	}

	wantCloses := []float64{148, 149, 150, 151}
	if len(series.Closes) != len(wantCloses) {
		t.Fatalf("Closes = %v, want %v", series.Closes, wantCloses)
	}
	for i := range wantCloses {
		if !near(series.Closes[i], wantCloses[i]) {
			t.Errorf("Closes[%d] = %v, want %v", i, series.Closes[i], wantCloses[i])
		}
	}
	if len(series.Separators) != 2 || series.Separators[0] != "01/01" || series.Separators[2] != "01/02" {
		t.Errorf("Separators = %v, want {0:01/01 2:01/02}", series.Separators)
	}
	if meta.ExchangeTimezone != "America/New_York" {
		t.Errorf("ExchangeTimezone = %q, want America/New_York", meta.ExchangeTimezone)
	}
}

func TestGetHistory_Errors(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"invalid json", "not json"},
		{"api error", `{"code":401,"message":"Invalid API key","status":"error"}`},
		{"no bars", `{"meta":{},"values":[],"status":"ok"}`},
	}
	for _, tc := range tests {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(tc.body))
		}))
		withBaseURL(t, srv)
		_, _, err := getHistory("AAPL", "k")
		srv.Close()
		if err == nil {
			t.Errorf("%s: expected error, got nil", tc.name)
		}
	}
}

func TestParseFloat_TrimsAndDefaultsToZero(t *testing.T) {
	if got := parseFloat(" 331.34000 "); !near(got, 331.34) {
		t.Errorf("parseFloat(padded) = %v, want 331.34", got)
	}
	if got := parseFloat(""); got != 0 {
		t.Errorf("parseFloat(empty) = %v, want 0", got)
	}
	if got := parseFloat("n/a"); got != 0 {
		t.Errorf("parseFloat(garbage) = %v, want 0", got)
	}
}

func TestFormatMoney_TwoDecimals(t *testing.T) {
	tests := []struct {
		in   float64
		want string
	}{
		{331.34, "331.34"},
		{1234.5, "1234.50"},
		{0.5, "0.50"},
	}
	for _, tc := range tests {
		if got := formatMoney(tc.in); got != tc.want {
			t.Errorf("formatMoney(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestFormatVolume_UsesShortSuffixes(t *testing.T) {
	tests := []struct {
		in   float64
		want string
	}{
		{31694100, "31.7M"},
		{4432026, "4.4M"},
		{950000, "950.0K"},
		{512, "512"},
		{2500000000, "2.5B"},
	}
	for _, tc := range tests {
		if got := formatVolume(tc.in); got != tc.want {
			t.Errorf("formatVolume(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestFormatChange_SignedValueAndPercent(t *testing.T) {
	tests := []struct {
		change, pct float64
		want        string
	}{
		{-1.73999, -0.52239411, "-1.74 (-0.52%)"},
		{1.73999, 0.52239411, "+1.74 (+0.52%)"},
		{0, 0, "0.00 (0.00%)"},
	}
	for _, tc := range tests {
		if got := formatChange(tc.change, tc.pct); got != tc.want {
			t.Errorf("formatChange(%v, %v) = %q, want %q", tc.change, tc.pct, got, tc.want)
		}
	}
}

func TestTrendIcon(t *testing.T) {
	tests := []struct {
		in   float64
		want string
	}{
		{1.74, icons.TrendUp},
		{-1.74, icons.TrendDown},
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

func TestQuoteStatusLine_UsesExchangeTimezone(t *testing.T) {
	// 1789502340 is 2026-09-15 19:59 UTC, which is 15:59 in New York (EDT).
	if got := quoteStatusLine(1789502340, "America/New_York", false); got != "Sep 15 15:59 · Closed" {
		t.Errorf("status line = %q, want %q", got, "Sep 15 15:59 · Closed")
	}
	if got := quoteStatusLine(1789502340, "America/New_York", true); got != "Sep 15 15:59 · Open" {
		t.Errorf("status line (open) = %q, want %q", got, "Sep 15 15:59 · Open")
	}
	// An unknown zone falls back to UTC rather than failing.
	if got := quoteStatusLine(1789502340, "Nowhere/Land", false); got != "Sep 15 19:59 · Closed" {
		t.Errorf("status line (bad tz) = %q, want %q", got, "Sep 15 19:59 · Closed")
	}
}

// TestRender_WritesFullScreenPNG renders a real screen against fake API
// endpoints. It needs the repo-root font.ttf for text; without it the test
// skips. The icon font cache is seeded with the same TTF so no network is hit.
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

	mux := http.NewServeMux()
	mux.HandleFunc("/quote", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(quoteFixture)) })
	mux.HandleFunc("/time_series", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(historyFixture)) })
	mux.HandleFunc("/price", func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("the /price endpoint should no longer be called")
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	withBaseURL(t, srv)

	p := &StocksPlugin{Symbols: []string{"AAPL"}, ApiKey: "k"}
	out := filepath.Join(t.TempDir(), "twelvedata_AAPL.png")
	if err := p.Render("twelvedata_AAPL", out, 4.0); err != nil {
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

	// Header, chart, and footer bands must each contain drawn pixels.
	bands := map[string][2]int{"header": {0, 140}, "chart": {190, 415}, "footer": {430, 480}}
	for name, yr := range bands {
		dark := 0
		for y := yr[0]; y < yr[1]; y++ {
			for x := 0; x < 800; x++ {
				r, _, _, _ := img.At(x, y).RGBA()
				if r < 0x8000 {
					dark++
				}
			}
		}
		if dark == 0 {
			t.Errorf("%s band (y %d-%d) has no dark pixels", name, yr[0], yr[1])
		}
	}
}
