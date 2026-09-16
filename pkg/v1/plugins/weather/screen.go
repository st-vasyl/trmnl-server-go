package weather

import (
	"fmt"
	"image"
	"image/color"
	"time"
	"trmnl-server-go/pkg/v1/icons"
	"trmnl-server-go/pkg/v1/render"
)

// Layout. The screen is a header over two panels: current conditions on the
// left, one forecast row per day on the right. All positions are destination
// pixels; AddIcon takes them negated (see render.AddIcon).
const (
	screenW, screenH = 800, 480
	marginX          = 20
	contentRight     = screenW - marginX
	splitX           = 400 // dotted divider between the two panels
	batteryLeft      = 740 // render.WriteFile stamps the battery icon at x=750
	dotStep          = 3   // spacing of the dotted rules and the divider

	headerBaseline = 52
	headerRuleY    = 66
	citySize       = 44.0
	dateSize       = 24.0

	// Hero: condition icon and the temperature in the largest type. Four
	// characters ("-12°", "104°") at 96px are about 212px, so from x=168 the
	// number stays inside the panel.
	heroIconX, heroIconY, heroIconSize = 28, 86, 120
	heroTempX, heroTempBaseline        = 168, 188
	heroTempSize                       = 96.0
	conditionBaseline, conditionSize   = 240, 30.0
	rangeBaseline, rangeSize           = 274, 22.0

	// Stats grid: two columns, four rows of icon + value. The widest value,
	// "G 25.0" or "12.3 km/h", is well under the 178px column.
	statCol1, statCol2 = 44, 222
	statFirstBaseline  = 318
	statRowH           = 38
	statIconSize       = 28
	statTextOffset     = 36
	statSize           = 24.0

	// Forecast table: rows of weekday, icon, high over low, rain over wind.
	tableTop, tableBottom = 80, 440
	rowH                  = 88
	dayX                  = splitX + 24
	daySize               = 28.0
	rowIconX, rowIconSize = splitX + 90, 56
	tempX                 = splitX + 164
	hiSize, loSize        = 30.0, 22.0
	rowStatIconX          = splitX + 250
	rowStatIconSize       = 26
	rowStatX              = splitX + 282
	rowStatSize           = 24.0

	footerBaseline, footerSize = 468, 22.0
)

var black = color.Black

func renderScreen(v view, outputPath string, voltage float32) error {
	img := render.NewImage(screenW, screenH)

	// Header: city in large type, the date beside it, and a rule under both.
	if err := render.AddText(img, v.City, image.Point{marginX, headerBaseline}, black, citySize); err != nil {
		return err
	}
	date, err := headerDate(v.City, v.Updated)
	if err != nil {
		return err
	}
	if date != "" {
		cityW, err := render.TextWidth(v.City, citySize)
		if err != nil {
			return err
		}
		if err := render.AddText(img, date, image.Point{marginX + cityW + 20, headerBaseline}, black, dateSize); err != nil {
			return err
		}
	}
	render.AddDottedLine(img, marginX, contentRight, headerRuleY, dotStep)

	// Hero.
	if name := weatherIconByCode(v.Code); name != "" {
		if err := render.AddIcon(img, name, image.Point{-heroIconX, -heroIconY}, heroIconSize); err != nil {
			return err
		}
	}
	if err := render.AddText(img, formatDegrees(v.Temp), image.Point{heroTempX, heroTempBaseline}, black, heroTempSize); err != nil {
		return err
	}
	if cond := conditionText(v.Code); cond != "" {
		if err := render.AddText(img, cond, image.Point{statCol1, conditionBaseline}, black, conditionSize); err != nil {
			return err
		}
	}
	rangeLine := fmt.Sprintf("Feels %s  H %s  L %s", formatDegrees(v.Feels), formatDegrees(v.Hi), formatDegrees(v.Lo))
	if err := render.AddText(img, rangeLine, image.Point{statCol1, rangeBaseline}, black, rangeSize); err != nil {
		return err
	}

	// Stats grid.
	cells := []struct {
		icon, value string
		x, row      int
	}{
		{humidityIcon(v.Humidity), fmt.Sprintf("%d%%", v.Humidity), statCol1, 0},
		{icons.Wind, fmt.Sprintf("%.1f %s", v.Wind, v.WindUnit), statCol2, 0},
		{icons.WindGusts, fmt.Sprintf("G %.1f", v.Gust), statCol1, 1},
		{icons.Umbrella, fmt.Sprintf("%d%%", v.Rain), statCol2, 1},
		{icons.UV, fmt.Sprintf("UV %.0f", v.UV), statCol1, 2},
		{icons.Pressure, fmt.Sprintf("%.0f hPa", v.Pressure), statCol2, 2},
		{icons.Sunrise, v.Sunrise.Format("15:04"), statCol1, 3},
		{icons.Sunset, v.Sunset.Format("15:04"), statCol2, 3},
	}
	for _, c := range cells {
		if err := drawStat(img, c.icon, c.value, c.x, statFirstBaseline+c.row*statRowH); err != nil {
			return err
		}
	}

	// Divider and forecast rows.
	render.AddDitherRect(img, image.Rect(splitX, tableTop, splitX+1, tableBottom), dotStep)
	for i, d := range v.Days {
		y := tableTop + i*rowH
		if y+rowH > tableBottom {
			break
		}
		if i > 0 {
			render.AddDottedLine(img, splitX+20, contentRight, y, dotStep)
		}
		if err := drawRow(img, d, y); err != nil {
			return err
		}
	}

	// Footer: observation time, right-aligned like the other screens.
	if err := drawRight(img, "Updated "+v.Updated.Format("15:04"), contentRight, footerBaseline, footerSize); err != nil {
		return err
	}

	return render.WriteFile(outputPath, img, voltage)
}

