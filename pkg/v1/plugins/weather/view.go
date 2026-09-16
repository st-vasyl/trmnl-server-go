package weather

import (
	"fmt"
	"math"
	"time"
)

// apiTime is the layout of Open-Meteo's iso8601 timestamps with timezone=auto;
// they carry no offset because they are already local to the location.
const apiTime = "2006-01-02T15:04"

// view is everything the screen draws, decoupled from the API shape.
type view struct {
	City    string
	Updated time.Time // observation time, local to the location

	Code                int
	Temp, Feels, Hi, Lo float64
	Humidity, Rain      int
	Wind, Gust          float64
	WindUnit            string
	UV, Pressure        float64
	Sunrise, Sunset     time.Time
	Days                []forecastDay // tomorrow onwards
}

// forecastDay is one row of the forecast table.
type forecastDay struct {
	Date   time.Time
	Code   int
	Hi, Lo float64
	Rain   int
	Wind   float64
}

// buildView maps an API response onto the screen model. Today's values come
// from the current block and the first daily entry; the forecast rows are the
// remaining daily entries.
func buildView(loc locationResult, w weatherResponse, windSpeedUnit string) (view, error) {
	d := w.Daily
	n := len(d.Time)
	if n == 0 {
		return view{}, fmt.Errorf("weather: forecast has no daily data")
	}
	for name, l := range map[string]int{
		"temperature_2m_max": len(d.TMax), "temperature_2m_min": len(d.TMin), "weather_code": len(d.WeatherCode),
		"wind_speed_10m_max": len(d.Wind), "precipitation_probability_max": len(d.Precipitation),
		"sunrise": len(d.Sunrise), "sunset": len(d.Sunset), "uv_index_max": len(d.UVMax),
	} {
		if l != n {
			return view{}, fmt.Errorf("weather: daily %s has %d entries, want %d", name, l, n)
		}
	}

	updated, err := time.Parse(apiTime, w.Current.Time)
	if err != nil {
		return view{}, fmt.Errorf("weather: parse current time: %w", err)
	}
	sunrise, err := time.Parse(apiTime, d.Sunrise[0])
	if err != nil {
		return view{}, fmt.Errorf("weather: parse sunrise: %w", err)
	}
	sunset, err := time.Parse(apiTime, d.Sunset[0])
	if err != nil {
		return view{}, fmt.Errorf("weather: parse sunset: %w", err)
	}

	v := view{
		City:     loc.Name,
		Updated:  updated,
		Code:     w.Current.WeatherCode,
		Temp:     w.Current.Temperature2m,
		Feels:    w.Current.ApparentTemperature,
		Hi:       d.TMax[0],
		Lo:       d.TMin[0],
		Humidity: w.Current.RelativeHumidity2m,
		Rain:     d.Precipitation[0],
		Wind:     w.Current.WindSpeed10m,
		Gust:     w.Current.WindGusts10m,
		WindUnit: windSpeedUnits[windSpeedUnit],
		UV:       d.UVMax[0],
		Pressure: w.Current.SurfacePressure,
		Sunrise:  sunrise,
		Sunset:   sunset,
	}
	for i := 1; i < n; i++ {
		date, err := time.Parse("2006-01-02", d.Time[i])
		if err != nil {
			return view{}, fmt.Errorf("weather: parse day %d: %w", i, err)
		}
		v.Days = append(v.Days, forecastDay{
			Date: date,
			Code: d.WeatherCode[i],
			Hi:   d.TMax[i],
			Lo:   d.TMin[i],
			Rain: d.Precipitation[i],
			Wind: d.Wind[i],
		})
	}
	return v, nil
}

// conditionText names a WMO weather code. Unknown codes return "".
func conditionText(code int) string {
	switch code {
	case 0:
		return "Clear"
	case 1:
		return "Mainly clear"
	case 2:
		return "Partly cloudy"
	case 3:
		return "Overcast"
	case 45, 48:
		return "Fog"
	case 51, 53, 55:
		return "Drizzle"
	case 56, 57:
		return "Freezing drizzle"
	case 61:
		return "Light rain"
	case 63:
		return "Rain"
	case 65:
		return "Heavy rain"
	case 66, 67:
		return "Freezing rain"
	case 71:
		return "Light snow"
	case 73:
		return "Snow"
	case 75:
		return "Heavy snow"
	case 77:
		return "Snow grains"
	case 80:
		return "Light showers"
	case 81:
		return "Showers"
	case 82:
		return "Heavy showers"
	case 85, 86:
		return "Snow showers"
	case 95:
		return "Thunderstorm"
	case 96, 99:
		return "Thunderstorm, hail"
	default:
		return ""
	}
}

// formatDegrees prints a temperature rounded to whole degrees with a bare
// degree sign; values that round to zero never print as "-0°".
func formatDegrees(v float64) string {
	r := math.Round(v)
	if r == 0 {
		r = 0 // drops the sign of -0
	}
	return fmt.Sprintf("%.0f°", r)
}

// dayLabel is the short weekday name used in the forecast table.
func dayLabel(d time.Time) string { return d.Format("Mon") }
