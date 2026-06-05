package render

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"trmnl-server-go/pkg/v1/icons"
)

// TestMain bootstraps a font for tests that exercise text rendering.
// The repository ships ./font.ttf at its root; from this package's working
// directory that's three levels up.
func TestMain(m *testing.M) {
	ttf, err := os.ReadFile("../../../font.ttf")
	if err != nil {
		// Fall back to skipping font setup; tests that require a font will
		// be the only ones to fail. We don't want to mask other failures.
		os.Stderr.WriteString("render tests: could not read ../../../font.ttf: " + err.Error() + "\n")
	} else if err := SetFont(ttf); err != nil {
		os.Stderr.WriteString("render tests: SetFont failed: " + err.Error() + "\n")
	}
	os.Exit(m.Run())
}

// withoutFont temporarily clears the cached font for tests that exercise the
// "font not initialized" error path, then restores it.
func withoutFont(t *testing.T) {
	t.Helper()
	orig := cachedFont
	cachedFont = nil
	t.Cleanup(func() { cachedFont = orig })
}

func TestSetFont_InvalidBytesReturnsError(t *testing.T) {
	// Save current font so a failed parse can't pollute later tests.
	orig := cachedFont
	t.Cleanup(func() { cachedFont = orig })

	if err := SetFont([]byte("not a real ttf")); err == nil {
		t.Fatal("expected error for invalid TTF bytes")
	}
	if cachedFont != orig {
		t.Error("cachedFont was mutated even though SetFont failed")
	}
}

func TestGetFont_ErrorBeforeSetFont(t *testing.T) {
	withoutFont(t)
	if _, err := getFont(); err == nil {
		t.Fatal("expected error when cachedFont is nil")
	}
}

func TestNewImage_DimensionsAndWhiteBackground(t *testing.T) {
	img := NewImage(120, 80)
	b := img.Bounds()
	if b.Dx() != 120 || b.Dy() != 80 {
		t.Fatalf("bounds = %v, want 120x80", b)
	}

	// Sample a few corners and the centre; all should be opaque white.
	white := color.RGBA{255, 255, 255, 255}
	for _, p := range []image.Point{{0, 0}, {119, 79}, {60, 40}} {
		got := img.RGBAAt(p.X, p.Y)
		if got != white {
			t.Errorf("pixel at %v = %v, want %v", p, got, white)
		}
	}
}

func TestAddText_ErrorWhenFontUnset(t *testing.T) {
	withoutFont(t)
	img := NewImage(100, 50)
	if err := AddText(img, "hi", image.Point{10, 30}, color.Black, 16); err == nil {
		t.Fatal("expected error when font is uninitialised")
	}
}

func TestAddText_WritesPixelsWhenFontSet(t *testing.T) {
	if cachedFont == nil {
		t.Skip("font not loaded in TestMain; skipping")
	}
	img := NewImage(200, 60)

	// Sanity: image starts all-white.
	if got := img.RGBAAt(10, 30); got.R != 255 || got.G != 255 || got.B != 255 {
		t.Fatalf("starting pixel not white: %v", got)
	}

	if err := AddText(img, "hello", image.Point{10, 40}, color.Black, 24); err != nil {
		t.Fatalf("AddText: %v", err)
	}

	// At least one pixel inside the text area must have darkened.
	darkened := false
	for y := 0; y < 60 && !darkened; y++ {
		for x := 0; x < 200; x++ {
			c := img.RGBAAt(x, y)
			if c.R < 200 && c.G < 200 && c.B < 200 {
				darkened = true
				break
			}
		}
	}
	if !darkened {
		t.Error("AddText did not modify any pixels")
	}
}

func TestConvertToGray_ReturnsGrayWithSameBounds(t *testing.T) {
	img := NewImage(50, 30)
	// Stamp a black pixel so we can assert intensity dropped.
	img.Set(25, 15, color.Black)

	gray := ConvertToGray(img)
	if gray.Bounds() != img.Bounds() {
		t.Errorf("gray bounds = %v, want %v", gray.Bounds(), img.Bounds())
	}
	if got := gray.GrayAt(25, 15).Y; got != 0 {
		t.Errorf("centre pixel Y = %d, want 0 (black)", got)
	}
	if got := gray.GrayAt(0, 0).Y; got != 255 {
		t.Errorf("corner pixel Y = %d, want 255 (white)", got)
	}
}

