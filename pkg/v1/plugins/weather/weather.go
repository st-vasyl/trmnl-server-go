package weather

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"trmnl-server-go/pkg/v1/httpclient"
)

// API roots. Overridden in tests.
var (
	geocodingBaseURL = "https://geocoding-api.open-meteo.com"
	forecastBaseURL  = "https://api.open-meteo.com"
)

const (
	pluginName = "weather"

	// forecastDays is how many days the forecast request asks for: today plus
	// the four rows the screen shows.
	forecastDays = 5
)

// temperatureUnits are the values Open-Meteo accepts for temperature_unit.
var temperatureUnits = map[string]bool{"celsius": true, "fahrenheit": true}

// windSpeedUnits maps the values Open-Meteo accepts for wind_speed_unit to
// the label printed after a speed.
var windSpeedUnits = map[string]string{"ms": "m/s", "kmh": "km/h", "mph": "mph", "kn": "kn"}

// Plugin renders the current conditions and a four-day forecast for one city.
type Plugin struct {
	location        string
	temperatureUnit string
	windSpeedUnit   string
}

// New validates the configuration and returns the plugin. Empty units default
// to celsius and m/s; an unknown unit or a blank location is an error so the
// server stops at startup instead of rendering a broken screen.
func New(location, temperatureUnit, windSpeedUnit string) (*Plugin, error) {
	location = strings.TrimSpace(location)
	if location == "" {
		return nil, fmt.Errorf("weather: location is required")
	}
	if temperatureUnit == "" {
		temperatureUnit = "celsius"
	}
	if !temperatureUnits[temperatureUnit] {
		return nil, fmt.Errorf("weather: unknown temperature_unit %q (use celsius or fahrenheit)", temperatureUnit)
	}
	if windSpeedUnit == "" {
		windSpeedUnit = "ms"
	}
	if _, ok := windSpeedUnits[windSpeedUnit]; !ok {
		return nil, fmt.Errorf("weather: unknown wind_speed_unit %q (use ms, kmh, mph or kn)", windSpeedUnit)
	}
	return &Plugin{location: location, temperatureUnit: temperatureUnit, windSpeedUnit: windSpeedUnit}, nil
}

func (p *Plugin) Name() string      { return pluginName }
func (p *Plugin) Screens() []string { return []string{pluginName} }

func (p *Plugin) Render(_ string, outputPath string, voltage float32) error {
	loc, err := getLocation(p.location)
	if err != nil {
		return err
	}
	w, err := getWeather(loc, p.temperatureUnit, p.windSpeedUnit)
	if err != nil {
		return err
	}
	v, err := buildView(loc, w, p.windSpeedUnit)
	if err != nil {
		return err
	}
	return renderScreen(v, outputPath, voltage)
}

type locationResponse struct {
	Results []locationResult `json:"results"`
}

type locationResult struct {
	Name      string  `json:"name"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Country   string  `json:"country"`
}

type weatherResponse struct {
	Current weatherData `json:"current"`
	Daily   daily       `json:"daily"`
}

type daily struct {
	Time          []string  `json:"time"`
	TMax          []float64 `json:"temperature_2m_max"`
	TMin          []float64 `json:"temperature_2m_min"`
	Wind          []float64 `json:"wind_speed_10m_max"`
	WeatherCode   []int     `json:"weather_code"`
	Precipitation []int     `json:"precipitation_probability_max"`
	Sunrise       []string  `json:"sunrise"`
	Sunset        []string  `json:"sunset"`
	UVMax         []float64 `json:"uv_index_max"`
}

type weatherData struct {
	Time                string  `json:"time"`
	Interval            int     `json:"interval"`
	Temperature2m       float64 `json:"temperature_2m"`
	ApparentTemperature float64 `json:"apparent_temperature"`
	RelativeHumidity2m  int     `json:"relative_humidity_2m"`
	WindSpeed10m        float64 `json:"wind_speed_10m"`
	WindGusts10m        float64 `json:"wind_gusts_10m"`
	SurfacePressure     float64 `json:"surface_pressure"`
	WeatherCode         int     `json:"weather_code"`
}

// getLocation geocodes a city name to its best match. An unknown name is an
// error rather than an empty result.
func getLocation(city string) (locationResult, error) {
	q := url.Values{}
	q.Set("name", city)
	q.Set("count", "1")
	q.Set("language", "en")
	q.Set("format", "json")
	body, err := httpclient.Get(geocodingBaseURL + "/v1/search?" + q.Encode())
	if err != nil {
		return locationResult{}, err
	}
	var l locationResponse
	if err := json.Unmarshal(body, &l); err != nil {
		return locationResult{}, err
	}
	if len(l.Results) == 0 {
		return locationResult{}, fmt.Errorf("weather: no location found for %q", city)
	}
	return l.Results[0], nil
}

// getWeather fetches current conditions and the daily forecast for loc in the
// configured units. timezone=auto makes every timestamp local to loc.
func getWeather(loc locationResult, temperatureUnit, windSpeedUnit string) (weatherResponse, error) {
	q := url.Values{}
	q.Set("latitude", fmt.Sprintf("%f", loc.Latitude))
	q.Set("longitude", fmt.Sprintf("%f", loc.Longitude))
	q.Set("current", "temperature_2m,apparent_temperature,relative_humidity_2m,wind_speed_10m,wind_gusts_10m,surface_pressure,weather_code")
	q.Set("daily", "temperature_2m_max,temperature_2m_min,weather_code,wind_speed_10m_max,precipitation_probability_max,sunrise,sunset,uv_index_max")
	q.Set("temperature_unit", temperatureUnit)
	q.Set("wind_speed_unit", windSpeedUnit)
	q.Set("forecast_days", fmt.Sprint(forecastDays))
	q.Set("timezone", "auto")
	body, err := httpclient.Get(forecastBaseURL + "/v1/forecast?" + q.Encode())
	if err != nil {
		return weatherResponse{}, err
	}
	var w weatherResponse
	if err := json.Unmarshal(body, &w); err != nil {
		return weatherResponse{}, err
	}
	return w, nil
}
