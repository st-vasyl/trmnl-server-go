package weather

import (
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"trmnl-server-go/pkg/v1/render"
)

// loadFonts seeds the text font and the icon-font cache from the repo's
// font.ttf so rendering tests hit no network. Skips when the file is absent.
func loadFonts(t *testing.T) {
	t.Helper()
	ttf, err := os.ReadFile("../../../../font.ttf")
	if err != nil {
		t.Skipf("font.ttf not available: %v", err)
	}
	if err := render.SetFont(ttf); err != nil {
		t.Fatalf("SetFont: %v", err)
	}
	if err := os.MkdirAll("icons", 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join("icons", "MaterialSymbols.ttf"), ttf, 0644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll("icons") })
}

const forecastFixture = `{
	"current": {"time": "2026-09-16T15:25", "temperature_2m": 34.4, "apparent_temperature": 34.6,
		"relative_humidity_2m": 30, "wind_speed_10m": 2.1, "wind_gusts_10m": 5.0,
		"surface_pressure": 1013.2, "weather_code": 3},
	"daily": {
		"time": ["2026-09-16", "2026-09-17", "2026-09-18", "2026-09-19", "2026-09-20"],
		"temperature_2m_max": [36.2, 38.7, 33.7, 25.6, 27.2],
		"temperature_2m_min": [17.9, 24.1, 22.0, 18.3, 16.9],
		"wind_speed_10m_max": [4.4, 3.9, 5.0, 3.6, 2.8],
		"weather_code": [3, 3, 95, 95, 2],
		"precipitation_probability_max": [7, 1, 1, 29, 10],
		"sunrise": ["2026-09-16T06:31", "2026-09-17T06:32", "2026-09-18T06:34", "2026-09-19T06:35", "2026-09-20T06:37"],
		"sunset": ["2026-09-16T18:59", "2026-09-17T18:57", "2026-09-18T18:55", "2026-09-19T18:52", "2026-09-20T18:50"],
		"uv_index_max": [7.2, 6.9, 5.1, 3.0, 4.4]
	}
}`

// darkPixels counts pixels darker than mid-grey inside the rectangle.
func darkPixels(t *testing.T, path string, x0, y0, x1, y1 int) int {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open output: %v", err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if b := img.Bounds(); b.Dx() != 800 || b.Dy() != 480 {
		t.Fatalf("bounds = %v, want 800x480", b)
	}
	dark := 0
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			if r, _, _, _ := img.At(x, y).RGBA(); r < 0x8000 {
				dark++
			}
		}
	}
	return dark
}

