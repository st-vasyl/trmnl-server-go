package crypto

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"io"
	"net/http"
	"trmnl-server-go/pkg/v1/render"

	"github.com/rs/zerolog/log"
)

type HistoryRecords struct {
	Prices [][]float64 `json:"prices"`
}

type Crypto struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	MarketData MarketData `json:"market_data"`
}

type MarketData struct {
	CurrentPrice        Price   `json:"current_price"`
	High24h             Price   `json:"high_24h"`
	Low24h              Price   `json:"low_24h"`
	Diff_percentage_7d  float64 `json:"price_change_percentage_7d"`
	Diff_percentage_30d float64 `json:"price_change_percentage_30d"`
	Diff_percentage_1y  float64 `json:"price_change_percentage_1y"`
}

type Price struct {
	USD int `json:"usd"`
}

func GetCryptoData(symbol string) (Crypto, error) {
	var c Crypto

	url := fmt.Sprintf("https://api.coingecko.com/api/v3/coins/%s?localization=false&tickers=false&market_data=true&community_data=false&developer_data=false&sparkline=false", symbol)
	r, err := http.Get(url)
	r.Header.Set("Accept", "application/json")
	r.Header.Set("Accept-Language", "en-US")
	if err != nil {
		log.Error().
			Str("plugin", "crypto").
			Str("func", "GetCryptoData").
			Err(err).
			Msg("Unable to get json data from API")
		return c, err
	}
	body, err := io.ReadAll(r.Body)
	r.Body.Close()
	if err != nil {
		return c, err
	}

	err = json.Unmarshal([]byte(body), &c)
	if err != nil {
		log.Error().
			Str("plugin", "crypto").
			Str("func", "GetCryptoData").
			Err(err).
			Msg("Unable unmarshal json data")
		return c, err
	}

	return c, nil
}

func GetCryptoHistoryData(symbol string) (render.ChartRecords, error) {
	var hr HistoryRecords
	var records render.ChartRecords

	url := fmt.Sprintf("https://api.coingecko.com/api/v3/coins/%s/market_chart?vs_currency=usd&days=1", symbol)
	r, err := http.Get(url)
	r.Header.Set("Accept", "application/json")
	r.Header.Set("Accept-Language", "en-US")
	if err != nil {
		log.Error().
			Str("plugin", "crypto").
			Str("func", "GetCryptoHistoryData").
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

	for _, v := range hr.Prices {
		var r render.ChartRecord
		r.T = v[0]
		r.V = v[1]
		records.ChartRecord = append(records.ChartRecord, r)
	}
	return records, nil
}

func RenderScreenCrypto(width, height int, coin, filename string, voltage float32) error {
	b, _ := GetCryptoData(coin)
	r, _ := GetCryptoHistoryData(coin)
	img := render.NewImage(width, height)

	if err := render.AddText(img, fmt.Sprintf("%s: $%d", b.Name, b.MarketData.CurrentPrice.USD), image.Point{50, 50}, color.Black, 50); err != nil {
		return err
	}

	if err := render.AddText(img, fmt.Sprintf("High 24h: $%d", b.MarketData.High24h.USD), image.Point{50, 100}, color.Black, 30); err != nil {
		return err
	}

	if err := render.AddText(img, fmt.Sprintf("Low 24h:  $%d", b.MarketData.Low24h.USD), image.Point{50, 150}, color.Black, 30); err != nil {
		return err
	}

	if err := render.AddText(img, fmt.Sprintf("Diff 1m: %.1f %s", b.MarketData.Diff_percentage_30d, "%"), image.Point{400, 100}, color.Black, 30); err != nil {
		return err
	}

	if err := render.AddText(img, fmt.Sprintf("Diff 1y:  %.1f %s", b.MarketData.Diff_percentage_1y, "%"), image.Point{400, 150}, color.Black, 30); err != nil {
		return err
	}

	if err := render.AddChart(img, r, 550, 200, image.Point{-30, -200}); err != nil {
		return err
	}

	if err := render.WriteFile(filename, img, voltage); err != nil {
		return err
	}

	return nil
}
