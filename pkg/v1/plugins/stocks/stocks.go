package stocks

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
	"trmnl-server-go/pkg/v1/render"

	"github.com/rs/zerolog/log"
)

type StockLatestPrice struct {
	Price string `json:"price"`
}

type Stock struct {
	Symbol            string       `json:"symbol"`
	Name              string       `json:"name"`
	Open              string       `json:"open"`
	High              string       `json:"high"`
	Low               string       `json:"low"`
	Close             string       `json:"close"`
	Volume            string       `json:"volume"`
	PreviousClose     string       `json:"previous_close"`
	Change            string       `json:"change"`
	PercentChange     string       `json:"percent_change"`
	AverageVolume     string       `json:"average_volume"`
	RollingChange1d   string       `json:"rolling_1d_change"`
	RollingChange7d   string       `json:"rolling_7d_change"`
	RollingChange     string       `json:"rolling_change"`
	ExtendedPrice     string       `json:"extended_price"`
	ExtendedTimestamp string       `json:"extended_timestamp"`
	FiftyTwoWeek      FiftyTwoWeek `json:"fifty_two_week"`
}

type FiftyTwoWeek struct {
	Low  string `json:"low"`
	High string `json:"high"`
}

type HistoryRecords struct {
	Meta   HistoryMeta     `json:"meta"`
	Status string          `json:"status"`
	Values []HistoryRecord `json:"values"`
}

type HistoryRecord struct {
	Datetime string `json:"datetime"`
	Open     string `json:"open"`
	High     string `json:"high"`
	Low      string `json:"low"`
	Close    string `json:"close"`
	Volume   string `json:"volume"`
}

type HistoryMeta struct {
	Symbol           string `json:"symbol"`
	Interval         string `json:"interval"`
	Currency         string `json:"currency"`
	ExchangeTimezone string `json:"exchange_timezone"`
	Exchange         string `json:"exchange"`
	Mic_code         string `json:"mic_code"`
	Type             string `json:"type"`
}

func GetStockLatestPrice(symbol, apiKey string) (StockLatestPrice, error) {
	var s StockLatestPrice

	url := fmt.Sprintf("https://api.twelvedata.com/price?symbol=%s&apikey=%s", symbol, apiKey)
	r, err := http.Get(url)
	r.Header.Set("Accept", "application/json")
	r.Header.Set("Accept-Language", "en-US")
	if err != nil {
		log.Error().
			Str("plugin", "stocks").
			Str("func", "GetStockLatestPrice").
			Err(err).
			Msg("Unable to get json data from API")
		return s, err
	}
	body, err := io.ReadAll(r.Body)
	r.Body.Close()
	if err != nil {
		return s, err
	}

	err = json.Unmarshal([]byte(body), &s)
	if err != nil {
		log.Error().
			Str("plugin", "stocks").
			Str("func", "GetStockLatestPrice").
			Err(err).
			Msg("Unable unmarshal json data")
		return s, err
	}

	return s, nil
}

func GetStockData(symbol, apiKey string) (Stock, error) {
	var s Stock

	url := fmt.Sprintf("https://api.twelvedata.com/quote?symbol=%s&apikey=%s", symbol, apiKey)
	r, err := http.Get(url)
	r.Header.Set("Accept", "application/json")
	r.Header.Set("Accept-Language", "en-US")
	if err != nil {
		log.Error().
			Str("plugin", "stocks").
			Str("func", "GetStockData").
			Err(err).
			Msg("Unable to get json data from API")
		return s, err
	}
	body, err := io.ReadAll(r.Body)
	r.Body.Close()
	if err != nil {
		return s, err
	}

	err = json.Unmarshal([]byte(body), &s)
	if err != nil {
		log.Error().
			Str("plugin", "stocks").
			Str("func", "GetStockData").
			Err(err).
			Msg("Unable unmarshal json data")
		return s, err
	}

	return s, nil
}

