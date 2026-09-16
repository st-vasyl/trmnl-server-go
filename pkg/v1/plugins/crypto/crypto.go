package crypto

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"
	"time"
	"trmnl-server-go/pkg/v1/httpclient"
	"trmnl-server-go/pkg/v1/icons"
	"trmnl-server-go/pkg/v1/render"

	"github.com/rs/zerolog/log"
)

// baseURL is the CoinGecko API root. Overridden in tests.
var baseURL = "https://api.coingecko.com"

// chartDays is the market-chart window. CoinGecko returns hourly points for it.
const chartDays = 7

// CryptoPlugin renders a quote summary and a 7-day price chart for each
// configured coin.
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

// crypto is the CoinGecko /coins/{id} response. Error and Status are only set
// on error bodies ("coin not found", rate limiting).
type crypto struct {
	ID            string     `json:"id"`
	Symbol        string     `json:"symbol"`
	Name          string     `json:"name"`
	MarketCapRank int        `json:"market_cap_rank"`
	MarketData    marketData `json:"market_data"`
	Error         string     `json:"error"`
	Status        apiStatus  `json:"status"`
}

type apiStatus struct {
	ErrorCode    int    `json:"error_code"`
	ErrorMessage string `json:"error_message"`
}

func (s apiStatus) err() error {
	if s.ErrorCode != 0 || s.ErrorMessage != "" {
		return fmt.Errorf("coingecko: %s", s.ErrorMessage)
	}
	return nil
}

type marketData struct {
	CurrentPrice price   `json:"current_price"`
	High24h      price   `json:"high_24h"`
	Low24h       price   `json:"low_24h"`
	MarketCap    price   `json:"market_cap"`
	TotalVolume  price   `json:"total_volume"`
	AthChangePct price   `json:"ath_change_percentage"`
	Change24h    float64 `json:"price_change_24h"`
	ChangePct24h float64 `json:"price_change_percentage_24h"`
	ChangePct7d  float64 `json:"price_change_percentage_7d"`
	ChangePct30d float64 `json:"price_change_percentage_30d"`
	ChangePct1y  float64 `json:"price_change_percentage_1y"`
	LastUpdated  string  `json:"last_updated"`
}

type price struct {
	USD float64 `json:"usd"`
}

type historyRecords struct {
	Prices [][]float64 `json:"prices"` // [unix ms, price]
	Status apiStatus   `json:"status"`
}

func getCryptoData(symbol string) (crypto, error) {
	var c crypto
	url := fmt.Sprintf("%s/api/v3/coins/%s?localization=false&tickers=false&market_data=true&community_data=false&developer_data=false&sparkline=false", baseURL, symbol)
	body, err := httpclient.Get(url)
	if err != nil {
		log.Error().Str("plugin", "crypto").Str("symbol", symbol).Err(err).Msg("Failed to fetch crypto data")
		return c, err
	}
	if err := json.Unmarshal(body, &c); err != nil {
		log.Error().Str("plugin", "crypto").Str("symbol", symbol).Err(err).Msg("Failed to parse crypto data")
		return c, err
	}
	if c.Error != "" {
		return c, fmt.Errorf("coingecko: %s", c.Error)
	}
	if err := c.Status.err(); err != nil {
		return c, err
	}
	return c, nil
}

