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
	"trmnl-server-go/pkg/v1/render"

	"github.com/rs/zerolog/log"
)

// baseURL is the TwelveData API root. Overridden in tests.
var baseURL = "https://api.twelvedata.com"

// StocksPlugin renders a 7-day OHLC candlestick chart for each configured symbol.
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

type stockLatestPrice struct {
	Price string `json:"price"`
}

type stock struct {
	Symbol        string       `json:"symbol"`
	Name          string       `json:"name"`
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
	Meta   historyMeta     `json:"meta"`
	Status string          `json:"status"`
	Values []historyRecord `json:"values"`
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

func getLatestPrice(symbol, apiKey string) (stockLatestPrice, error) {
	var s stockLatestPrice
	url := fmt.Sprintf("%s/price?symbol=%s&apikey=%s", baseURL, symbol, apiKey)
	body, err := httpclient.Get(url)
	if err != nil {
		log.Error().Str("plugin", "stocks").Str("symbol", symbol).Err(err).Msg("Failed to fetch latest price")
		return s, err
	}
	if err := json.Unmarshal(body, &s); err != nil {
		log.Error().Str("plugin", "stocks").Str("symbol", symbol).Err(err).Msg("Failed to parse latest price")
		return s, err
	}
	return s, nil
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
	return s, nil
}

func getHistory(symbol, apiKey string) (render.BoxPlotRecords, error) {
	var hr historyRecords
	records := render.BoxPlotRecords{XLabels: make(map[float64]string)}

	now := time.Now()
	weekAgo := now.AddDate(0, 0, -7)
	startDate := fmt.Sprintf("%d-%02d-%02d", weekAgo.Year(), weekAgo.Month(), weekAgo.Day())
	endDate := fmt.Sprintf("%d-%02d-%02dT%02d:%02d:%02d", now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute(), now.Second())
	url := fmt.Sprintf("%s/time_series?symbol=%s&interval=30min&start_date=%s&end_date=%s&apikey=%s", baseURL, symbol, startDate, endDate, apiKey)

	body, err := httpclient.Get(url)
	if err != nil {
		log.Error().Str("plugin", "stocks").Str("symbol", symbol).Err(err).Msg("Failed to fetch history")
		return records, err
	}
	if err := json.Unmarshal(body, &hr); err != nil {
		log.Error().Str("plugin", "stocks").Str("symbol", symbol).Err(err).Msg("Failed to parse history")
		return records, err
	}

	n := len(hr.Values)
	var prevDay int
	for j := 0; j < n; j++ {
		v := hr.Values[n-1-j]
		t, _ := time.Parse("2006-01-02 15:04:05", v.Datetime)
		vmin, _ := strconv.ParseFloat(strings.TrimSpace(v.Low), 64)
		vmax, _ := strconv.ParseFloat(strings.TrimSpace(v.High), 64)
		records.BoxPlotRecord = append(records.BoxPlotRecord, render.BoxPlotRecord{
			T:    float64(j),
			Vmin: vmin,
			Vmax: vmax,
		})
		if j == 0 || t.Day() != prevDay {
			records.XLabels[float64(j)] = t.Format("01/02 15:04")
		}
		prevDay = t.Day()
	}
	return records, nil
}

func renderScreen(symbol, apiKey, outputPath string, voltage float32) error {
	latestPrice, err := getLatestPrice(symbol, apiKey)
	if err != nil {
		return err
	}
	quote, err := getQuote(symbol, apiKey)
	if err != nil {
		return err
	}
	history, err := getHistory(symbol, apiKey)
	if err != nil {
		return err
	}

	img := render.NewImage(800, 480)

	current, _ := strconv.ParseFloat(strings.TrimSpace(latestPrice.Price), 64)
	if err := render.AddText(img, fmt.Sprintf("%s: $%.1f", quote.Name, current), image.Point{50, 50}, color.Black, 50); err != nil {
		return err
	}

	high24, _ := strconv.ParseFloat(strings.TrimSpace(quote.High), 64)
	if err := render.AddText(img, fmt.Sprintf("High 24h: $%.1f", high24), image.Point{50, 100}, color.Black, 30); err != nil {
		return err
	}

	low24, _ := strconv.ParseFloat(strings.TrimSpace(quote.Low), 64)
	if err := render.AddText(img, fmt.Sprintf("Low  24h: $%.1f", low24), image.Point{50, 150}, color.Black, 30); err != nil {
		return err
	}

	high52w, _ := strconv.ParseFloat(strings.TrimSpace(quote.FiftyTwoWeek.High), 64)
	if err := render.AddText(img, fmt.Sprintf("High 52w: $%.1f", high52w), image.Point{400, 100}, color.Black, 30); err != nil {
		return err
	}

	low52w, _ := strconv.ParseFloat(strings.TrimSpace(quote.FiftyTwoWeek.Low), 64)
	if err := render.AddText(img, fmt.Sprintf("Low  52w: $%.1f", low52w), image.Point{400, 150}, color.Black, 30); err != nil {
		return err
	}

	if err := render.AddStocksChart(img, history, 550, 200, image.Point{-30, -200}); err != nil {
		return err
	}

	return render.WriteFile(outputPath, img, voltage)
}
