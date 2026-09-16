package crypto

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
	"trmnl-server-go/pkg/v1/render"
)

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestCryptoPlugin_NameAndScreens(t *testing.T) {
	p := &CryptoPlugin{Symbols: []string{"bitcoin", "ethereum"}}
	if p.Name() != "coingecko" {
		t.Errorf("Name = %q, want coingecko", p.Name())
	}
	got := p.Screens()
	want := []string{"coingecko_bitcoin", "coingecko_ethereum"}
	if len(got) != len(want) {
		t.Fatalf("Screens len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Screens[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestCryptoPlugin_EmptySymbols(t *testing.T) {
	p := &CryptoPlugin{}
	if got := p.Screens(); len(got) != 0 {
		t.Errorf("Screens = %v, want empty", got)
	}
}

// withBaseURL points the package-level baseURL at srv for one test and restores it.
func withBaseURL(t *testing.T, srv *httptest.Server) {
	t.Helper()
	orig := baseURL
	baseURL = srv.URL
	t.Cleanup(func() { baseURL = orig })
}

// coinFixture mirrors the fields of a real CoinGecko /coins/{id} response that
// the screen reads.
const coinFixture = `{
	"id": "bitcoin", "symbol": "btc", "name": "Bitcoin", "market_cap_rank": 1,
	"market_data": {
		"current_price": {"usd": 75694.0, "eur": 65634},
		"high_24h": {"usd": 77146}, "low_24h": {"usd": 75038},
		"price_change_24h": -1318.4084460463491,
		"price_change_percentage_24h": -1.71198,
		"price_change_percentage_7d": -4.89189,
		"price_change_percentage_30d": 19.20655,
		"price_change_percentage_1y": -34.42744,
		"market_cap": {"usd": 1520415030055},
		"total_volume": {"usd": 38204860065},
		"ath": {"usd": 126080},
		"ath_change_percentage": {"usd": -39.96371},
		"last_updated": "2026-09-16T13:09:30.000Z"
	},
	"last_updated": "2026-09-16T13:09:30.000Z"
}`

// historyFixture holds four hourly points spanning a UTC midnight:
// 2024-01-01 22:00, 23:00, 2024-01-02 00:00, 01:00.
const historyFixture = `{"prices": [
	[1704146400000, 100.0], [1704150000000, 101.0], [1704153600000, 102.0], [1704157200000, 103.0]
]}`

func TestGetCryptoData_ParsesFieldsTheScreenNeeds(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/v3/coins/bitcoin") {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		w.Write([]byte(coinFixture))
	}))
	defer srv.Close()
	withBaseURL(t, srv)

	c, err := getCryptoData("bitcoin")
	if err != nil {
		t.Fatalf("getCryptoData: %v", err)
	}
	if c.Symbol != "btc" || c.Name != "Bitcoin" || c.MarketCapRank != 1 {
		t.Errorf("identity = %q %q #%d, want btc Bitcoin #1", c.Symbol, c.Name, c.MarketCapRank)
	}
	md := c.MarketData
	floats := []struct {
		name      string
		got, want float64
	}{
		{"CurrentPrice", md.CurrentPrice.USD, 75694},
		{"High24h", md.High24h.USD, 77146},
		{"Low24h", md.Low24h.USD, 75038},
		{"Change24h", md.Change24h, -1318.4084460463491},
		{"ChangePct24h", md.ChangePct24h, -1.71198},
		{"ChangePct7d", md.ChangePct7d, -4.89189},
		{"ChangePct30d", md.ChangePct30d, 19.20655},
		{"ChangePct1y", md.ChangePct1y, -34.42744},
		{"MarketCap", md.MarketCap.USD, 1520415030055},
		{"TotalVolume", md.TotalVolume.USD, 38204860065},
		{"AthChangePct", md.AthChangePct.USD, -39.96371},
	}
	for _, f := range floats {
		if !near(f.got, f.want) {
			t.Errorf("%s = %v, want %v", f.name, f.got, f.want)
		}
	}
	if md.LastUpdated != "2026-09-16T13:09:30.000Z" {
		t.Errorf("LastUpdated = %q", md.LastUpdated)
	}
}

