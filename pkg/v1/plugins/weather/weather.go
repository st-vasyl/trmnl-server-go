package weather

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"trmnl-server-go/pkg/v1/httpclient"
	"trmnl-server-go/pkg/v1/icons"
	"trmnl-server-go/pkg/v1/render"
)

// API roots. Overridden in tests.
var (
	geocodingBaseURL = "https://geocoding-api.open-meteo.com"
	forecastBaseURL  = "https://api.open-meteo.com"
)

// WeatherPlugin renders the current weather and hourly forecast for a configured city.
type WeatherPlugin struct {
	Location string
}

func (p *WeatherPlugin) Name() string      { return "weather" }
func (p *WeatherPlugin) Screens() []string { return []string{"weather"} }
func (p *WeatherPlugin) Render(_ string, outputPath string, voltage float32) error {
	return renderScreen(p.Location, outputPath, voltage)
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
	Elevation    float64      `json:"elevation"`
	CurrentUnits weatherUnits `json:"current_units"`
	Current      weatherData  `json:"current"`
	Daily        daily        `json:"daily"`
}

type daily struct {
	Time          []string  `json:"time"`
	TMax          []float64 `json:"temperature_2m_max"`
	TMin          []float64 `json:"temperature_2m_min"`
	Wind          []float64 `json:"wind_speed_10m_max"`
	WeatherCode   []int     `json:"weather_code"`
	Precipitation []int     `json:"precipitation_probability_max"`
}

type weatherUnits struct {
	Time                string `json:"time"`
	Interval            string `json:"interval"`
	Temperature2m       string `json:"temperature_2m"`
	ApparentTemperature string `json:"apparent_temperature"`
	RelativeHumidity2m  string `json:"relative_humidity_2m"`
	WindSpeed10m        string `json:"wind_speed_10m"`
	WindGusts10m        string `json:"wind_gusts_10m"`
	SurfacePressure     string `json:"surface_pressure"`
	WeatherCode         string `json:"weather_code"`
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

func getLocation(city string) (locationResponse, error) {
	var l locationResponse
	url := fmt.Sprintf("%s/v1/search?name=%s&count=1&language=en&format=json", geocodingBaseURL, city)
	body, err := httpclient.Get(url)
	if err != nil {
		return l, err
	}
	if err := json.Unmarshal(body, &l); err != nil {
		return l, err
	}
	return l, nil
}

func getWeather(l locationResponse) (weatherResponse, error) {
	var w weatherResponse
	url := fmt.Sprintf(
		"%s/v1/forecast?latitude=%f&longitude=%f"+
			"&current=temperature_2m,apparent_temperature,relative_humidity_2m,wind_speed_10m,wind_gusts_10m,surface_pressure,weather_code"+
			"&daily=temperature_2m_max,temperature_2m_min,weather_code,wind_speed_10m_max,precipitation_probability_max"+
			"&wind_speed_unit=ms&timezone=auto",
		forecastBaseURL, l.Results[0].Latitude, l.Results[0].Longitude,
	)
	body, err := httpclient.Get(url)
	if err != nil {
		return w, err
	}
	if err := json.Unmarshal(body, &w); err != nil {
		return w, err
	}
	return w, nil
}

func renderScreen(city, outputPath string, voltage float32) error {
	l, err := getLocation(city)
	if err != nil {
		return err
	}
	w, err := getWeather(l)
	if err != nil {
		return err
	}

	img := render.NewImage(800, 480)

	if err := render.AddIcon(img, weatherIconByCode(w.Current.WeatherCode), image.Point{-50, 0}, 128); err != nil {
		return err
	}
	if err := render.AddText(img, fmt.Sprintf("%.1f %s", w.Current.Temperature2m, w.CurrentUnits.Temperature2m), image.Point{30, 170}, color.Black, 50); err != nil {
		return err
	}
	if err := render.AddIcon(img, icons.Temperature, image.Point{-300, -20}, 40); err != nil {
		return err
	}
	if err := render.AddText(img, fmt.Sprintf("%.1f %s", w.Current.ApparentTemperature, w.CurrentUnits.Temperature2m), image.Point{360, 50}, color.Black, 30); err != nil {
		return err
	}
	if err := render.AddIcon(img, icons.TemperatureHigh, image.Point{-300, -70}, 40); err != nil {
		return err
	}
	if err := render.AddText(img, fmt.Sprintf("%.1f %s", w.Daily.TMax[0], w.CurrentUnits.Temperature2m), image.Point{360, 100}, color.Black, 30); err != nil {
		return err
	}
	if err := render.AddIcon(img, icons.TemperatureLow, image.Point{-300, -120}, 40); err != nil {
		return err
	}
	if err := render.AddText(img, fmt.Sprintf("%.1f %s", w.Daily.TMin[0], w.CurrentUnits.Temperature2m), image.Point{360, 150}, color.Black, 30); err != nil {
		return err
	}
	if err := render.AddIcon(img, humidityIcon(w.Current.RelativeHumidity2m), image.Point{-530, -20}, 40); err != nil {
		return err
	}
	if err := render.AddText(img, fmt.Sprintf("%d %s", w.Current.RelativeHumidity2m, w.CurrentUnits.RelativeHumidity2m), image.Point{590, 50}, color.Black, 30); err != nil {
		return err
	}
	if err := render.AddIcon(img, icons.Wind, image.Point{-530, -70}, 40); err != nil {
		return err
	}
	if err := render.AddText(img, fmt.Sprintf("%.1f m/s", w.Current.WindSpeed10m), image.Point{590, 100}, color.Black, 30); err != nil {
		return err
	}
	if err := render.AddIcon(img, icons.WindGusts, image.Point{-530, -120}, 40); err != nil {
		return err
	}
	if err := render.AddText(img, fmt.Sprintf("%.1f m/s", w.Current.WindGusts10m), image.Point{590, 150}, color.Black, 30); err != nil {
		return err
	}
	// Forecast text
	if err := render.AddText(img, "3-DAY FORECAST", image.Point{300, 220}, color.Black, 30); err != nil {
		return err
	}

	// Add wheather Day+1
	for i := 0; i < 3; i++ {
		shift := 250 * i
		if err := render.AddIcon(img, weatherIconByCode(w.Daily.WeatherCode[i+1]), image.Point{-80 - shift, -250}, 100); err != nil {
			return err
		}
		if err := render.AddIcon(img, icons.Temperature, image.Point{-50 - shift, -377}, 30); err != nil {
			return err
		}
		if err := render.AddText(img, fmt.Sprintf("%.1f %s", w.Daily.TMax[i+1], w.CurrentUnits.Temperature2m), image.Point{100 + shift, 400}, color.Black, 30); err != nil {
			return err
		}
		if err := render.AddIcon(img, icons.Wind, image.Point{-50 - shift, -407}, 30); err != nil {
			return err
		}
		if err := render.AddText(img, fmt.Sprintf("%.1f m/s", w.Daily.Wind[i+1]), image.Point{100 + shift, 430}, color.Black, 30); err != nil {
			return err
		}
		if err := render.AddIcon(img, icons.WeatherCode5, image.Point{-50 - shift, -437}, 30); err != nil {
			return err
		}
		if err := render.AddText(img, fmt.Sprintf("%d %s", w.Daily.Precipitation[i]+1, w.CurrentUnits.RelativeHumidity2m), image.Point{100 + shift, 460}, color.Black, 30); err != nil {
			return err
		}
	}

	return render.WriteFile(outputPath, img, voltage)
}

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
