package currency

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
	"trmnl-server-go/pkg/v1/httpclient"

	"github.com/rs/zerolog/log"
)

// baseURL is the Frankfurter API root. Overridden in tests.
var baseURL = "https://api.frankfurter.dev"

// series is the Frankfurter time-series response, indexed rates[date][code].
type series struct {
	Base    string                        `json:"base"`
	Rates   map[string]map[string]float64 `json:"rates"`
	Message string                        `json:"message"` // set on API errors such as "not found"
}

// screenCurrencies lists the non-EUR codes a screen needs, sorted and unique.
// EUR is the request base, so it never appears in the symbols parameter.
func screenCurrencies(pairs []Pair) []string {
	seen := map[string]bool{}
	var codes []string
	for _, p := range pairs {
		for _, c := range []string{p.Base, p.Quote} {
			if c == "EUR" || seen[c] {
				continue
			}
			seen[c] = true
			codes = append(codes, c)
		}
	}
	sort.Strings(codes)
	return codes
}

// fetchSeries downloads EUR-based daily rates for codes between from and to,
// inclusive by calendar date.
func fetchSeries(codes []string, from, to time.Time) (series, error) {
	var s series
	url := fmt.Sprintf("%s/v1/%s..%s?base=EUR&symbols=%s",
		baseURL, from.Format("2006-01-02"), to.Format("2006-01-02"), strings.Join(codes, ","))

	body, err := httpclient.Get(url)
	if err != nil {
		log.Error().Str("plugin", pluginName).Err(err).Msg("Failed to fetch exchange rates")
		return s, err
	}
	if err := json.Unmarshal(body, &s); err != nil {
		log.Error().Str("plugin", pluginName).Err(err).Msg("Failed to parse exchange rates")
		return s, err
	}
	if s.Message != "" {
		return s, fmt.Errorf("frankfurter: %s", s.Message)
	}
	if len(s.Rates) == 0 {
		return s, fmt.Errorf("frankfurter: no rates returned")
	}
	return s, nil
}

// pairStats is everything one screen cell displays for a pair.
type pairStats struct {
	Rate    float64   // most recent rate
	Change  float64   // percent change from the previous available date
	History []float64 // rates in chronological order
}

// crossRate derives pair from EUR-based rates, treating EUR itself as 1.
func crossRate(rates map[string]float64, pair Pair) (float64, error) {
	base, err := eurRate(rates, pair.Base)
	if err != nil {
		return 0, err
	}
	quote, err := eurRate(rates, pair.Quote)
	if err != nil {
		return 0, err
	}
	return quote / base, nil
}

func eurRate(rates map[string]float64, code string) (float64, error) {
	if code == "EUR" {
		return 1, nil
	}
	r, ok := rates[code]
	if !ok {
		return 0, fmt.Errorf("no rate for %s", code)
	}
	return r, nil
}

// computeStats walks the series in date order (ISO dates sort lexically) and
// derives the latest rate, its change from the previous date, and the history.
func computeStats(s series, pair Pair) (pairStats, error) {
	if len(s.Rates) == 0 {
		return pairStats{}, fmt.Errorf("%s: no rates in series", pair)
	}
	dates := make([]string, 0, len(s.Rates))
	for d := range s.Rates {
		dates = append(dates, d)
	}
	sort.Strings(dates)

	history := make([]float64, 0, len(dates))
	for _, d := range dates {
		r, err := crossRate(s.Rates[d], pair)
		if err != nil {
			return pairStats{}, fmt.Errorf("%s on %s: %w", pair, d, err)
		}
		history = append(history, r)
	}

	st := pairStats{Rate: history[len(history)-1], History: history}
	if n := len(history); n >= 2 {
		st.Change = (history[n-1]/history[n-2] - 1) * 100
	}
	return st, nil
}
