package weather

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"trmnl-server-go/pkg/v1/icons"
)

func TestWeatherPlugin_NameAndScreens(t *testing.T) {
	p := &WeatherPlugin{Location: "Wroclaw"}
	if p.Name() != "weather" {
		t.Errorf("Name = %q, want weather", p.Name())
	}
	got := p.Screens()
	if len(got) != 1 || got[0] != "weather" {
		t.Errorf("Screens = %v, want [weather]", got)
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
			"results": [{"name": "Wroclaw", "latitude": 51.1, "longitude": 17.03, "country": "PL"}]
		}`))
	}))
	defer srv.Close()
	withGeocodingURL(t, srv)

	l, err := getLocation("Wroclaw")
	if err != nil {
		t.Fatalf("getLocation: %v", err)
	}
	if len(l.Results) != 1 || l.Results[0].Name != "Wroclaw" {
		t.Errorf("results = %+v", l.Results)
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

	// getWeather requires l.Results[0] to be set.
	l := locationResponse{Results: []locationResult{{Latitude: 51.1, Longitude: 17.03}}}
	weather, err := getWeather(l)
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
