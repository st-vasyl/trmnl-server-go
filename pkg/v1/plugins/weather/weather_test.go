package weather

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"trmnl-server-go/pkg/v1/icons"
)

func TestWeatherPlugin_NameAndScreens(t *testing.T) {
	p, err := New("Kyiv", "", "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if p.Name() != "weather" {
		t.Errorf("Name = %q, want weather", p.Name())
	}
	got := p.Screens()
	if len(got) != 1 || got[0] != "weather" {
		t.Errorf("Screens = %v, want [weather]", got)
	}
}

func TestNew_DefaultsToCelsiusAndMetersPerSecond(t *testing.T) {
	p, err := New("Kyiv", "", "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if p.temperatureUnit != "celsius" {
		t.Errorf("temperatureUnit = %q, want celsius", p.temperatureUnit)
	}
	if p.windSpeedUnit != "ms" {
		t.Errorf("windSpeedUnit = %q, want ms", p.windSpeedUnit)
	}
}

func TestNew_AcceptsEveryOpenMeteoUnit(t *testing.T) {
	for _, tu := range []string{"celsius", "fahrenheit"} {
		for _, wu := range []string{"ms", "kmh", "mph", "kn"} {
			if _, err := New("Denver", tu, wu); err != nil {
				t.Errorf("New(%q, %q): %v", tu, wu, err)
			}
		}
	}
}

func TestNew_RejectsUnknownTemperatureUnit(t *testing.T) {
	if _, err := New("Kyiv", "kelvin", ""); err == nil {
		t.Fatal("expected error for temperature_unit kelvin")
	}
}

func TestNew_RejectsUnknownWindSpeedUnit(t *testing.T) {
	if _, err := New("Kyiv", "", "knots"); err == nil {
		t.Fatal("expected error for wind_speed_unit knots")
	}
}

func TestNew_RequiresLocation(t *testing.T) {
	if _, err := New("  ", "", ""); err == nil {
		t.Fatal("expected error for an empty location")
	}
}

