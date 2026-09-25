package custom

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// seedIconFont copies the repo-root font.ttf into the icon cache so the
// battery stamp in render.WriteFile never hits the network. Skips when the
// font is missing.
func seedIconFont(t *testing.T) {
	t.Helper()
	ttf, err := os.ReadFile("../../../../font.ttf")
	if err != nil {
		t.Skipf("font.ttf not available: %v", err)
	}
	if err := os.MkdirAll("icons", 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join("icons", "MaterialSymbols.ttf"), ttf, 0644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll("icons") })
}

// solidPNG encodes a w×h image filled with c.
func solidPNG(t *testing.T, w, h int, c color.Color) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), &image.Uniform{c}, image.Point{}, draw.Src)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// serveBytes starts a server that answers every request with body and counts
// the requests it received.
func serveBytes(t *testing.T, status int, body []byte) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(status)
		w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

// decodeOutput opens the PNG Render wrote.
func decodeOutput(t *testing.T, path string) image.Image {
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
	return img
}

func TestRender_WritesFullScreenPNG(t *testing.T) {
	seedIconFont(t)
	srv, _ := serveBytes(t, http.StatusOK, solidPNG(t, 800, 480, color.Black))

	p, err := New([]string{srv.URL + "/a.png"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	out := filepath.Join(t.TempDir(), "custom_1.png")
	if err := p.Render("custom_1", out, 4.0); err != nil {
		t.Fatalf("Render: %v", err)
	}

	img := decodeOutput(t, out)
	if img.Bounds().Dx() != 800 || img.Bounds().Dy() != 480 {
		t.Errorf("output is %v, want 800x480", img.Bounds())
	}
	if _, ok := img.(*image.Gray); !ok {
		t.Errorf("output should be grayscale like every other screen, got %T", img)
	}
}

// withCacheTTL overrides how long a downloaded image is reused.
func withCacheTTL(t *testing.T, d time.Duration) {
	t.Helper()
	old := cacheTTL
	cacheTTL = d
	t.Cleanup(func() { cacheTTL = old })
}

func TestRender_ReusesDownloadWithinTTL(t *testing.T) {
	seedIconFont(t)
	withCacheTTL(t, time.Hour)
	srv, hits := serveBytes(t, http.StatusOK, solidPNG(t, 800, 480, color.Black))

	p, err := New([]string{srv.URL + "/a.png"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	dir := t.TempDir()
	for _, name := range []string{"dev1_custom_1.png", "dev2_custom_1.png", "dev3_custom_1.png"} {
		if err := p.Render("custom_1", filepath.Join(dir, name), 4.0); err != nil {
			t.Fatalf("Render %s: %v", name, err)
		}
	}
	if got := hits.Load(); got != 1 {
		t.Errorf("server hit %d times, want 1 (one download shared by every device)", got)
	}
}

func TestRender_RefetchesAfterTTL(t *testing.T) {
	seedIconFont(t)
	withCacheTTL(t, 0)
	srv, hits := serveBytes(t, http.StatusOK, solidPNG(t, 800, 480, color.Black))

	p, err := New([]string{srv.URL + "/a.png"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	dir := t.TempDir()
	for _, name := range []string{"first.png", "second.png"} {
		if err := p.Render("custom_1", filepath.Join(dir, name), 4.0); err != nil {
			t.Fatalf("Render %s: %v", name, err)
		}
	}
	if got := hits.Load(); got != 2 {
		t.Errorf("server hit %d times, want 2 (expired entry is downloaded again)", got)
	}
}

func TestRender_EachScreenFetchesItsOwnURL(t *testing.T) {
	seedIconFont(t)
	withCacheTTL(t, time.Hour)
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.Write(solidPNG(t, 800, 480, color.Black))
	}))
	t.Cleanup(srv.Close)

	p, err := New([]string{srv.URL + "/a.png", srv.URL + "/b.png"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	dir := t.TempDir()
	if err := p.Render("custom_2", filepath.Join(dir, "custom_2.png"), 4.0); err != nil {
		t.Fatalf("Render custom_2: %v", err)
	}
	if err := p.Render("custom_1", filepath.Join(dir, "custom_1.png"), 4.0); err != nil {
		t.Fatalf("Render custom_1: %v", err)
	}
	if got := strings.Join(paths, ","); got != "/b.png,/a.png" {
		t.Errorf("requested paths = %v, want [/b.png /a.png]", paths)
	}
}

func TestRender_Non200IsAnErrorAndWritesNothing(t *testing.T) {
	seedIconFont(t)
	srv, _ := serveBytes(t, http.StatusInternalServerError, []byte("boom"))

	p, err := New([]string{srv.URL + "/a.png"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	out := filepath.Join(t.TempDir(), "custom_1.png")
	err = p.Render("custom_1", out, 4.0)
	if err == nil {
		t.Fatal("expected error for a 500 response")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error should mention the status, got %q", err)
	}
	if _, statErr := os.Stat(out); statErr == nil {
		t.Error("no PNG should be written when the download fails")
	}
}

func TestRender_NonImageBodyIsAnErrorAndWritesNothing(t *testing.T) {
	seedIconFont(t)
	srv, _ := serveBytes(t, http.StatusOK, []byte("<html>not an image</html>"))

	p, err := New([]string{srv.URL + "/a.png"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	out := filepath.Join(t.TempDir(), "custom_1.png")
	if err := p.Render("custom_1", out, 4.0); err == nil {
		t.Fatal("expected error for a body that is not an image")
	}
	if _, statErr := os.Stat(out); statErr == nil {
		t.Error("no PNG should be written when the body does not decode")
	}
}

func TestRender_FailedDownloadIsNotCached(t *testing.T) {
	seedIconFont(t)
	withCacheTTL(t, time.Hour)
	var fail atomic.Bool
	fail.Store(true)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fail.Load() {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.Write(solidPNG(t, 800, 480, color.Black))
	}))
	t.Cleanup(srv.Close)

	p, err := New([]string{srv.URL + "/a.png"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	out := filepath.Join(t.TempDir(), "custom_1.png")
	if err := p.Render("custom_1", out, 4.0); err == nil {
		t.Fatal("expected error while the server fails")
	}
	fail.Store(false)
	if err := p.Render("custom_1", out, 4.0); err != nil {
		t.Fatalf("Render after the server recovered: %v", err)
	}
	if _, statErr := os.Stat(out); statErr != nil {
		t.Error("PNG should be written once the server recovers")
	}
}

func TestRender_RejectsOversizedBody(t *testing.T) {
	seedIconFont(t)
	body := solidPNG(t, 800, 480, color.Black)
	old := maxBodyBytes
	maxBodyBytes = int64(len(body)) - 1
	t.Cleanup(func() { maxBodyBytes = old })
	srv, _ := serveBytes(t, http.StatusOK, body)

	p, err := New([]string{srv.URL + "/a.png"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	out := filepath.Join(t.TempDir(), "custom_1.png")
	err = p.Render("custom_1", out, 4.0)
	if err == nil {
		t.Fatal("expected error for a body over the limit")
	}
	if !strings.Contains(err.Error(), "large") {
		t.Errorf("error should say the body is too large, got %q", err)
	}
}

func TestRender_SlowServerTimesOut(t *testing.T) {
	seedIconFont(t)
	old := client
	client = &http.Client{Timeout: 50 * time.Millisecond}
	t.Cleanup(func() { client = old })
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
		w.Write(solidPNG(t, 800, 480, color.Black))
	}))
	t.Cleanup(srv.Close)

	p, err := New([]string{srv.URL + "/a.png"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	out := filepath.Join(t.TempDir(), "custom_1.png")
	if err := p.Render("custom_1", out, 4.0); err == nil {
		t.Fatal("expected a timeout error")
	}
}

// quadrantPNG encodes a white w×h image whose bottom-right quarter is black.
func quadrantPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), image.White, image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(w/2, h/2, w, h), image.Black, image.Point{}, draw.Src)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// isDark reports whether the output pixel at x,y is closer to black than white.
func isDark(img image.Image, x, y int) bool {
	return color.GrayModel.Convert(img.At(x, y)).(color.Gray).Y < 128
}

func renderOne(t *testing.T, body []byte) image.Image {
	t.Helper()
	seedIconFont(t)
	srv, _ := serveBytes(t, http.StatusOK, body)
	p, err := New([]string{srv.URL + "/a.png"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	out := filepath.Join(t.TempDir(), "custom_1.png")
	if err := p.Render("custom_1", out, 4.0); err != nil {
		t.Fatalf("Render: %v", err)
	}
	return decodeOutput(t, out)
}

func TestRender_SmallerImageIsScaledUpToFill(t *testing.T) {
	img := renderOne(t, solidPNG(t, 400, 240, color.Black))
	// Same aspect ratio, so the whole canvas is covered. Check away from the
	// battery icon in the top-right corner.
	if !isDark(img, 10, 470) || !isDark(img, 790, 470) {
		t.Error("a 400x240 image should be scaled to cover the 800x480 canvas")
	}
}

func TestRender_SquareImageIsCenteredOnWhite(t *testing.T) {
	img := renderOne(t, solidPNG(t, 480, 480, color.Black))
	// Fits the height, so it occupies x in [160, 640) with white margins.
	if isDark(img, 10, 240) || isDark(img, 700, 240) {
		t.Error("margins beside a square image should be white")
	}
	if !isDark(img, 400, 240) || !isDark(img, 170, 240) || !isDark(img, 630, 240) {
		t.Error("the square image should be centered horizontally")
	}
}

func TestRender_LargerImageIsScaledDownNotCropped(t *testing.T) {
	img := renderOne(t, quadrantPNG(t, 1600, 960))
	// After scaling, the black bottom-right quarter lands at (400..800, 240..480).
	// Cropping would instead show only the white top-left of the source.
	if !isDark(img, 700, 470) {
		t.Error("bottom-right of the output should show the source's black quadrant")
	}
	if isDark(img, 100, 100) || isDark(img, 100, 470) || isDark(img, 700, 100) {
		t.Error("other quadrants should stay white")
	}
}

func TestRender_AcceptsJPEG(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 800, 480))
	draw.Draw(src, src.Bounds(), image.Black, image.Point{}, draw.Src)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, src, nil); err != nil {
		t.Fatal(err)
	}
	img := renderOne(t, buf.Bytes())
	if !isDark(img, 10, 470) {
		t.Error("JPEG input should render like PNG input")
	}
}

func TestRender_UnknownScreenIsAnError(t *testing.T) {
	p, err := New([]string{"https://example.com/a.png"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	out := filepath.Join(t.TempDir(), "x.png")
	for _, screen := range []string{"weather", "custom_", "custom_0", "custom_2", "custom_x"} {
		if err := p.Render(screen, out, 4.0); err == nil {
			t.Errorf("Render(%q): expected error", screen)
		}
	}
	if _, err := os.Stat(out); err == nil {
		t.Error("no PNG should be written for an unknown screen")
	}
}

func TestNew_RejectsEmptyList(t *testing.T) {
	if _, err := New(nil); err == nil {
		t.Fatal("expected error for no urls")
	}
}

func TestNew_RejectsNonHTTPURL(t *testing.T) {
	for _, u := range []string{"ftp://example.com/a.png", "example.com/a.png", "", "http://"} {
		if _, err := New([]string{u}); err == nil {
			t.Errorf("New(%q): expected error", u)
		}
	}
}

func TestNew_ErrorNamesTheOffendingEntry(t *testing.T) {
	_, err := New([]string{"https://ok.example/a.png", "ftp://bad.example/b.png"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "2") || !strings.Contains(err.Error(), "ftp://bad.example/b.png") {
		t.Errorf("error should name entry 2 and its url, got %q", err)
	}
}

func TestNameAndScreens(t *testing.T) {
	p, err := New([]string{"https://example.com/a.png", "http://example.com/b.jpg"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if p.Name() != "custom" {
		t.Errorf("Name = %q, want custom", p.Name())
	}
	if got := strings.Join(p.Screens(), ","); got != "custom_1,custom_2" {
		t.Errorf("Screens = %v, want [custom_1 custom_2]", p.Screens())
	}
}
