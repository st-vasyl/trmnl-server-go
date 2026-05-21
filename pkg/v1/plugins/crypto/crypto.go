package crypto

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"strings"
	"trmnl-server-go/pkg/v1/httpclient"
	"trmnl-server-go/pkg/v1/render"

	"github.com/rs/zerolog/log"
)

// CryptoPlugin renders 24h price charts for each configured coin.
type CryptoPlugin struct {
	Symbols []string
}

func (p *CryptoPlugin) Name() string { return "coingecko" }

func (p *CryptoPlugin) Screens() []string {
	screens := make([]string, len(p.Symbols))
	for i, s := range p.Symbols {
		screens[i] = fmt.Sprintf("coingecko_%s", s)
	}
	return screens
}

func (p *CryptoPlugin) Render(screen, outputPath string, voltage float32) error {
	coin := strings.TrimPrefix(screen, "coingecko_")
	return renderScreen(coin, outputPath, voltage)
}

type historyRecords struct {
	Prices [][]float64 `json:"prices"`
}

type crypto struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	MarketData marketData `json:"market_data"`
}

type marketData struct {
	CurrentPrice       price   `json:"current_price"`
	High24h            price   `json:"high_24h"`
	Low24h             price   `json:"low_24h"`
	DiffPercentage7d   float64 `json:"price_change_percentage_7d"`
	DiffPercentage30d  float64 `json:"price_change_percentage_30d"`
	DiffPercentage1y   float64 `json:"price_change_percentage_1y"`
}

type price struct {
	USD int `json:"usd"`
}

func getCryptoData(symbol string) (crypto, error) {
	var c crypto
	url := fmt.Sprintf("https://api.coingecko.com/api/v3/coins/%s?localization=false&tickers=false&market_data=true&community_data=false&developer_data=false&sparkline=false", symbol)
	body, err := httpclient.Get(url)
	if err != nil {
		log.Error().Str("plugin", "crypto").Str("symbol", symbol).Err(err).Msg("Failed to fetch crypto data")
		return c, err
	}
	if err := json.Unmarshal(body, &c); err != nil {
		log.Error().Str("plugin", "crypto").Str("symbol", symbol).Err(err).Msg("Failed to parse crypto data")
		return c, err
	}
	return c, nil
}

func getCryptoHistory(symbol string) (render.ChartRecords, error) {
	var hr historyRecords
	var records render.ChartRecords

	url := fmt.Sprintf("https://api.coingecko.com/api/v3/coins/%s/market_chart?vs_currency=usd&days=1", symbol)
	body, err := httpclient.Get(url)
	if err != nil {
		log.Error().Str("plugin", "crypto").Str("symbol", symbol).Err(err).Msg("Failed to fetch crypto history")
		return records, err
	}
	if err := json.Unmarshal(body, &hr); err != nil {
		log.Error().Str("plugin", "crypto").Str("symbol", symbol).Err(err).Msg("Failed to parse crypto history")
		return records, err
	}

	for _, v := range hr.Prices {
		records.ChartRecord = append(records.ChartRecord, render.ChartRecord{T: v[0], V: v[1]})
	}
	return records, nil
}

func renderScreen(coin, outputPath string, voltage float32) error {
	data, err := getCryptoData(coin)
	if err != nil {
		return err
	}
	history, err := getCryptoHistory(coin)
	if err != nil {
		return err
	}

	img := render.NewImage(800, 480)

	if err := render.AddText(img, fmt.Sprintf("%s: $%d", data.Name, data.MarketData.CurrentPrice.USD), image.Point{50, 50}, color.Black, 50); err != nil {
		return err
	}
	if err := render.AddText(img, fmt.Sprintf("High 24h: $%d", data.MarketData.High24h.USD), image.Point{50, 100}, color.Black, 30); err != nil {
		return err
	}
	if err := render.AddText(img, fmt.Sprintf("Low  24h: $%d", data.MarketData.Low24h.USD), image.Point{50, 150}, color.Black, 30); err != nil {
		return err
	}
	if err := render.AddText(img, fmt.Sprintf("Diff 1m: %.1f%%", data.MarketData.DiffPercentage30d), image.Point{400, 100}, color.Black, 30); err != nil {
		return err
	}
	if err := render.AddText(img, fmt.Sprintf("Diff 1y: %.1f%%", data.MarketData.DiffPercentage1y), image.Point{400, 150}, color.Black, 30); err != nil {
		return err
	}
	if err := render.AddChart(img, history, 550, 200, image.Point{-30, -200}); err != nil {
		return err
	}

	return render.WriteFile(outputPath, img, voltage)
}
