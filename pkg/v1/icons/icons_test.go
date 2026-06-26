package icons

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// samplePNG returns the bytes of a small valid PNG.
func samplePNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode sample png: %v", err)
	}
	return buf.Bytes()
}

// withOverrides points the package at a temp cache dir and the given base URL,
// restoring the originals on cleanup.
func withOverrides(t *testing.T, baseURL string) string {
	t.Helper()
	origURL, origDir := rawBaseURL, cacheDir
	dir := t.TempDir()
	rawBaseURL, cacheDir = baseURL, dir
	t.Cleanup(func() { rawBaseURL, cacheDir = origURL, origDir })
	return dir
}

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

func TestLoad_CacheHitSkipsNetwork(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("network hit for cached icon: %s", r.URL.Path)
	}))
	defer srv.Close()

	dir := withOverrides(t, srv.URL+"/")
	want := samplePNG(t)
	if err := os.WriteFile(filepath.Join(dir, "wind.png"), want, 0644); err != nil {
		t.Fatal(err)
	}

	got, err := Load("wind")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Error("Load did not return cached bytes")
	}
}

func TestLoad_DownloadsAndCaches(t *testing.T) {
	want := samplePNG(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/wind.png" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		_, _ = w.Write(want)
	}))
	defer srv.Close()

	dir := withOverrides(t, srv.URL+"/")

	got, err := Load("wind")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Error("Load returned wrong bytes")
	}

	cached, err := os.ReadFile(filepath.Join(dir, "wind.png"))
	if err != nil {
		t.Fatalf("expected cached file: %v", err)
	}
	if !bytes.Equal(cached, want) {
		t.Error("cached file has wrong bytes")
	}
}

func TestLoad_NonPNGResponseReturnsErrorAndDoesNotCache(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("404 page not found"))
	}))
	defer srv.Close()

	dir := withOverrides(t, srv.URL+"/")

	if _, err := Load("wind"); err == nil {
		t.Fatal("expected error for non-PNG response")
	}
	if _, err := os.Stat(filepath.Join(dir, "wind.png")); !os.IsNotExist(err) {
		t.Error("non-PNG response should not be cached")
	}
}

func TestLoad_NetworkErrorReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close() // port no longer accepts connections

	withOverrides(t, url+"/")

	if _, err := Load("wind"); err == nil {
		t.Fatal("expected network error")
	}
}
