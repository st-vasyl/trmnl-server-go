package currency

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	pluginName        = "currency"
	maxPairsPerScreen = 4

	// trendDays is how much history each screen fetches for its sparklines.
	trendDays = 30
)

// knownCurrencies is the set of ISO codes Frankfurter v2 currently publishes
// (165 active currencies blended from about a hundred central banks; taken
// from https://api.frankfurter.dev/v2/currencies on 2026-09-16). Validating
// against it at startup turns a typo in config.yaml into an immediate error
// instead of an empty screen. Regenerate from that endpoint when Frankfurter
// adds a currency.
var knownCurrencies = map[string]bool{
	"AED": true, "AFN": true, "ALL": true, "AMD": true, "ANG": true, "AOA": true, "ARS": true, "AUD": true,
	"AWG": true, "AZN": true, "BAM": true, "BBD": true, "BDT": true, "BHD": true, "BIF": true, "BMD": true,
	"BND": true, "BOB": true, "BRL": true, "BSD": true, "BTN": true, "BWP": true, "BYN": true, "BZD": true,
	"CAD": true, "CDF": true, "CHF": true, "CLP": true, "CNH": true, "CNY": true, "COP": true, "CRC": true,
	"CUP": true, "CVE": true, "CZK": true, "DJF": true, "DKK": true, "DOP": true, "DZD": true, "EGP": true,
	"ERN": true, "ETB": true, "EUR": true, "FJD": true, "FKP": true, "GBP": true, "GEL": true, "GGP": true,
	"GHS": true, "GIP": true, "GMD": true, "GNF": true, "GTQ": true, "GYD": true, "HKD": true, "HNL": true,
	"HTG": true, "HUF": true, "IDR": true, "ILS": true, "IMP": true, "INR": true, "IQD": true, "IRR": true,
	"ISK": true, "JEP": true, "JMD": true, "JOD": true, "JPY": true, "KES": true, "KGS": true, "KHR": true,
	"KMF": true, "KPW": true, "KRW": true, "KWD": true, "KYD": true, "KZT": true, "LAK": true, "LBP": true,
	"LKR": true, "LRD": true, "LSL": true, "LYD": true, "MAD": true, "MDL": true, "MGA": true, "MKD": true,
	"MMK": true, "MNT": true, "MOP": true, "MRO": true, "MRU": true, "MUR": true, "MVR": true, "MWK": true,
	"MXN": true, "MYR": true, "MZN": true, "NAD": true, "NGN": true, "NIO": true, "NOK": true, "NPR": true,
	"NZD": true, "OMR": true, "PAB": true, "PEN": true, "PGK": true, "PHP": true, "PKR": true, "PLN": true,
	"PYG": true, "QAR": true, "RON": true, "RSD": true, "RUB": true, "RWF": true, "SAR": true, "SBD": true,
	"SCR": true, "SDG": true, "SEK": true, "SGD": true, "SHP": true, "SLE": true, "SOS": true, "SRD": true,
	"SSP": true, "STN": true, "SVC": true, "SYP": true, "SZL": true, "THB": true, "TJS": true, "TMT": true,
	"TND": true, "TOP": true, "TRY": true, "TTD": true, "TWD": true, "TZS": true, "UAH": true, "UGX": true,
	"USD": true, "UYU": true, "UZS": true, "VES": true, "VND": true, "VUV": true, "WST": true, "XAF": true,
	"XAG": true, "XAU": true, "XCD": true, "XCG": true, "XDR": true, "XOF": true, "XPD": true, "XPF": true,
	"XPT": true, "YER": true, "ZAR": true, "ZMW": true, "ZWG": true,
}

// Pair is one currency pair: Base priced in Quote, so EUR/UAH is the number
// of UAH one EUR buys.
type Pair struct {
	Base  string
	Quote string
}

func (p Pair) String() string { return p.Base + "/" + p.Quote }

// Plugin renders screens of up to four currency pairs each.
type Plugin struct {
	screens [][]Pair
}

// New validates the configured screens, a list of pair lists such as
// [["EUR/UAH", "USD/UAH"]], and returns the plugin. It fails on an empty
// configuration, an empty screen, more than four pairs on one screen, or any
// malformed or unknown pair.
func New(screens [][]string) (*Plugin, error) {
	if len(screens) == 0 {
		return nil, fmt.Errorf("currency: no screens configured")
	}
	p := &Plugin{}
	for i, raw := range screens {
		if len(raw) == 0 {
			return nil, fmt.Errorf("currency: screen %d has no pairs", i+1)
		}
		if len(raw) > maxPairsPerScreen {
			return nil, fmt.Errorf("currency: screen %d has %d pairs, max is %d", i+1, len(raw), maxPairsPerScreen)
		}
		pairs := make([]Pair, 0, len(raw))
		for _, s := range raw {
			pair, err := parsePair(s)
			if err != nil {
				return nil, fmt.Errorf("currency: screen %d: %w", i+1, err)
			}
			pairs = append(pairs, pair)
		}
		p.screens = append(p.screens, pairs)
	}
	return p, nil
}

func (p *Plugin) Name() string { return pluginName }

// Screens returns "currency_1" through "currency_N" in configured order.
func (p *Plugin) Screens() []string {
	names := make([]string, len(p.screens))
	for i := range p.screens {
		names[i] = fmt.Sprintf("%s_%d", pluginName, i+1)
	}
	return names
}

// screenIndex maps a screen name such as "currency_3" to its zero-based index.
func screenIndex(screen string) (int, error) {
	num, ok := strings.CutPrefix(screen, pluginName+"_")
	if !ok {
		return 0, fmt.Errorf("currency: unknown screen %q", screen)
	}
	n, err := strconv.Atoi(num)
	if err != nil || n < 1 {
		return 0, fmt.Errorf("currency: unknown screen %q", screen)
	}
	return n - 1, nil
}

// Render fetches trendDays of rates for the screen's currencies and draws the
// two-by-two grid to outputPath. Nothing is written when the fetch fails.
func (p *Plugin) Render(screen, outputPath string, voltage float32) error {
	i, err := screenIndex(screen)
	if err != nil {
		return err
	}
	if i >= len(p.screens) {
		return fmt.Errorf("currency: screen %q is not configured (have %d)", screen, len(p.screens))
	}
	pairs := p.screens[i]

	now := time.Now()
	s, err := fetchSeries(screenCurrencies(pairs), now.AddDate(0, 0, -trendDays), now)
	if err != nil {
		return err
	}

	stats := make([]pairStats, len(pairs))
	for j, pair := range pairs {
		st, err := computeStats(s, pair)
		if err != nil {
			return err
		}
		stats[j] = st
	}
	return renderScreen(pairs, stats, outputPath, voltage)
}

// parsePair parses "eur/uah" into Pair{EUR, UAH}, validating both codes.
func parsePair(s string) (Pair, error) {
	base, quote, ok := strings.Cut(strings.ToUpper(strings.TrimSpace(s)), "/")
	if !ok {
		return Pair{}, fmt.Errorf("pair %q must look like EUR/UAH", s)
	}
	base, quote = strings.TrimSpace(base), strings.TrimSpace(quote)
	for _, code := range []string{base, quote} {
		if !knownCurrencies[code] {
			return Pair{}, fmt.Errorf("pair %q: unknown currency %q", s, code)
		}
	}
	if base == quote {
		return Pair{}, fmt.Errorf("pair %q: base and quote are the same", s)
	}
	return Pair{Base: base, Quote: quote}, nil
}
