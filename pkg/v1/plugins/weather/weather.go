package weather

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"io"
	"net/http"
	"time"
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
	Daily        Daily        `json:"daily"`
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

	url := fmt.Sprintf("https://api.open-meteo.com/v1/forecast?latitude=%f&longitude=%f&current=temperature_2m,apparent_temperature,relative_humidity_2m,wind_speed_10m,surface_pressure,weather_code&daily=temperature_2m_max,temperature_2m_min,sunset,sunrise&timezone=auto", l.LocationResult[0].Latitude, l.LocationResult[0].Longitude)
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

	var daily_min render.ChartRecords
	var daily_max render.ChartRecords
	for i := range len(w.Daily.Time) {
		var min render.ChartRecord
		var max render.ChartRecord
		t, _ := time.Parse("2006-01-02", w.Daily.Time[i])
		min.T = float64(t.UnixMilli())
		min.V = w.Daily.TMin[i]
		max.T = float64(t.UnixMilli())
		max.V = w.Daily.TMax[i]
		daily_min.ChartRecord = append(daily_min.ChartRecord, min)
		daily_max.ChartRecord = append(daily_max.ChartRecord, max)
	}

	if err := render.AddText(img, fmt.Sprintf("%.1f %s", w.Current.Temperature_2m, w.CurrentUnits.Temperature_2m), image.Point{50, 50}, color.Black, 50); err != nil {
		return err
	}

	if err := render.AddText(img, fmt.Sprintf("Feels: %.1f %s", w.Current.Apparent_temperature, w.CurrentUnits.Temperature_2m), image.Point{50, 100}, color.Black, 30); err != nil {
		return err
	}

	if err := render.AddText(img, fmt.Sprintf("Humidity: %d %s", w.Current.Relative_humidity_2m, w.CurrentUnits.Relative_humidity_2m), image.Point{50, 150}, color.Black, 30); err != nil {
		return err
	}

	if err := render.AddText(img, fmt.Sprintf("Wind: %.1f m/s", w.Current.Wind_speed_10m*10/36), image.Point{400, 50}, color.Black, 30); err != nil {
		return err
	}

	if err := render.AddText(img, fmt.Sprintf("Min: %.1f %s", w.Daily.TMin[0], w.CurrentUnits.Temperature_2m), image.Point{400, 100}, color.Black, 30); err != nil {
		return err
	}

	if err := render.AddText(img, fmt.Sprintf("Max: %.1f %s", w.Daily.TMax[0], w.CurrentUnits.Temperature_2m), image.Point{400, 150}, color.Black, 30); err != nil {
		return err
	}

	if err := render.AddWeatherChart(img, daily_min, daily_max, 550, 200, image.Point{-30, -200}); err != nil {
		return err
	}

	if err := render.WriteFile(filename, img, voltage); err != nil {
		return err
	}

	return nil
}