// smallPNG returns bytes of a tiny valid PNG for seeding the icon cache.
func smallPNG(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, NewImage(8, 8)); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

// seedIconCache writes valid PNGs into ./icons so icons.Load resolves from the
// on-disk cache without any network access. Removed on cleanup.
func seedIconCache(t *testing.T, names ...string) {
	t.Helper()
	if err := os.MkdirAll("icons", 0755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll("icons") })
	data := smallPNG(t)
	for _, n := range names {
		if err := os.WriteFile(filepath.Join("icons", n+".png"), data, 0644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestAddImageFromBytes_ValidPNGDecodes(t *testing.T) {
	img := NewImage(800, 480)
	if err := AddImageFromBytes(img, smallPNG(t), image.Point{0, 0}); err != nil {
		t.Fatalf("AddImageFromBytes: %v", err)
	}
}

func TestAddImageFromBytes_InvalidReturnsError(t *testing.T) {
	img := NewImage(100, 100)
	if err := AddImageFromBytes(img, []byte("not a png"), image.Point{0, 0}); err == nil {
		t.Fatal("expected error for non-PNG bytes")
	}
}

func TestAddIcon_DrawsCachedIcon(t *testing.T) {
	seedIconCache(t, icons.Wind)
	img := NewImage(800, 480)
	if err := AddIcon(img, icons.Wind, image.Point{0, 0}); err != nil {
		t.Fatalf("AddIcon: %v", err)
	}
}

func TestAddImageVoltage_DispatchesByThreshold(t *testing.T) {
	seedIconCache(t, icons.Battery0, icons.Battery20, icons.Battery40,
		icons.Battery60, icons.Battery80, icons.Battery100)
	// Each branch selects a battery icon name and draws it; calling
	// AddImageVoltage at representative voltages must succeed.
	voltages := []float32{
		4.20, // > 90% → Battery100
		4.00, // 70 < % ≤ 90 → Battery80
		3.80, // 50 < % ≤ 70 → Battery60
		3.55, // 30 < % ≤ 50 → Battery40
		3.20, // 10 < % ≤ 30 → Battery20
		3.00, // ≤ 10 → Battery0
		2.50, // very low → Battery0
	}
	for _, v := range voltages {
		img := NewImage(800, 480)
		if err := AddImageVoltage(img, v, image.Point{-750, -5}); err != nil {
			t.Errorf("voltage %v: %v", v, err)
		}
	}
}

func TestGenPoints_MapsRecordsToXYs(t *testing.T) {
	records := ChartRecords{
		ChartRecord: []ChartRecord{
			{T: 1_700_000_000_000, V: 1.5},
			{T: 1_700_003_600_000, V: 2.5},
		},
	}
	pts := genPoints(records)
	if len(pts) != 2 {
		t.Fatalf("len = %d, want 2", len(pts))
	}
	// T is in milliseconds; X is seconds.
	if pts[0].X != 1_700_000_000 {
		t.Errorf("pts[0].X = %v, want 1700000000", pts[0].X)
	}
	if pts[0].Y != 1.5 || pts[1].Y != 2.5 {
		t.Errorf("Y values = %v, %v; want 1.5, 2.5", pts[0].Y, pts[1].Y)
	}
}

func TestSparseTicks_FiltersByRange(t *testing.T) {
	st := sparseTicks{labels: map[float64]string{
		0:  "a",
		5:  "b",
		10: "c",
	}}
	ticks := st.Ticks(0, 7)
	if len(ticks) != 2 {
		t.Fatalf("len = %d, want 2 (labels at 0 and 5)", len(ticks))
	}
	got := map[float64]string{}
	for _, tk := range ticks {
		got[tk.Value] = tk.Label
	}
	if got[0] != "a" || got[5] != "b" {
		t.Errorf("ticks = %v, want {0:a, 5:b}", got)
	}

	if out := st.Ticks(100, 200); len(out) != 0 {
		t.Errorf("expected no ticks outside range, got %v", out)
	}
}

func TestAddChart_DrawsWithoutError(t *testing.T) {
	img := NewImage(800, 480)
	records := ChartRecords{
		ChartRecord: []ChartRecord{
			{T: 1_700_000_000_000, V: 100.0},
			{T: 1_700_003_600_000, V: 110.0},
			{T: 1_700_007_200_000, V: 105.0},
		},
	}
	if err := AddChart(img, records, 400, 200, image.Point{0, 0}); err != nil {
		t.Fatalf("AddChart: %v", err)
	}
}

func TestAddStocksChart_DrawsWithoutError(t *testing.T) {
	img := NewImage(800, 480)
	records := BoxPlotRecords{
		BoxPlotRecord: []BoxPlotRecord{
			{T: 0, Vmin: 100, Vmax: 105},
			{T: 1, Vmin: 102, Vmax: 108},
			{T: 2, Vmin: 101, Vmax: 107},
		},
		XLabels: map[float64]string{
			0: "Mon",
			2: "Wed",
		},
	}
	if err := AddStocksChart(img, records, 400, 200, image.Point{0, 0}); err != nil {
		t.Fatalf("AddStocksChart: %v", err)
	}
}

func TestAddWeatherChart_DrawsWithoutError(t *testing.T) {
	img := NewImage(800, 480)
	mins := ChartRecords{ChartRecord: []ChartRecord{
		{T: 1_700_000_000_000, V: 5},
		{T: 1_700_086_400_000, V: 6},
	}}
	maxes := ChartRecords{ChartRecord: []ChartRecord{
		{T: 1_700_000_000_000, V: 12},
		{T: 1_700_086_400_000, V: 14},
	}}
	if err := AddWeatherChart(img, mins, maxes, 400, 200, image.Point{0, 0}); err != nil {
		t.Fatalf("AddWeatherChart: %v", err)
	}
}

func TestWriteFile_ProducesDecodablePNG(t *testing.T) {
	img := NewImage(100, 60)
	path := filepath.Join(t.TempDir(), "out.png")

	// Use a voltage that hits a valid branch in AddImageVoltage (drawn outside
	// the visible canvas via the same negative offset main.go uses).
	if err := WriteFile(path, img, 4.0); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open output: %v", err)
	}
	defer f.Close()
	decoded, err := png.Decode(f)
	if err != nil {
		t.Fatalf("decode output as PNG: %v", err)
	}
	if got := decoded.Bounds(); got.Dx() != 100 || got.Dy() != 60 {
		t.Errorf("decoded bounds = %v, want 100x60", got)
	}
	if _, ok := decoded.(*image.Gray); !ok {
		t.Errorf("decoded image type = %T, want *image.Gray", decoded)
	}
}

func TestWriteFile_UnwritablePathReturnsError(t *testing.T) {
	img := NewImage(50, 50)
	if err := WriteFile("/this/dir/does/not/exist/out.png", img, 4.0); err == nil {
		t.Fatal("expected error for unwritable path")
	}
}

func TestGetImageByUrl_FetchesAndDecodesJPEG(t *testing.T) {
	// Build a small in-memory JPEG to serve.
	src := image.NewRGBA(image.Rect(0, 0, 20, 20))
	for y := 0; y < 20; y++ {
		for x := 0; x < 20; x++ {
			src.Set(x, y, color.RGBA{R: 200, G: 100, B: 50, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, src, &jpeg.Options{Quality: 80}); err != nil {
		t.Fatalf("jpeg.Encode: %v", err)
	}
	body := buf.Bytes()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Write(body)
	}))
	defer srv.Close()

	got, err := GetImageByUrl(srv.URL)
	if err != nil {
		t.Fatalf("GetImageByUrl: %v", err)
	}
	if b := got.Bounds(); b.Dx() != 20 || b.Dy() != 20 {
		t.Errorf("bounds = %v, want 20x20", b)
	}
}

func TestGetImageByUrl_DecodeErrorOnNonImage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("definitely not a jpeg"))
	}))
	defer srv.Close()

	if _, err := GetImageByUrl(srv.URL); err == nil {
		t.Fatal("expected JPEG decode error")
	}
}

func TestGetImageByUrl_NetworkErrorPropagates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close()

	if _, err := GetImageByUrl(url); err == nil {
		t.Fatal("expected network error after server close")
	}
}