func TestGetLocation_NoResultsIsAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"generationtime_ms": 0.5}`))
	}))
	defer srv.Close()
	withGeocodingURL(t, srv)

	if _, err := getLocation("Nowhere"); err == nil {
		t.Fatal("expected error when geocoding returns no results")
	}
}

func TestGetLocation_EscapesCityName(t *testing.T) {
	var gotName string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotName = r.URL.Query().Get("name")
		w.Write([]byte(`{"results": [{"name": "New York", "latitude": 40.7, "longitude": -74.0, "country": "US"}]}`))
	}))
	defer srv.Close()
	withGeocodingURL(t, srv)

	if _, err := getLocation("New York"); err != nil {
		t.Fatalf("getLocation: %v", err)
	}
	if gotName != "New York" {
		t.Errorf("name query = %q, want %q", gotName, "New York")
	}
}

func TestGetWeather_RequestsConfiguredUnitsAndFields(t *testing.T) {
	var query url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.Query()
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	withForecastURL(t, srv)

	loc := locationResult{Latitude: 39.7, Longitude: -104.9}
	if _, err := getWeather(loc, "fahrenheit", "mph"); err != nil {
		t.Fatalf("getWeather: %v", err)
	}
	if got := query.Get("temperature_unit"); got != "fahrenheit" {
		t.Errorf("temperature_unit = %q, want fahrenheit", got)
	}
	if got := query.Get("wind_speed_unit"); got != "mph" {
		t.Errorf("wind_speed_unit = %q, want mph", got)
	}
	if got := query.Get("forecast_days"); got != "5" {
		t.Errorf("forecast_days = %q, want 5", got)
	}
	if got := query.Get("timezone"); got != "auto" {
		t.Errorf("timezone = %q, want auto", got)
	}
	daily := query.Get("daily")
	for _, field := range []string{"sunrise", "sunset", "uv_index_max", "precipitation_probability_max", "temperature_2m_min"} {
		if !strings.Contains(daily, field) {
			t.Errorf("daily = %q, missing %s", daily, field)
		}
	}
}

func withGeocodingURL(t *testing.T, srv *httptest.Server) {
	t.Helper()
	orig := geocodingBaseURL
	geocodingBaseURL = srv.URL
	t.Cleanup(func() { geocodingBaseURL = orig })
}

func withForecastURL(t *testing.T, srv *httptest.Server) {
	t.Helper()
	orig := forecastBaseURL
	forecastBaseURL = srv.URL
	t.Cleanup(func() { forecastBaseURL = orig })
}

func TestGetLocation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/v1/search") {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		w.Write([]byte(`{
			"results": [{"name": "Kyiv", "latitude": 50.45, "longitude": 30.52, "country": "UA"}]
		}`))
	}))
	defer srv.Close()
	withGeocodingURL(t, srv)

	l, err := getLocation("Kyiv")
	if err != nil {
		t.Fatalf("getLocation: %v", err)
	}
	if l.Name != "Kyiv" || l.Latitude != 50.45 || l.Longitude != 30.52 {
		t.Errorf("location = %+v", l)
	}
}

func TestGetWeather(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/v1/forecast") {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		w.Write([]byte(`{
			"elevation": 100,
			"current_units": {"temperature_2m": "°C"},
			"current": {
				"time": "2024-01-01T10:00",
				"temperature_2m": 12.3,
				"apparent_temperature": 10.5,
				"relative_humidity_2m": 55,
				"wind_speed_10m": 3.2,
				"wind_gusts_10m": 5.0,
				"surface_pressure": 1013.0,
				"weather_code": 3
			},
			"daily": {
				"time": ["2024-01-01"],
				"temperature_2m_max": [14.0],
				"temperature_2m_min": [5.0],
				"weather_code": [3],
				"wind_speed_10m_max": [7.5],
				"precipitation_probability_max": [40]
			}
		}`))
	}))
	defer srv.Close()
	withForecastURL(t, srv)

	weather, err := getWeather(locationResult{Latitude: 50.45, Longitude: 30.52}, "celsius", "ms")
	if err != nil {
		t.Fatalf("getWeather: %v", err)
	}
	if weather.Current.Temperature2m != 12.3 {
		t.Errorf("Temperature2m = %v, want 12.3", weather.Current.Temperature2m)
	}
	if weather.Current.WeatherCode != 3 {
		t.Errorf("WeatherCode = %d, want 3", weather.Current.WeatherCode)
	}
	if len(weather.Daily.TMax) != 1 || weather.Daily.TMax[0] != 14.0 {
		t.Errorf("Daily.TMax = %v, want [14]", weather.Daily.TMax)
	}
	if len(weather.Daily.WeatherCode) != 1 || weather.Daily.WeatherCode[0] != 3 {
		t.Errorf("Daily.WeatherCode = %v, want [3]", weather.Daily.WeatherCode)
	}
	if len(weather.Daily.Wind) != 1 || weather.Daily.Wind[0] != 7.5 {
		t.Errorf("Daily.Wind = %v, want [7.5]", weather.Daily.Wind)
	}
}

func TestWeatherIconByCode(t *testing.T) {
	tests := []struct {
		code int
		want string
	}{
		{0, icons.WeatherCode0},
		{1, icons.WeatherCode1},
		{2, icons.WeatherCode1},
		{3, icons.WeatherCode3},
		{45, icons.WeatherCode4},
		{48, icons.WeatherCode4},
		{55, icons.WeatherCode5}, // in (50,70)
		{72, icons.WeatherCode7}, // in (70,76)
		{77, icons.WeatherCode77},
		{80, icons.WeatherCode8},
		{81, icons.WeatherCode8},
		{82, icons.WeatherCode8},
		{85, icons.WeatherCode85},
		{86, icons.WeatherCode85},
		{95, icons.WeatherCode9},
		{4, ""}, // unmatched -> empty
	}
	for _, tc := range tests {
		if got := weatherIconByCode(tc.code); got != tc.want {
			t.Errorf("weatherIconByCode(%d) returned %d bytes, want %d (code %d)", tc.code, len(got), len(tc.want), tc.code)
		}
	}
}

func TestHumidityIcon(t *testing.T) {
	if humidityIcon(30) != icons.HumidityLow {
		t.Error("humidityIcon(30) want low")
	}
	if humidityIcon(60) != icons.HumidityMid {
		t.Error("humidityIcon(60) want mid")
	}
	if humidityIcon(90) != icons.HumidityHigh {
		t.Error("humidityIcon(90) want high")
	}
	// Boundary: 50 falls into "mid" (humidity < 50 is low).
	if humidityIcon(50) != icons.HumidityMid {
		t.Error("humidityIcon(50) want mid")
	}
	// Boundary: 80 falls into "high" (humidity < 80 is mid).
	if humidityIcon(80) != icons.HumidityHigh {
		t.Error("humidityIcon(80) want high")
	}
}

func TestGetLocation_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`not json`))
	}))
	defer srv.Close()
	withGeocodingURL(t, srv)

	if _, err := getLocation("X"); err == nil {
		t.Fatal("expected JSON error")
	}
}
