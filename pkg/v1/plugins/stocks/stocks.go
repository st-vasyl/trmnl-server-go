package stocks

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
)

// Layout constants for screen rendering
const (
	TitleOffsetY       = 40 // Offset from center for title text
	MessageStartY      = 10 // Starting Y offset below center for messages
	MessageLineSpacing = 20 // Vertical spacing between message lines
	BottomMarginY      = 30 // Distance from bottom edge
	MinTextMarginX     = 10 // Minimum horizontal margin for text
	ErrorTitleOffsetY  = 60 // Offset from center for error titles
	ErrorMessageStartY = 20 // Starting Y offset below center for error messages
	MaxLineWrapChars   = 60 // Maximum characters per line for text wrapping
)

type StockCompany struct {
	Symbol                     string `json:"Symbol"`
	AssetType                  string `json:"AssetType"`
	Name                       string `json:"Name"`
	Description                string `json:"Description"`
	CIK                        string `json:"CIK"`
	Exchange                   string `json:"Exchange"`
	Currency                   string `json:"Currency"`
	Country                    string `json:"Country"`
	Sector                     string `json:"Sector"`
	Industry                   string `json:"Industry"`
	Address                    string `json:"Address"`
	OfficialSite               string `json:"OfficialSite"`
	FiscalYearEnd              string `json:"FiscalYearEnd"`
	LatestQuarter              string `json:"LatestQuarter"`
	MarketCapitalization       string `json:"MarketCapitalization"`
	EBITDA                     string `json:"EBITDA"`
	PERatio                    string `json:"PERatio"`
	PEGRatio                   string `json:"PEGRatio"`
	BookValue                  string `json:"BookValue"`
	DividendPerShare           string `json:"DividendPerShare"`
	DividendYield              string `json:"DividendYield"`
	EPS                        string `json:"EPS"`
	RevenuePerShareTTM         string `json:"RevenuePerShareTTM"`
	ProfitMargin               string `json:"ProfitMargin"`
	OperatingMarginTTM         string `json:"OperatingMarginTTM"`
	ReturnOnAssetsTTM          string `json:"ReturnOnAssetsTTM"`
	ReturnOnEquityTTM          string `json:"ReturnOnEquityTTM"`
	RevenueTTM                 string `json:"RevenueTTM"`
	GrossProfitTTM             string `json:"GrossProfitTTM"`
	DilutedEPSTTM              string `json:"DilutedEPSTTM"`
	QuarterlyEarningsGrowthYOY string `json:"QuarterlyEarningsGrowthYOY"`
	QuarterlyRevenueGrowthYOY  string `json:"QuarterlyRevenueGrowthYOY"`
	AnalystTargetPrice         string `json:"AnalystTargetPrice"`
	AnalystRatingStrongBuy     string `json:"AnalystRatingStrongBuy"`
	AnalystRatingBuy           string `json:"AnalystRatingBuy"`
	AnalystRatingHold          string `json:"AnalystRatingHold"`
	AnalystRatingSell          string `json:"AnalystRatingSell"`
	AnalystRatingStrongSell    string `json:"AnalystRatingStrongSell"`
	TrailingPE                 string `json:"TrailingPE"`
	ForwardPE                  string `json:"ForwardPE"`
	PriceToSalesRatioTTM       string `json:"PriceToSalesRatioTTM"`
	PriceToBookRatio           string `json:"PriceToBookRatio"`
	EVToRevenue                string `json:"EVToRevenue"`
	EVToEBITDA                 string `json:"EVToEBITDA"`
	Beta                       string `json:"Beta"`
	D52WeekHigh                string `json:"52WeekHigh"`
	D52WeekLow                 string `json:"52WeekLow"`
	D50DayMovingAverage        string `json:"50DayMovingAverage"`
	D200DayMovingAverage       string `json:"200DayMovingAverage"`
	SharesOutstanding          string `json:"SharesOutstanding"`
	SharesFloat                string `json:"SharesFloat"`
	PercentInsiders            string `json:"PercentInsiders"`
	PercentInstitutions        string `json:"PercentInstitutions"`
	DividendDate               string `json:"DividendDate"`
	ExDividendDate             string `json:"ExDividendDate"`
}

func GetStocksData(company string, apiKey string) (StockCompany, error) {
	var sc StockCompany

	url := fmt.Sprintf("https://www.alphavantage.co/query?function=OVERVIEW&symbol=%s&apikey=%s", company, apiKey)
	r, err := http.Get(url)
	r.Header.Set("Accept", "application/json")
	r.Header.Set("Accept-Language", "en-US")
	if err != nil {
		return sc, err
	}
	body, err := io.ReadAll(r.Body)
	r.Body.Close()
	if err != nil {
		return sc, err
	}

	err = json.Unmarshal([]byte(body), &sc)
	if err != nil {
		panic(err)
	}

	return sc, nil
}

// GenerateScreen creates a TRMNL screen
func RenderStocks(company, apiKey string, width, height, positionX, positionY int, filename string) error {
	// Create a white background
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	white := color.RGBA{255, 255, 255, 255}
	draw.Draw(img, img.Bounds(), &image.Uniform{white}, image.Point{}, draw.Src)

	// sc, _ := GetStocksData(company, apiKey)

	// price := fmt.Sprintf("Current price : %s ", sc.AnalystTargetPrice)
	// yearHigh := fmt.Sprintf("Year high : %s ", sc.D52WeekHigh)
	// yearLow := fmt.Sprintf("Year Low : %s ", sc.D52WeekLow)

	// // Draw TRMNL logo text in center
	// render.DrawText(img, positionX, positionY, sc.Name, color.Black)
	// render.DrawText(img, positionX, positionY+15, price, color.Black)
	// render.DrawText(img, positionX, positionY+30, yearHigh, color.Black)
	// render.DrawText(img, positionX, positionY+45, yearLow, color.Black)

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
