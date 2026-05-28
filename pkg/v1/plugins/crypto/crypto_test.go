package crypto

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

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

func TestGetCryptoData_ParsesResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/v3/coins/bitcoin") {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		w.Write([]byte(`{
			"id": "bitcoin",
			"name": "Bitcoin",
			"market_data": {
				"current_price": {"usd": 50000},
				"high_24h":     {"usd": 51000},
				"low_24h":      {"usd": 49000},
				"price_change_percentage_30d": 5.5,
				"price_change_percentage_1y": 25.0
			}
		}`))
	}))
	defer srv.Close()
	withBaseURL(t, srv)

	c, err := getCryptoData("bitcoin")
	if err != nil {
		t.Fatalf("getCryptoData: %v", err)
	}
	if c.Name != "Bitcoin" {
		t.Errorf("Name = %q, want Bitcoin", c.Name)
	}
	if c.MarketData.CurrentPrice.USD != 50000 {
		t.Errorf("CurrentPrice = %d, want 50000", c.MarketData.CurrentPrice.USD)
	}
	if c.MarketData.High24h.USD != 51000 {
		t.Errorf("High24h = %d, want 51000", c.MarketData.High24h.USD)
	}
	if c.MarketData.DiffPercentage1y != 25.0 {
		t.Errorf("DiffPercentage1y = %v, want 25.0", c.MarketData.DiffPercentage1y)
	}
}

func TestGetCryptoData_InvalidJSONReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`not json`))
	}))
	defer srv.Close()
	withBaseURL(t, srv)

	if _, err := getCryptoData("bitcoin"); err == nil {
		t.Fatal("expected JSON parse error")
	}
}

func TestGetCryptoHistory_ParsesPrices(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"prices": [[1700000000000, 50000.5], [1700003600000, 50100.0]]}`))
	}))
	defer srv.Close()
	withBaseURL(t, srv)

	records, err := getCryptoHistory("bitcoin")
	if err != nil {
		t.Fatalf("getCryptoHistory: %v", err)
	}
	if got := len(records.ChartRecord); got != 2 {
		t.Fatalf("records = %d, want 2", got)
	}
	if records.ChartRecord[0].T != 1700000000000 {
		t.Errorf("first T = %v, want 1700000000000", records.ChartRecord[0].T)
	}
	if records.ChartRecord[0].V != 50000.5 {
		t.Errorf("first V = %v, want 50000.5", records.ChartRecord[0].V)
	}
}

func TestGetCryptoHistory_EmptyPrices(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"prices": []}`))
	}))
	defer srv.Close()
	withBaseURL(t, srv)

	records, err := getCryptoHistory("bitcoin")
	if err != nil {
		t.Fatalf("getCryptoHistory: %v", err)
	}
	if len(records.ChartRecord) != 0 {
		t.Errorf("records = %d, want 0", len(records.ChartRecord))
	}
}