func TestGetCryptoData_KeepsDecimalsForSmallCoins(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"id":"dogecoin","symbol":"doge","name":"Dogecoin","market_data":{"current_price":{"usd":0.1234}}}`))
	}))
	defer srv.Close()
	withBaseURL(t, srv)

	c, err := getCryptoData("dogecoin")
	if err != nil {
		t.Fatalf("getCryptoData: %v", err)
	}
	if !near(c.MarketData.CurrentPrice.USD, 0.1234) {
		t.Errorf("CurrentPrice = %v, want 0.1234 (decimals must survive parsing)", c.MarketData.CurrentPrice.USD)
	}
}

func TestGetCryptoData_Errors(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"invalid json", "not json"},
		{"not found", `{"error":"coin not found"}`},
		{"rate limited", `{"status":{"error_code":429,"error_message":"You've exceeded the Rate Limit."}}`},
	}
	for _, tc := range tests {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(tc.body))
		}))
		withBaseURL(t, srv)
		_, err := getCryptoData("bitcoin")
		srv.Close()
		if err == nil {
			t.Errorf("%s: expected error, got nil", tc.name)
		}
	}
}

func TestGetCryptoHistory_SevenDaysWithMidnightSeparators(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/v3/coins/bitcoin/market_chart") {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		if got := r.URL.Query().Get("days"); got != "7" {
			t.Errorf("days = %q, want 7", got)
		}
		w.Write([]byte(historyFixture))
	}))
	defer srv.Close()
	withBaseURL(t, srv)

	s, err := getCryptoHistory("bitcoin", time.UTC)
	if err != nil {
		t.Fatalf("getCryptoHistory: %v", err)
	}
	want := []float64{100, 101, 102, 103}
	if len(s.Closes) != len(want) {
		t.Fatalf("Closes = %v, want %v", s.Closes, want)
	}
	for i := range want {
		if !near(s.Closes[i], want[i]) {
			t.Errorf("Closes[%d] = %v, want %v", i, s.Closes[i], want[i])
		}
	}
	// Only day changes are marked; the partial first day gets no label, or it
	// would collide with the first midnight label a few hours later.
	if len(s.Separators) != 1 || s.Separators[2] != "01/02" {
		t.Errorf("Separators = %v, want {2:01/02}", s.Separators)
	}
}

func TestGetCryptoHistory_UsesGivenLocationForDayBoundaries(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(historyFixture))
	}))
	defer srv.Close()
	withBaseURL(t, srv)

	// In UTC+2 the points fall on Jan 2 00:00, 01:00, 02:00, 03:00: one day,
	// so no day change and no separator.
	s, err := getCryptoHistory("bitcoin", time.FixedZone("EET", 2*3600))
	if err != nil {
		t.Fatalf("getCryptoHistory: %v", err)
	}
	if len(s.Separators) != 0 {
		t.Errorf("Separators = %v, want none", s.Separators)
	}

	// In UTC-1 the midnight falls between the third and fourth points.
	s, err = getCryptoHistory("bitcoin", time.FixedZone("AZOT", -3600))
	if err != nil {
		t.Fatalf("getCryptoHistory: %v", err)
	}
	if len(s.Separators) != 1 || s.Separators[3] != "01/02" {
		t.Errorf("Separators = %v, want {3:01/02}", s.Separators)
	}
}

func TestGetCryptoHistory_Errors(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"invalid json", "not json"},
		{"no prices", `{"prices": []}`},
		{"rate limited", `{"status":{"error_code":429,"error_message":"You've exceeded the Rate Limit."}}`},
	}
	for _, tc := range tests {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(tc.body))
		}))
		withBaseURL(t, srv)
		_, err := getCryptoHistory("bitcoin", time.UTC)
		srv.Close()
		if err == nil {
			t.Errorf("%s: expected error, got nil", tc.name)
		}
	}
}

func TestFormatPrice_AdaptsPrecisionToMagnitude(t *testing.T) {
	tests := []struct {
		in   float64
		want string
	}{
		{75694, "75,694"},
		{1234567.8, "1,234,568"},
		{1000, "1,000"},
		{999.99, "999.99"},
		{12.3, "12.30"},
		{1, "1.00"},
		{0.1234, "0.1234"},
		{0.5, "0.5000"},
		{0.00001234, "0.00001234"},
		{0, "0.00"},
	}
	for _, tc := range tests {
		if got := formatPrice(tc.in); got != tc.want {
			t.Errorf("formatPrice(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestFormatSignedPrice(t *testing.T) {
	tests := []struct {
		in   float64
		want string
	}{
		{-1318.41, "-1,318"},
		{2.5, "+2.50"},
		{0.001, "0.00"},
		{-0.001, "0.00"},
	}
	for _, tc := range tests {
		if got := formatSignedPrice(tc.in); got != tc.want {
			t.Errorf("formatSignedPrice(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestFormatPct(t *testing.T) {
	tests := []struct {
		in   float64
		want string
	}{
		{19.20655, "+19.21%"},
		{-1.71198, "-1.71%"},
		{0, "0.00%"},
		{0.004, "0.00%"},
	}
	for _, tc := range tests {
		if got := formatPct(tc.in); got != tc.want {
			t.Errorf("formatPct(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestFormatBig_UsesShortSuffixes(t *testing.T) {
	tests := []struct {
		in   float64
		want string
	}{
		{1520415030055, "1.52T"},
		{38204860065, "38.2B"},
		{4432026, "4.4M"},
		{950000, "950.0K"},
		{512, "512"},
	}
	for _, tc := range tests {
		if got := formatBig(tc.in); got != tc.want {
			t.Errorf("formatBig(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestUpdatedLine(t *testing.T) {
	if got := updatedLine("2026-09-16T13:09:30.000Z", time.UTC); got != "Updated Sep 16 13:09 UTC" {
		t.Errorf("UTC = %q, want %q", got, "Updated Sep 16 13:09 UTC")
	}
	cet := time.FixedZone("CET", 3600)
	if got := updatedLine("2026-09-16T13:09:30.000Z", cet); got != "Updated Sep 16 14:09 CET" {
		t.Errorf("CET = %q, want %q", got, "Updated Sep 16 14:09 CET")
	}
	if got := updatedLine("garbage", time.UTC); got != "" {
		t.Errorf("unparsable = %q, want empty", got)
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
	mux.HandleFunc("/api/v3/coins/bitcoin/market_chart", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(historyFixture))
	})
	mux.HandleFunc("/api/v3/coins/bitcoin", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(coinFixture))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	withBaseURL(t, srv)

	p := &CryptoPlugin{Symbols: []string{"bitcoin"}}
	out := filepath.Join(t.TempDir(), "coingecko_bitcoin.png")
	if err := p.Render("coingecko_bitcoin", out, 4.0); err != nil {
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
