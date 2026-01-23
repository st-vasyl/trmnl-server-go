package weather

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"io"
	"net/http"
	"time"
	"trmnl-server-go/pkg/v1/icons"
	"trmnl-server-go/pkg/v1/render"
)

type Location struct {
	LocationResult []LocationResult `json:"results"`
}

type LocationResult struct {
	Name      string  `json:"name"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Country   string  `json:"country"`
}

type Weather struct {
	Elevation    float64      `json:"elevation"`
	CurrentUnits WeatherUnits `json:"current_units"`
	Current      WeatherData  `json:"current"`
	Hourly       Hourly       `json:"hourly"`
	Daily        Daily        `json:"daily"`
}

type Hourly struct {
	Time        []string  `json:"time"`
	Temperature []float64 `json:"temperature_2m"`
}

type Daily struct {
	Time    []string  `json:"time"`
	Sunrise []string  `json:"sunrise"`
	Sunset  []string  `json:"sunset"`
	TMax    []float64 `json:"temperature_2m_max"`
	TMin    []float64 `json:"temperature_2m_min"`
}

type WeatherUnits struct {
	Time                 string `json:"time"`
	Interval             string `json:"interval"`
	Temperature_2m       string `json:"temperature_2m"`
	Apparent_temperature string `json:"apparent_temperature"`
	Relative_humidity_2m string `json:"relative_humidity_2m"`
	Wind_speed_10m       string `json:"wind_speed_10m"`
	Wind_gusts_10m       string `json:"wind_gusts_10m"`
	Surface_pressure     string `json:"surface_pressure"`
	Weather_code         string `json:"weather_code"`
}

type WeatherData struct {
	Time                 string  `json:"time"`
	Interval             int     `json:"interval"`
	Temperature_2m       float64 `json:"temperature_2m"`
	Apparent_temperature float64 `json:"apparent_temperature"`
	Relative_humidity_2m int     `json:"relative_humidity_2m"`
	Wind_speed_10m       float64 `json:"wind_speed_10m"`
	Wind_gusts_10m       float64 `json:"wind_gusts_10m"`
	Surface_pressure     float64 `json:"surface_pressure"`
	Weather_code         int     `json:"weather_code"`
}

func getLocation(city string) (Location, error) {
	var l Location

	url := fmt.Sprintf("https://geocoding-api.open-meteo.com/v1/search?name=%s&count=1&language=en&format=json", city)
	r, err := http.Get(url)
	r.Header.Set("Accept", "application/json")
	r.Header.Set("Accept-Language", "en-US")
	if err != nil {
		return l, err
	}
	body, err := io.ReadAll(r.Body)
	r.Body.Close()
	if err != nil {
		return l, err
	}

	err = json.Unmarshal([]byte(body), &l)
	if err != nil {
		panic(err)
	}
	return l, nil
}

func getWeather(l Location) (Weather, error) {
	var w Weather

	url := fmt.Sprintf("https://api.open-meteo.com/v1/forecast?latitude=%f&longitude=%f&current=temperature_2m,apparent_temperature,relative_humidity_2m,wind_speed_10m,wind_gusts_10m,surface_pressure,weather_code&hourly=temperature_2m&daily=temperature_2m_max,temperature_2m_min,sunset,sunrise&wind_speed_unit=ms&timezone=auto", l.LocationResult[0].Latitude, l.LocationResult[0].Longitude)
	r, err := http.Get(url)
	r.Header.Set("Accept", "application/json")
	r.Header.Set("Accept-Language", "en-US")
	if err != nil {
		return w, err
	}
	body, err := io.ReadAll(r.Body)
	r.Body.Close()
	if err != nil {
		return w, err
	}

	err = json.Unmarshal([]byte(body), &w)
	if err != nil {
		panic(err)
	}

	return w, nil
}

func RenderScreenWeather(width, height int, city, filename string, voltage float32) error {
	l, _ := getLocation(city)
	w, _ := getWeather(l)
	img := render.NewImage(width, height)
	weatherImage := getWeatherConditionByCode(w.Current.Weather_code)

	var temperature render.ChartRecords
	for i := range len(w.Hourly.Time) {
		var tmp render.ChartRecord
		t, _ := time.Parse("2006-01-02T15:04", w.Hourly.Time[i])
		tmp.T = float64(t.UnixMilli())
		tmp.V = w.Hourly.Temperature[i]
		temperature.ChartRecord = append(temperature.ChartRecord, tmp)
	}

	// Adding Weather conditions icon
	if err := render.AddImageFromBase64(img, weatherImage, image.Point{-50, 0}); err != nil {
		return err
	}

	// Adding current temperature
	if err := render.AddText(img, fmt.Sprintf("%.1f%s", w.Current.Temperature_2m, w.CurrentUnits.Temperature_2m), image.Point{30, 170}, color.Black, 50); err != nil {
		return err
	}

	// Adding feels like temperature icon
	if err := render.AddImageFromBase64(img, icons.Temperature, image.Point{-293, -20}); err != nil {
		return err
	}

	if err := render.AddText(img, fmt.Sprintf("%.1f%s", w.Current.Apparent_temperature, w.CurrentUnits.Temperature_2m), image.Point{360, 50}, color.Black, 30); err != nil {
		return err
	}

	// Adding max temperature icon
	if err := render.AddImageFromBase64(img, icons.TemperatureHigh, image.Point{-300, -70}); err != nil {
		return err
	}

	// Adding max temperature
	if err := render.AddText(img, fmt.Sprintf("%.1f%s", w.Daily.TMax[0], w.CurrentUnits.Temperature_2m), image.Point{360, 100}, color.Black, 30); err != nil {
		return err
	}

	// Adding min temperature icon
	if err := render.AddImageFromBase64(img, icons.TemperatureLow, image.Point{-300, -120}); err != nil {
		return err
	}

	// Adding min temperature
	if err := render.AddText(img, fmt.Sprintf("%.1f%s", w.Daily.TMin[0], w.CurrentUnits.Temperature_2m), image.Point{360, 150}, color.Black, 30); err != nil {
		return err
	}

	// Add humidity icon
	if err := render.AddImageFromBase64(img, getHumidity(w.Current.Relative_humidity_2m), image.Point{-530, -20}); err != nil {
		return err
	}

	// Add humidity
	if err := render.AddText(img, fmt.Sprintf("%d%s", w.Current.Relative_humidity_2m, w.CurrentUnits.Relative_humidity_2m), image.Point{590, 50}, color.Black, 30); err != nil {
		return err
	}

	// Add Wind icon
	if err := render.AddImageFromBase64(img, icons.Wind, image.Point{-530, -70}); err != nil {
		return err
	}

	// Add Wind
	if err := render.AddText(img, fmt.Sprintf("%.1fm/s", w.Current.Wind_speed_10m), image.Point{590, 100}, color.Black, 30); err != nil {
		return err
	}

	// Add Wind Gusts icon
	if err := render.AddImageFromBase64(img, icons.WindGusts, image.Point{-530, -120}); err != nil {
		return err
	}

	// Add Wind Gusts
	if err := render.AddText(img, fmt.Sprintf("%.1fm/s", w.Current.Wind_gusts_10m), image.Point{590, 150}, color.Black, 30); err != nil {
		return err
	}

	if err := render.AddChart(img, temperature, 550, 200, image.Point{-30, -200}); err != nil {
		return err
	}

	if err := render.WriteFile(filename, img, voltage); err != nil {
		return err
	}

	return nil
}

func getWeatherConditionByCode(weatherCode int) string {
	var weatherImage string

	switch {
	case weatherCode == 0:
		weatherImage = icons.WeatherCode0
	case weatherCode == 1:
		weatherImage = icons.WeatherCode1
	case weatherCode == 2:
		weatherImage = icons.WeatherCode1
	case weatherCode == 3:
		weatherImage = icons.WeatherCode3
	case weatherCode == 45 || weatherCode == 48:
		weatherImage = icons.WeatherCode4
	case weatherCode > 50 && weatherCode < 70:
		weatherImage = icons.WeatherCode5
	case weatherCode > 70 && weatherCode < 76:
		weatherImage = icons.WeatherCode7
	case weatherCode == 77:
		weatherImage = icons.WeatherCode77
	case weatherCode == 80 || weatherCode == 81 || weatherCode == 82:
		weatherImage = icons.WeatherCode8
	case weatherCode == 85 || weatherCode == 86:
		weatherImage = icons.WeatherCode85
	case weatherCode > 90:
		weatherImage = icons.WeatherCode9
	}
	return weatherImage
}

func getHumidity(humidity int) string {
	var humidityImage string

	switch {
	case humidity < 50:
		humidityImage = icons.HumidityLow
	case humidity >= 50 && humidity < 80:
		humidityImage = icons.HumidityMid
	case humidity >= 80:
		humidityImage = icons.HumidityHigh
	}
	return humidityImage
}