// headerDate picks the longest date form that fits between the city name and
// the battery icon: "Tuesday, 15 September", then "Tue 15 Sep", then nothing.
func headerDate(city string, day time.Time) (string, error) {
	cityW, err := render.TextWidth(city, citySize)
	if err != nil {
		return "", err
	}
	x := marginX + cityW + 20
	for _, layout := range []string{"Monday, 2 January", "Mon 2 Jan"} {
		candidate := day.Format(layout)
		w, err := render.TextWidth(candidate, dateSize)
		if err != nil {
			return "", err
		}
		if x+w <= batteryLeft {
			return candidate, nil
		}
	}
	return "", nil
}

// drawStat draws one grid cell: an icon with its value to the right, both
// sitting on the same baseline.
func drawStat(img *image.RGBA, icon, value string, x, baseline int) error {
	if err := render.AddIcon(img, icon, image.Point{-x, -(baseline - statIconSize + 4)}, statIconSize); err != nil {
		return err
	}
	return render.AddText(img, value, image.Point{x + statTextOffset, baseline}, black, statSize)
}

// drawRow draws one forecast day whose row starts at y.
func drawRow(img *image.RGBA, d forecastDay, y int) error {
	if err := render.AddText(img, dayLabel(d.Date), image.Point{dayX, y + 52}, black, daySize); err != nil {
		return err
	}
	if name := weatherIconByCode(d.Code); name != "" {
		if err := render.AddIcon(img, name, image.Point{-rowIconX, -(y + 18)}, rowIconSize); err != nil {
			return err
		}
	}
	if err := render.AddText(img, formatDegrees(d.Hi), image.Point{tempX, y + 44}, black, hiSize); err != nil {
		return err
	}
	if err := render.AddText(img, formatDegrees(d.Lo), image.Point{tempX, y + 74}, black, loSize); err != nil {
		return err
	}
	if err := render.AddIcon(img, icons.Umbrella, image.Point{-rowStatIconX, -(y + 28)}, rowStatIconSize); err != nil {
		return err
	}
	if err := render.AddText(img, fmt.Sprintf("%d%%", d.Rain), image.Point{rowStatX, y + 50}, black, rowStatSize); err != nil {
		return err
	}
	if err := render.AddIcon(img, icons.Wind, image.Point{-rowStatIconX, -(y + 56)}, rowStatIconSize); err != nil {
		return err
	}
	return render.AddText(img, fmt.Sprintf("%.1f", d.Wind), image.Point{rowStatX, y + 78}, black, rowStatSize)
}

// drawRight draws text ending at the given x.
func drawRight(img *image.RGBA, text string, right, baseline int, size float64) error {
	w, err := render.TextWidth(text, size)
	if err != nil {
		return err
	}
	return render.AddText(img, text, image.Point{right - w, baseline}, black, size)
}

// weatherIconByCode picks the condition glyph for a WMO weather code.
func weatherIconByCode(code int) string {
	switch {
	case code == 0:
		return icons.WeatherCode0
	case code == 1 || code == 2:
		return icons.WeatherCode1
	case code == 3:
		return icons.WeatherCode3
	case code == 45 || code == 48:
		return icons.WeatherCode4
	case code > 50 && code < 70:
		return icons.WeatherCode5
	case code > 70 && code < 76:
		return icons.WeatherCode7
	case code == 77:
		return icons.WeatherCode77
	case code == 80 || code == 81 || code == 82:
		return icons.WeatherCode8
	case code == 85 || code == 86:
		return icons.WeatherCode85
	case code > 90:
		return icons.WeatherCode9
	default:
		return ""
	}
}

// humidityIcon picks a drop glyph whose fill grows with the humidity.
func humidityIcon(humidity int) string {
	switch {
	case humidity < 50:
		return icons.HumidityLow
	case humidity < 80:
		return icons.HumidityMid
	default:
		return icons.HumidityHigh
	}
}
