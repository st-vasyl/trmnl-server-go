package bitcoin

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"log"
	"net/http"
	"os"
	"trmnl-server-go/pkg/v1/render"
)

type Bitcoin struct {
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

func GetBitcoinData() (Bitcoin, error) {
	var b Bitcoin

	url := fmt.Sprintf("https://api.coingecko.com/api/v3/coins/bitcoin?localization=false&tickers=false&market_data=true&community_data=false&developer_data=false&sparkline=false")
	r, err := http.Get(url)
	r.Header.Set("Accept", "application/json")
	r.Header.Set("Accept-Language", "en-US")
	if err != nil {
		return b, err
	}
	body, err := io.ReadAll(r.Body)
	r.Body.Close()
	if err != nil {
		return b, err
	}

	err = json.Unmarshal([]byte(body), &b)
	if err != nil {
		panic(err)
	}

	return b, nil
}

// Generatebreen creates a TRMNL breen
func RenderBitconin(width, height, positionX, positionY int, filename string) error {
	// Create a white background
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	white := color.RGBA{255, 255, 255, 255}
	draw.Draw(img, img.Bounds(), &image.Uniform{white}, image.Point{}, draw.Src)

	b, _ := GetBitcoinData()

	price := fmt.Sprintf("Current price : $%d ", b.MarketData.CurrentPrice.USD)

	// Draw TRMNL logo text in center
	render.DrawText(img, positionX, positionY, b.Name, color.Black)
	render.DrawText(img, positionX, positionY+15, price, color.Black)

	f, err := os.Create(filename)
	if err != nil {
		log.Fatal(err)
	}

	if err := png.Encode(f, img); err != nil {
		f.Close()
		log.Fatal(err)
	}

	if err := f.Close(); err != nil {
		log.Fatal(err)
	}

	return nil
}