// getCryptoHistory fetches chartDays of hourly prices and returns them in
// order with a separator at the first point of each calendar day in loc.
func getCryptoHistory(symbol string, loc *time.Location) (render.CloseSeries, error) {
	var hr historyRecords
	series := render.CloseSeries{Separators: map[int]string{}}

	url := fmt.Sprintf("%s/api/v3/coins/%s/market_chart?vs_currency=usd&days=%d", baseURL, symbol, chartDays)
	body, err := httpclient.Get(url)
	if err != nil {
		log.Error().Str("plugin", "crypto").Str("symbol", symbol).Err(err).Msg("Failed to fetch crypto history")
		return series, err
	}
	if err := json.Unmarshal(body, &hr); err != nil {
		log.Error().Str("plugin", "crypto").Str("symbol", symbol).Err(err).Msg("Failed to parse crypto history")
		return series, err
	}
	if err := hr.Status.err(); err != nil {
		return series, err
	}
	if len(hr.Prices) == 0 {
		return series, fmt.Errorf("coingecko: no prices returned for %s", symbol)
	}

	// Only day changes are marked. The series starts mid-day, so labelling the
	// first point would put a label a few hours before the first midnight one.
	prevDay := ""
	for i, p := range hr.Prices {
		if len(p) < 2 {
			return series, fmt.Errorf("coingecko: malformed price point %v", p)
		}
		series.Closes = append(series.Closes, p[1])
		day := time.UnixMilli(int64(p[0])).In(loc).Format("2006-01-02")
		if i > 0 && day != prevDay {
			series.Separators[i] = time.UnixMilli(int64(p[0])).In(loc).Format("01/02")
		}
		prevDay = day
	}
	return series, nil
}

// formatPrice adapts precision to magnitude: thousands separators and no
// decimals from 1,000 up, two decimals from 1 to 999, and four significant
// digits below 1 so small coins never print as zero.
func formatPrice(v float64) string {
	v = math.Abs(v)
	switch {
	case v >= 1000:
		return groupThousands(fmt.Sprintf("%.0f", v))
	case v >= 1:
		return fmt.Sprintf("%.2f", v)
	case v > 0:
		decimals := 3 - int(math.Floor(math.Log10(v)))
		return fmt.Sprintf("%.*f", decimals, v)
	default:
		return "0.00"
	}
}

func groupThousands(digits string) string {
	if len(digits) <= 3 {
		return digits
	}
	var b strings.Builder
	pre := len(digits) % 3
	if pre > 0 {
		b.WriteString(digits[:pre])
	}
	for i := pre; i < len(digits); i += 3 {
		if b.Len() > 0 {
			b.WriteByte(',')
		}
		b.WriteString(digits[i : i+3])
	}
	return b.String()
}

// formatSignedPrice prefixes formatPrice with the sign; moves too small to
// show print as 0.00.
func formatSignedPrice(v float64) string {
	switch {
	case v >= 0.005:
		return "+" + formatPrice(v)
	case v <= -0.005:
		return "-" + formatPrice(v)
	default:
		return "0.00"
	}
}

// formatPct prints a signed percent with two decimals; tiny moves print as 0.00%.
func formatPct(v float64) string {
	switch {
	case v >= 0.005:
		return fmt.Sprintf("+%.2f%%", v)
	case v <= -0.005:
		return fmt.Sprintf("%.2f%%", v)
	default:
		return "0.00%"
	}
}