func GetStocksHistoryData(symbol, apiKey string) (render.BoxPlotRecords, error) {
	var hr HistoryRecords
	var records render.BoxPlotRecords

	now := time.Now()
	weekAgo := now.AddDate(0, 0, -1)
	startDate := fmt.Sprintf("%d-%02d-%02d", weekAgo.Year(), weekAgo.Month(), weekAgo.Day())
	endDate := fmt.Sprintf("%d-%02d-%02dT%02d:%02d:%02d", now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute(), now.Second())
	url := fmt.Sprintf("https://api.twelvedata.com/time_series?symbol=%s&interval=15min&start_date=%s&end_date=%s&apikey=%s", symbol, startDate, endDate, apiKey)
	r, err := http.Get(url)
	r.Header.Set("Accept", "application/json")
	r.Header.Set("Accept-Language", "en-US")
	if err != nil {
		log.Error().
			Str("plugin", "stocks").
			Str("func", "GetStocksHistoryData").
			Err(err).
			Msg("Unable to get json data from API")
		return records, err
	}
	body, err := io.ReadAll(r.Body)
	r.Body.Close()
	if err != nil {
		return records, err
	}

	err = json.Unmarshal([]byte(body), &hr)
	if err != nil {
		log.Error().
			Str("plugin", "crypto").
			Str("func", "GetCryptoHistoryData").
			Err(err).
			Msg("Unable unmarshal json data")
		return records, err
	}

	for _, v := range hr.Values {
		var r render.BoxPlotRecord
		t, _ := time.Parse("2006-01-02 15:04:05", v.Datetime)
		r.T = float64(t.UnixMilli())
		r.Vmin, _ = strconv.ParseFloat(strings.TrimSpace(v.Low), 64)
		r.Vmax, _ = strconv.ParseFloat(strings.TrimSpace(v.High), 64)
		records.BoxPlotRecord = append(records.BoxPlotRecord, r)
		// log.Printf("Trace: parsed history records. Parsed Time: %s, and Source Time %s", t.Format("2006-01-02T15:04:05 -07:00:00"), v.Datetime)
	}
	return records, nil
}

func RenderScreenStocks(width, height int, symbol, apiKey, filename string, voltage float32) error {
	l, _ := GetStockLatestPrice(symbol, apiKey)
	s, _ := GetStockData(symbol, apiKey)
	records, _ := GetStocksHistoryData(symbol, apiKey)
	img := render.NewImage(width, height)

	current, _ := strconv.ParseFloat(strings.TrimSpace(l.Price), 64)
	if err := render.AddText(img, fmt.Sprintf("%s: $%.1f", s.Name, current), image.Point{50, 50}, color.Black, 50); err != nil {
		return err
	}

	high24, _ := strconv.ParseFloat(strings.TrimSpace(s.High), 64)
	if err := render.AddText(img, fmt.Sprintf("High 24h: $%.1f", high24), image.Point{50, 100}, color.Black, 30); err != nil {
		return err
	}

	low24, _ := strconv.ParseFloat(strings.TrimSpace(s.Low), 64)
	if err := render.AddText(img, fmt.Sprintf("Low 24h:  $%.1f", low24), image.Point{50, 150}, color.Black, 30); err != nil {
		return err
	}

	high52w, _ := strconv.ParseFloat(strings.TrimSpace(s.FiftyTwoWeek.High), 64)
	if err := render.AddText(img, fmt.Sprintf("High 52w: $%.1f", high52w), image.Point{400, 100}, color.Black, 30); err != nil {
		return err
	}

	low52w, _ := strconv.ParseFloat(strings.TrimSpace(s.FiftyTwoWeek.Low), 64)
	if err := render.AddText(img, fmt.Sprintf("Low 52w:  $%.1f", low52w), image.Point{400, 150}, color.Black, 30); err != nil {
		return err
	}

	if err := render.AddStocksChart(img, records, 550, 200, image.Point{-30, -200}); err != nil {
		return err
	}

	if err := render.WriteFile(filename, img, voltage); err != nil {
		return err
	}

	return nil
}
