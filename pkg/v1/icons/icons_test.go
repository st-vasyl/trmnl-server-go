package icons

import (
	"fmt"
	"image"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// readTestTTF loads the repo's font.ttf (a real TTF) to stand in for the
// downloaded font in tests. From this package's dir it is three levels up.
func readTestTTF(t *testing.T) []byte {
	t.Helper()
	ttf, err := os.ReadFile("../../../font.ttf")
	if err != nil {
		t.Skipf("font.ttf not available: %v", err)
	}
	return ttf
}

// withFontOverrides points font acquisition at a temp cache dir and clears the
// in-memory parsed font, restoring originals on cleanup.
func withFontOverrides(t *testing.T) string {
	t.Helper()
	origURL, origDir, origFont := cssURL, cacheDir, parsedFont
	dir := t.TempDir()
	cssURL, cacheDir, parsedFont = "", dir, nil
	t.Cleanup(func() { cssURL, cacheDir, parsedFont = origURL, origDir, origFont })
	return dir
}

func TestGetFont_DownloadsDecodesAndCaches(t *testing.T) {
	ttf := readTestTTF(t)
	mux := http.NewServeMux()
	mux.HandleFunc("/css", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "@font-face{src: url(http://%s/font) format('woff2');}", r.Host)
	})
	mux.HandleFunc("/font", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(ttf)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	dir := withFontOverrides(t)
	cssURL = srv.URL + "/css"

	f, err := getFont()
	if err != nil {
		t.Fatalf("getFont: %v", err)
	}
	if f == nil {
		t.Fatal("getFont returned nil font")
	}
	if _, err := os.Stat(filepath.Join(dir, "MaterialSymbols.ttf")); err != nil {
		t.Errorf("font not cached: %v", err)
	}
}

func TestGetFont_CacheHitSkipsNetwork(t *testing.T) {
	ttf := readTestTTF(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("network hit despite cache: %s", r.URL.Path)
	}))
	defer srv.Close()

	dir := withFontOverrides(t)
	cssURL = srv.URL
	if err := os.WriteFile(filepath.Join(dir, "MaterialSymbols.ttf"), ttf, 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := getFont(); err != nil {
		t.Fatalf("getFont: %v", err)
	}
}

func TestGetFont_NoFontURLReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("/* css with no url */"))
	}))
	defer srv.Close()

	withFontOverrides(t)
	cssURL = srv.URL

	if _, err := getFont(); err == nil {
		t.Fatal("expected error when CSS has no font url")
	}
}

func TestRender_UnknownIconReturnsError(t *testing.T) {
	if _, err := Render("definitely-not-an-icon", 24); err == nil {
		t.Fatal("expected error for unknown icon name")
	}
}

func TestRender_DrawsGlyphPixels(t *testing.T) {
	ttf := readTestTTF(t)
	dir := withFontOverrides(t)
	if err := os.WriteFile(filepath.Join(dir, "MaterialSymbols.ttf"), ttf, 0644); err != nil {
		t.Fatal(err)
	}
	// Map a temporary name to a glyph that exists in font.ttf ('A').
	codepoints["__test_glyph__"] = 'A'
	t.Cleanup(func() { delete(codepoints, "__test_glyph__") })

	img, err := Render("__test_glyph__", 48)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	b := img.Bounds()
	if b.Dx() != 48 || b.Dy() != 48 {
		t.Fatalf("bounds = %v, want 48x48", b)
	}
	rgba, ok := img.(*image.RGBA)
	if !ok {
		t.Fatalf("image type = %T, want *image.RGBA", img)
	}
	drawn := false
	for i := 3; i < len(rgba.Pix) && !drawn; i += 4 {
		if rgba.Pix[i] != 0 { // any non-transparent pixel
			drawn = true
		}
	}
	if !drawn {
		t.Error("Render produced a fully transparent image")
	}
}