func TestRender_WritesFullScreenPNG(t *testing.T) {
	loadFonts(t)
	geo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"results": [{"name": "Kyiv", "latitude": 50.45, "longitude": 30.52, "country": "UA"}]}`))
	}))
	defer geo.Close()
	withGeocodingURL(t, geo)
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(forecastFixture))
	}))
	defer api.Close()
	withForecastURL(t, api)

	p, err := New("kyiv", "", "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	out := filepath.Join(t.TempDir(), "weather.png")
	if err := p.Render("weather", out, 4.0); err != nil {
		t.Fatalf("Render: %v", err)
	}

	regions := map[string][4]int{
		"header":         {0, 0, 400, 66},
		"now panel":      {0, 80, 400, 440},
		"forecast table": {420, 80, 800, 440},
		"footer":         {600, 445, 800, 480},
	}
	for name, r := range regions {
		if darkPixels(t, out, r[0], r[1], r[2], r[3]) == 0 {
			t.Errorf("%s region is blank", name)
		}
	}
	// The divider between the panels is a dotted column, dots every 3 px.
	if darkPixels(t, out, splitX, 80, splitX+1, 81) != 1 || darkPixels(t, out, splitX, 81, splitX+1, 82) != 0 {
		t.Error("panel divider not drawn as a dotted column at splitX")
	}
}

func TestRenderScreen_DrawsOneRowPerForecastDay(t *testing.T) {
	loadFonts(t)
	v, err := buildView(locationResult{Name: "Kyiv"}, fiveDayResponse(), "ms")
	if err != nil {
		t.Fatalf("buildView: %v", err)
	}
	v.Days = v.Days[:2]
	out := filepath.Join(t.TempDir(), "weather.png")
	if err := renderScreen(v, out, 4.0); err != nil {
		t.Fatalf("renderScreen: %v", err)
	}
	// Two rows drawn: something in row 2, nothing in row 3.
	if darkPixels(t, out, 420, tableTop+rowH, 800, tableTop+2*rowH) == 0 {
		t.Error("second forecast row is blank")
	}
	if darkPixels(t, out, 420, tableTop+2*rowH+1, 800, tableTop+3*rowH) != 0 {
		t.Error("third forecast row should be blank when only two days are given")
	}
}

func TestHeaderDate_ShortensWhenCityIsLong(t *testing.T) {
	loadFonts(t)
	day := time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC)

	got, err := headerDate("Kyiv", day)
	if err != nil {
		t.Fatalf("headerDate: %v", err)
	}
	if got != "Tuesday, 15 September" {
		t.Errorf("headerDate(Kyiv) = %q, want the full date", got)
	}

	got, err = headerDate(strings.Repeat("A", 20), day)
	if err != nil {
		t.Fatalf("headerDate: %v", err)
	}
	if got != "Tue 15 Sep" {
		t.Errorf("headerDate(20-char city) = %q, want the short date", got)
	}

	got, err = headerDate(strings.Repeat("A", 30), day)
	if err != nil {
		t.Fatalf("headerDate: %v", err)
	}
	if got != "" {
		t.Errorf("headerDate(30-char city) = %q, want no date", got)
	}
}

// fiveDayResponse is a forecast as Open-Meteo returns it with forecast_days=5:
// today first, then four more days. Values are distinct per day so index
// mistakes show up.
func fiveDayResponse() weatherResponse {
	return weatherResponse{
		Current: weatherData{
			Time:                "2026-09-16T15:25",
			Temperature2m:       34.4,
			ApparentTemperature: 34.6,
			RelativeHumidity2m:  30,
			WindSpeed10m:        2.1,
			WindGusts10m:        5.0,
			SurfacePressure:     1013.2,
			WeatherCode:         3,
		},
		Daily: daily{
			Time:          []string{"2026-09-16", "2026-09-17", "2026-09-18", "2026-09-19", "2026-09-20"},
			TMax:          []float64{36.2, 38.7, 33.7, 25.6, 27.2},
			TMin:          []float64{17.9, 24.1, 22.0, 18.3, 16.9},
			Wind:          []float64{4.4, 3.9, 5.0, 3.6, 2.8},
			WeatherCode:   []int{3, 3, 95, 95, 2},
			Precipitation: []int{7, 1, 1, 29, 10},
			Sunrise:       []string{"2026-09-16T06:31", "2026-09-17T06:32", "2026-09-18T06:34", "2026-09-19T06:35", "2026-09-20T06:37"},
			Sunset:        []string{"2026-09-16T18:59", "2026-09-17T18:57", "2026-09-18T18:55", "2026-09-19T18:52", "2026-09-20T18:50"},
			UVMax:         []float64{7.2, 6.9, 5.1, 3.0, 4.4},
		},
	}
}

func TestBuildView_TodayComesFromCurrentAndFirstDailyEntry(t *testing.T) {
	v, err := buildView(locationResult{Name: "Kyiv"}, fiveDayResponse(), "ms")
	if err != nil {
		t.Fatalf("buildView: %v", err)
	}
	if v.City != "Kyiv" {
		t.Errorf("City = %q, want Kyiv", v.City)
	}
	if v.Temp != 34.4 || v.Feels != 34.6 || v.Hi != 36.2 || v.Lo != 17.9 {
		t.Errorf("temps = %v %v %v %v, want 34.4 34.6 36.2 17.9", v.Temp, v.Feels, v.Hi, v.Lo)
	}
	if v.Humidity != 30 || v.Wind != 2.1 || v.Gust != 5.0 || v.Pressure != 1013.2 {
		t.Errorf("stats = %v %v %v %v", v.Humidity, v.Wind, v.Gust, v.Pressure)
	}
	if v.Rain != 7 || v.UV != 7.2 || v.Code != 3 {
		t.Errorf("rain/uv/code = %v %v %v, want 7 7.2 3", v.Rain, v.UV, v.Code)
	}
	if v.WindUnit != "m/s" {
		t.Errorf("WindUnit = %q, want m/s", v.WindUnit)
	}
}

func TestBuildView_ForecastStartsTomorrow(t *testing.T) {
	v, err := buildView(locationResult{Name: "Kyiv"}, fiveDayResponse(), "ms")
	if err != nil {
		t.Fatalf("buildView: %v", err)
	}
	if len(v.Days) != 4 {
		t.Fatalf("len(Days) = %d, want 4", len(v.Days))
	}
	first := v.Days[0]
	if first.Date.Format("2006-01-02") != "2026-09-17" {
		t.Errorf("Days[0].Date = %v, want 2026-09-17", first.Date)
	}
	if first.Hi != 38.7 || first.Lo != 24.1 || first.Wind != 3.9 || first.Code != 3 {
		t.Errorf("Days[0] = %+v, want tomorrow's values", first)
	}
	// The old screen read precipitation with the wrong index; tomorrow is 1%.
	if first.Rain != 1 {
		t.Errorf("Days[0].Rain = %d, want 1", first.Rain)
	}
	if last := v.Days[3]; last.Rain != 10 || last.Hi != 27.2 {
		t.Errorf("Days[3] = %+v, want the fifth day's values", last)
	}
}

func TestBuildView_ParsesLocalTimes(t *testing.T) {
	v, err := buildView(locationResult{Name: "Kyiv"}, fiveDayResponse(), "ms")
	if err != nil {
		t.Fatalf("buildView: %v", err)
	}
	if got := v.Updated.Format("2006-01-02 15:04"); got != "2026-09-16 15:25" {
		t.Errorf("Updated = %q, want 2026-09-16 15:25", got)
	}
	if got := v.Sunrise.Format("15:04"); got != "06:31" {
		t.Errorf("Sunrise = %q, want 06:31", got)
	}
	if got := v.Sunset.Format("15:04"); got != "18:59" {
		t.Errorf("Sunset = %q, want 18:59", got)
	}
}

func TestBuildView_WindLabelFollowsConfiguredUnit(t *testing.T) {
	for unit, label := range map[string]string{"ms": "m/s", "kmh": "km/h", "mph": "mph", "kn": "kn"} {
		v, err := buildView(locationResult{Name: "Kyiv"}, fiveDayResponse(), unit)
		if err != nil {
			t.Fatalf("buildView(%s): %v", unit, err)
		}
		if v.WindUnit != label {
			t.Errorf("WindUnit for %s = %q, want %q", unit, v.WindUnit, label)
		}
	}
}

func TestBuildView_ShortForecastYieldsFewerDays(t *testing.T) {
	w := fiveDayResponse()
	w.Daily.Time = w.Daily.Time[:2]
	w.Daily.TMax = w.Daily.TMax[:2]
	w.Daily.TMin = w.Daily.TMin[:2]
	w.Daily.Wind = w.Daily.Wind[:2]
	w.Daily.WeatherCode = w.Daily.WeatherCode[:2]
	w.Daily.Precipitation = w.Daily.Precipitation[:2]
	w.Daily.Sunrise = w.Daily.Sunrise[:2]
	w.Daily.Sunset = w.Daily.Sunset[:2]
	w.Daily.UVMax = w.Daily.UVMax[:2]

	v, err := buildView(locationResult{Name: "Kyiv"}, w, "ms")
	if err != nil {
		t.Fatalf("buildView: %v", err)
	}
	if len(v.Days) != 1 {
		t.Errorf("len(Days) = %d, want 1", len(v.Days))
	}
}

func TestBuildView_EmptyForecastIsAnError(t *testing.T) {
	if _, err := buildView(locationResult{Name: "Kyiv"}, weatherResponse{}, "ms"); err == nil {
		t.Fatal("expected error for a response with no daily data")
	}
}

func TestConditionText(t *testing.T) {
	tests := []struct {
		code int
		want string
	}{
		{0, "Clear"},
		{1, "Mainly clear"},
		{2, "Partly cloudy"},
		{3, "Overcast"},
		{45, "Fog"},
		{48, "Fog"},
		{51, "Drizzle"},
		{55, "Drizzle"},
		{56, "Freezing drizzle"},
		{61, "Light rain"},
		{63, "Rain"},
		{65, "Heavy rain"},
		{66, "Freezing rain"},
		{71, "Light snow"},
		{73, "Snow"},
		{75, "Heavy snow"},
		{77, "Snow grains"},
		{80, "Light showers"},
		{81, "Showers"},
		{82, "Heavy showers"},
		{85, "Snow showers"},
		{86, "Snow showers"},
		{95, "Thunderstorm"},
		{96, "Thunderstorm, hail"},
		{99, "Thunderstorm, hail"},
		{4, ""},
	}
	for _, tc := range tests {
		if got := conditionText(tc.code); got != tc.want {
			t.Errorf("conditionText(%d) = %q, want %q", tc.code, got, tc.want)
		}
	}
}

func TestFormatDegrees(t *testing.T) {
	tests := []struct {
		in   float64
		want string
	}{
		{34.4, "34°"},
		{34.5, "35°"},
		{104, "104°"},
		{-12.3, "-12°"},
		{-0.4, "0°"},
		{0, "0°"},
	}
	for _, tc := range tests {
		if got := formatDegrees(tc.in); got != tc.want {
			t.Errorf("formatDegrees(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestDayLabel(t *testing.T) {
	d := time.Date(2026, 9, 17, 0, 0, 0, 0, time.UTC)
	if got := dayLabel(d); got != "Thu" {
		t.Errorf("dayLabel = %q, want Thu", got)
	}
}
