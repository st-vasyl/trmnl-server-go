package stocks

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

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

func TestGetLatestPrice(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/price") {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		if r.URL.Query().Get("apikey") != "k" {
			t.Errorf("apikey = %q, want k", r.URL.Query().Get("apikey"))
		}
		if r.URL.Query().Get("symbol") != "AAPL" {
			t.Errorf("symbol = %q, want AAPL", r.URL.Query().Get("symbol"))
		}
		w.Write([]byte(`{"price": "150.25"}`))
	}))
	defer srv.Close()
	withBaseURL(t, srv)

	p, err := getLatestPrice("AAPL", "k")
	if err != nil {
		t.Fatalf("getLatestPrice: %v", err)
	}
	if p.Price != "150.25" {
		t.Errorf("Price = %q, want 150.25", p.Price)
	}
}

func TestGetQuote(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/quote") {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		w.Write([]byte(`{
			"symbol":"AAPL",
			"name":"Apple Inc",
			"high":"151.00",
			"low":"148.50",
			"fifty_two_week":{"high":"200","low":"100"}
		}`))
	}))
	defer srv.Close()
	withBaseURL(t, srv)

	q, err := getQuote("AAPL", "k")
	if err != nil {
		t.Fatalf("getQuote: %v", err)
	}
	if q.Name != "Apple Inc" {
		t.Errorf("Name = %q, want Apple Inc", q.Name)
	}
	if q.FiftyTwoWeek.High != "200" {
		t.Errorf("FiftyTwoWeek.High = %q, want 200", q.FiftyTwoWeek.High)
	}
}

func TestGetHistory_ParsesValuesAndPopulatesLabels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The history is consumed in reverse; this fixture provides two entries
		// from two different days, so we expect 2 records and at least 2 labels.
		w.Write([]byte(`{
			"meta": {"symbol": "AAPL", "interval": "30min"},
			"status": "ok",
			"values": [
				{"datetime": "2024-01-02 10:00:00", "high": "151.0", "low": "149.0"},
				{"datetime": "2024-01-01 10:00:00", "high": "150.5", "low": "148.0"}
			]
		}`))
	}))
	defer srv.Close()
	withBaseURL(t, srv)

	records, err := getHistory("AAPL", "k")
	if err != nil {
		t.Fatalf("getHistory: %v", err)
	}
	if len(records.BoxPlotRecord) != 2 {
		t.Fatalf("records = %d, want 2", len(records.BoxPlotRecord))
	}

	// The implementation iterates n-1-j, so the oldest entry (index 1 in JSON,
	// Jan 1) is consumed first and gets T=0.
	first := records.BoxPlotRecord[0]
	if first.Vmin != 148.0 || first.Vmax != 150.5 {
		t.Errorf("first record = %+v, want Vmin=148.0 Vmax=150.5", first)
	}
	if len(records.XLabels) == 0 {
		t.Error("XLabels is empty, want at least one entry")
	}
}

func TestGetLatestPrice_InvalidJSONReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`not json`))
	}))
	defer srv.Close()
	withBaseURL(t, srv)

	if _, err := getLatestPrice("AAPL", "k"); err == nil {
		t.Fatal("expected JSON parse error")
	}
}
