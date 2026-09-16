package stocks

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"strconv"
	"strings"
	"time"
	"trmnl-server-go/pkg/v1/httpclient"
	"trmnl-server-go/pkg/v1/icons"
	"trmnl-server-go/pkg/v1/render"

	"github.com/rs/zerolog/log"
)

// baseURL is the TwelveData API root. Overridden in tests.
var baseURL = "https://api.twelvedata.com"

// StocksPlugin renders a quote summary and a 7-day close-price chart for each
// configured symbol.
type StocksPlugin struct {
	Symbols []string
	ApiKey  string
}

func (p *StocksPlugin) Name() string { return "twelvedata" }

func (p *StocksPlugin) Screens() []string {
	screens := make([]string, len(p.Symbols))
	for i, s := range p.Symbols {
		screens[i] = fmt.Sprintf("twelvedata_%s", s)
	}
	return screens
}

func (p *StocksPlugin) Render(screen, outputPath string, voltage float32) error {
	symbol := strings.TrimPrefix(screen, "twelvedata_")
	return renderScreen(symbol, p.ApiKey, outputPath, voltage)
}

// stock is the TwelveData /quote response. Numbers arrive as strings. Status
// and Message are only set on error bodies such as an invalid API key.
type stock struct {
	Status        string       `json:"status"`
	Message       string       `json:"message"`
	Symbol        string       `json:"symbol"`
	Name          string       `json:"name"`
	Exchange      string       `json:"exchange"`
	Currency      string       `json:"currency"`
	LastQuoteAt   int64        `json:"last_quote_at"`
	IsMarketOpen  bool         `json:"is_market_open"`
	Open          string       `json:"open"`
	High          string       `json:"high"`
	Low           string       `json:"low"`
	Close         string       `json:"close"`
	Volume        string       `json:"volume"`
	PreviousClose string       `json:"previous_close"`
	Change        string       `json:"change"`
	PercentChange string       `json:"percent_change"`
	AverageVolume string       `json:"average_volume"`
	FiftyTwoWeek  fiftyTwoWeek `json:"fifty_two_week"`
}

type fiftyTwoWeek struct {
	Low  string `json:"low"`
	High string `json:"high"`
}

type historyRecords struct {
	Meta    historyMeta     `json:"meta"`
	Status  string          `json:"status"`
	Message string          `json:"message"`
	Values  []historyRecord `json:"values"`
}

type historyRecord struct {
	Datetime string `json:"datetime"`
	Open     string `json:"open"`
	High     string `json:"high"`
	Low      string `json:"low"`
	Close    string `json:"close"`
	Volume   string `json:"volume"`
}

type historyMeta struct {
	Symbol           string `json:"symbol"`
	Interval         string `json:"interval"`
	Currency         string `json:"currency"`
	ExchangeTimezone string `json:"exchange_timezone"`
	Exchange         string `json:"exchange"`
	MicCode          string `json:"mic_code"`
	Type             string `json:"type"`
}

func getQuote(symbol, apiKey string) (stock, error) {
	var s stock
	url := fmt.Sprintf("%s/quote?symbol=%s&apikey=%s", baseURL, symbol, apiKey)
	body, err := httpclient.Get(url)
	if err != nil {
		log.Error().Str("plugin", "stocks").Str("symbol", symbol).Err(err).Msg("Failed to fetch quote")
		return s, err
	}
	if err := json.Unmarshal(body, &s); err != nil {
		log.Error().Str("plugin", "stocks").Str("symbol", symbol).Err(err).Msg("Failed to parse quote")
		return s, err
	}
	if s.Status == "error" {
		return s, fmt.Errorf("twelvedata: %s", s.Message)
	}
	return s, nil
}

// getHistory fetches 7 days of 30-minute bars and returns the closes in
// chronological order with a separator at the first bar of each session.
func getHistory(symbol, apiKey string) (render.CloseSeries, historyMeta, error) {
	var hr historyRecords
	series := render.CloseSeries{Separators: map[int]string{}}

	now := time.Now()
	weekAgo := now.AddDate(0, 0, -7)
	url := fmt.Sprintf("%s/time_series?symbol=%s&interval=30min&start_date=%s&end_date=%s&apikey=%s",
		baseURL, symbol, weekAgo.Format("2006-01-02"), now.Format("2006-01-02T15:04:05"), apiKey)

	body, err := httpclient.Get(url)
	if err != nil {
		log.Error().Str("plugin", "stocks").Str("symbol", symbol).Err(err).Msg("Failed to fetch history")
		return series, hr.Meta, err
	}
	if err := json.Unmarshal(body, &hr); err != nil {
		log.Error().Str("plugin", "stocks").Str("symbol", symbol).Err(err).Msg("Failed to parse history")
		return series, hr.Meta, err
	}
	if hr.Status == "error" {
		return series, hr.Meta, fmt.Errorf("twelvedata: %s", hr.Message)
	}
	if len(hr.Values) == 0 {
		return series, hr.Meta, fmt.Errorf("twelvedata: no bars returned for %s", symbol)
	}

	n := len(hr.Values)
	prevDay := ""
	for j := 0; j < n; j++ {
		v := hr.Values[n-1-j] // the API returns the newest bar first
		series.Closes = append(series.Closes, parseFloat(v.Close))
		if t, err := time.Parse("2006-01-02 15:04:05", v.Datetime); err == nil {
			day := t.Format("2006-01-02")
			if day != prevDay {
				series.Separators[j] = t.Format("01/02")
				prevDay = day
			}
		}
	}
	return series, hr.Meta, nil
}

// parseFloat reads TwelveData's stringly numbers; garbage becomes 0.
func parseFloat(s string) float64 {
	v, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return v
}

func formatMoney(v float64) string { return fmt.Sprintf("%.2f", v) }

