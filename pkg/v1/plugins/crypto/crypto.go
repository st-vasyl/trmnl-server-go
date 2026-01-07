package crypto

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"io"
	"net/http"
	"trmnl-server-go/pkg/v1/render"
)

type Crypto struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	MarketData MarketData `json:"market_data"`
}

type MarketData struct {
	CurrentPrice CurrentPrice `json:"current_price"`
}

type CurrentPrice struct {
	USD int `json:"usd"`
}

func GetCryptoData(symbol string) (Crypto, error) {
	var c Crypto

	url := fmt.Sprintf("https://api.coingecko.com/api/v3/coins/%s?localization=false&tickers=false&market_data=true&community_data=false&developer_data=false&sparkline=false", symbol)
	r, err := http.Get(url)
	r.Header.Set("Accept", "application/json")
	r.Header.Set("Accept-Language", "en-US")
	if err != nil {
		return c, err
	}
	body, err := io.ReadAll(r.Body)
	r.Body.Close()
	if err != nil {
		return c, err
	}

	err = json.Unmarshal([]byte(body), &c)
	if err != nil {
		panic(err)
	}

	return c, nil
}

func GetCryptoHistoryData(symbol string) (render.Records, error) {
	var rec render.Records

	url := fmt.Sprintf("https://api.coingecko.com/api/v3/coins/%s/market_chart?vs_currency=usd&days=1", symbol)
	r, err := http.Get(url)
	r.Header.Set("Accept", "application/json")
	r.Header.Set("Accept-Language", "en-US")
	if err != nil {
		return rec, err
	}
	body, err := io.ReadAll(r.Body)
	r.Body.Close()
	if err != nil {
		return rec, err
	}

	err = json.Unmarshal([]byte(body), &rec)
	if err != nil {
		panic(err)
	}

	return rec, nil
}

func RenderScreenCrypto(width, height int, filename string) error {
	b, _ := GetCryptoData("bitcoin")
	r, _ := GetCryptoHistoryData("bitcoin")
	img := render.NewImage(width, height)

	if err := render.AddText(img, fmt.Sprintf("$%d", b.MarketData.CurrentPrice.USD), image.Point{50, 50}, color.Black, 50); err != nil {
		return err
	}

	if err := render.AddChart(img, r, width, height, 50, 200, 500, 400); err != nil {
		return err
	}

	if err := render.WriteFile(filename, img); err != nil {
		return err
	}

	return nil
}