// formatBig shortens large amounts to K, M, B or T.
func formatBig(v float64) string {
	switch {
	case v >= 1e12:
		return fmt.Sprintf("%.2fT", v/1e12)
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

// updatedLine formats CoinGecko's RFC 3339 last_updated in loc, or returns ""
// when it cannot be parsed so the caller can skip it.
func updatedLine(lastUpdated string, loc *time.Location) string {
	t, err := time.Parse(time.RFC3339Nano, lastUpdated)
	if err != nil {
		return ""
	}
	return "Updated " + t.In(loc).Format("Jan 2 15:04 MST")
}

func renderScreen(id, outputPath string, voltage float32) error {
	c, err := getCryptoData(id)
	if err != nil {
		return err
	}
	history, err := getCryptoHistory(id, time.Local)
	if err != nil {
		return err
	}
	history.Reference = history.Closes[0] // where the price stood chartDays ago
	md := c.MarketData

	img := render.NewImage(800, 480)
	black := color.Black

	// Header: ticker in large type, then name and market-cap rank. The battery
	// icon occupies the top-right 50 pixels, so the subtitle must stop before it.
	ticker := strings.ToUpper(c.Symbol)
	if ticker == "" {
		ticker = strings.ToUpper(id)
	}
	if err := render.AddText(img, ticker, image.Point{20, 52}, black, 44); err != nil {
		return err
	}
	tickerW, err := render.TextWidth(ticker, 44)
	if err != nil {
		return err
	}
	subX := 20 + tickerW + 20
	subtitle := c.Name
	if c.MarketCapRank > 0 {
		subtitle += fmt.Sprintf(" · #%d", c.MarketCapRank)
	}
	if w, _ := render.TextWidth(subtitle, 24); subX+w > 740 {
		subtitle = c.Name
	}
	if err := render.AddText(img, subtitle, image.Point{subX, 52}, black, 24); err != nil {
		return err
	}

	// Price row: price, then trend icon and the 24-hour change.
	priceText := formatPrice(md.CurrentPrice.USD) + " USD"
	if err := render.AddText(img, priceText, image.Point{20, 122}, black, 56); err != nil {
		return err
	}
	priceW, err := render.TextWidth(priceText, 56)
	if err != nil {
		return err
	}
	changeX := 20 + priceW + 30
	// AddIcon takes the negated destination position (see render.AddIcon).
	if err := render.AddIcon(img, icons.Trend(md.Change24h), image.Point{-changeX, -86}, 40); err != nil {
		return err
	}
	changeText := fmt.Sprintf("%s (%s) 24h", formatSignedPrice(md.Change24h), formatPct(md.ChangePct24h))
	if err := render.AddText(img, changeText, image.Point{changeX + 48, 118}, black, 30); err != nil {
		return err
	}

	// One compact stats line. Volume is appended only when it fits.
	stats := fmt.Sprintf("7d %s   30d %s   1y %s   Cap %s   ATH %s",
		formatPct(md.ChangePct7d), formatPct(md.ChangePct30d), formatPct(md.ChangePct1y),
		formatBig(md.MarketCap.USD), formatPct(md.AthChangePct.USD))
	if withVol := stats + "   Vol " + formatBig(md.TotalVolume.USD); fits(withVol, 22, 760) {
		stats = withVol
	}
	if err := render.AddText(img, stats, image.Point{20, 168}, black, 22); err != nil {
		return err
	}

	// Chart across the full width.
	if err := render.AddCloseChart(img, history, 760, 225, image.Point{20, 185}); err != nil {
		return err
	}

	// Footer: the update time on the right, and a 24-hour low-to-high range bar
	// on the left sized to whatever room is left.
	updated := updatedLine(md.LastUpdated, time.Local)
	rightLimit := 780
	if updated != "" {
		updatedW, err := render.TextWidth(updated, 22)
		if err != nil {
			return err
		}
		if err := render.AddText(img, updated, image.Point{780 - updatedW, 462}, black, 22); err != nil {
			return err
		}
		rightLimit = 780 - updatedW - 30
	}
	lo, hi := md.Low24h.USD, md.High24h.USD
	if hi > lo {
		if err := render.AddText(img, "24h", image.Point{20, 462}, black, 22); err != nil {
			return err
		}
		loText, hiText := formatPrice(lo), formatPrice(hi)
		if err := render.AddText(img, loText, image.Point{70, 462}, black, 22); err != nil {
			return err
		}
		loW, err := render.TextWidth(loText, 22)
		if err != nil {
			return err
		}
		hiW, err := render.TextWidth(hiText, 22)
		if err != nil {
			return err
		}
		barX := 70 + loW + 12
		barW := min(260, rightLimit-hiW-12-barX)
		if barW >= 60 {
			if err := render.AddRangeBar(img, lo, hi, md.CurrentPrice.USD, image.Rect(barX, 444, barX+barW, 464)); err != nil {
				return err
			}
			if err := render.AddText(img, hiText, image.Point{barX + barW + 12, 462}, black, 22); err != nil {
				return err
			}
		}
	}

	return render.WriteFile(outputPath, img, voltage)
}

// fits reports whether text at size is at most maxW pixels wide.
func fits(text string, size float64, maxW int) bool {
	w, err := render.TextWidth(text, size)
	return err == nil && w <= maxW
}