// formatVolume shortens share counts to K, M or B with one decimal.
func formatVolume(v float64) string {
	switch {
	case v >= 1e9:
		return fmt.Sprintf("%.1fB", v/1e9)
	case v >= 1e6:
		return fmt.Sprintf("%.1fM", v/1e6)
	case v >= 1e3:
		return fmt.Sprintf("%.1fK", v/1e3)
	default:
		return fmt.Sprintf("%.0f", v)
	}
}

// formatChange prints "+1.74 (+0.52%)"; moves too small to show print as 0.00.
func formatChange(change, pct float64) string {
	return fmt.Sprintf("%s (%s%%)", signed(change), signed(pct))
}

func signed(v float64) string {
	switch {
	case v >= 0.005:
		return fmt.Sprintf("+%.2f", v)
	case v <= -0.005:
		return fmt.Sprintf("%.2f", v)
	default:
		return "0.00"
	}
}

// trendIcon picks the up, down, or flat glyph for a price change.
func trendIcon(change float64) string {
	switch {
	case change >= 0.005:
		return icons.TrendUp
	case change <= -0.005:
		return icons.TrendDown
	default:
		return icons.TrendFlat
	}
}

// quoteStatusLine formats the last quote time in the exchange's zone (UTC if
// unknown) followed by the market status, e.g. "Sep 15 15:59 · Closed".
func quoteStatusLine(unix int64, tz string, open bool) string {
	loc, err := time.LoadLocation(tz)
	if err != nil || tz == "" {
		loc = time.UTC
	}
	status := "Closed"
	if open {
		status = "Open"
	}
	return fmt.Sprintf("%s · %s", time.Unix(unix, 0).In(loc).Format("Jan 2 15:04"), status)
}

func renderScreen(symbol, apiKey, outputPath string, voltage float32) error {
	q, err := getQuote(symbol, apiKey)
	if err != nil {
		return err
	}
	history, meta, err := getHistory(symbol, apiKey)
	if err != nil {
		return err
	}
	history.Reference = parseFloat(q.PreviousClose)

	img := render.NewImage(800, 480)
	black := color.Black

	// Header: ticker in large type, then name and exchange. The battery icon
	// occupies the top-right 50 pixels, so the subtitle must stop before it.
	ticker := q.Symbol
	if ticker == "" {
		ticker = strings.ToUpper(symbol)
	}
	if err := render.AddText(img, ticker, image.Point{20, 52}, black, 44); err != nil {
		return err
	}
	tickerW, err := render.TextWidth(ticker, 44)
	if err != nil {
		return err
	}
	subX := 20 + tickerW + 20
	subtitle := q.Name
	if q.Exchange != "" {
		subtitle += " · " + q.Exchange
	}
	if w, _ := render.TextWidth(subtitle, 24); subX+w > 740 {
		subtitle = q.Name
	}
	if err := render.AddText(img, subtitle, image.Point{subX, 52}, black, 24); err != nil {
		return err
	}

	// Price row: price with currency, then trend icon and change.
	price := parseFloat(q.Close)
	priceText := formatMoney(price)
	if q.Currency != "" {
		priceText += " " + q.Currency
	}
	if err := render.AddText(img, priceText, image.Point{20, 122}, black, 56); err != nil {
		return err
	}
	priceW, err := render.TextWidth(priceText, 56)
	if err != nil {
		return err
	}
	change := parseFloat(q.Change)
	changeX := 20 + priceW + 30
	// AddIcon takes the negated destination position (see render.AddIcon).
	if err := render.AddIcon(img, trendIcon(change), image.Point{-changeX, -86}, 40); err != nil {
		return err
	}
	if err := render.AddText(img, formatChange(change, parseFloat(q.PercentChange)), image.Point{changeX + 48, 118}, black, 30); err != nil {
		return err
	}

	// One compact stats line.
	stats := fmt.Sprintf("Open %s   Prev %s   Day %s-%s   Vol %s",
		formatMoney(parseFloat(q.Open)), formatMoney(parseFloat(q.PreviousClose)),
		formatMoney(parseFloat(q.Low)), formatMoney(parseFloat(q.High)), formatVolume(parseFloat(q.Volume)))
	if err := render.AddText(img, stats, image.Point{20, 168}, black, 22); err != nil {
		return err
	}

	// Chart across the full width.
	if err := render.AddCloseChart(img, history, 760, 225, image.Point{20, 185}); err != nil {
		return err
	}

	// Footer: 52-week range bar on the left, quote time and status on the right.
	lo52, hi52 := parseFloat(q.FiftyTwoWeek.Low), parseFloat(q.FiftyTwoWeek.High)
	if hi52 > lo52 {
		if err := render.AddText(img, "52w", image.Point{20, 462}, black, 22); err != nil {
			return err
		}
		loText := formatMoney(lo52)
		if err := render.AddText(img, loText, image.Point{70, 462}, black, 22); err != nil {
			return err
		}
		loW, err := render.TextWidth(loText, 22)
		if err != nil {
			return err
		}
		barX := 70 + loW + 12
		if err := render.AddRangeBar(img, lo52, hi52, price, image.Rect(barX, 444, barX+260, 464)); err != nil {
			return err
		}
		if err := render.AddText(img, formatMoney(hi52), image.Point{barX + 260 + 12, 462}, black, 22); err != nil {
			return err
		}
	}
	status := quoteStatusLine(q.LastQuoteAt, meta.ExchangeTimezone, q.IsMarketOpen)
	statusW, err := render.TextWidth(status, 22)
	if err != nil {
		return err
	}
	if err := render.AddText(img, status, image.Point{780 - statusW, 462}, black, 22); err != nil {
		return err
	}

	return render.WriteFile(outputPath, img, voltage)
}
