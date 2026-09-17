package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"trmnl-server-go/pkg/v1/config"
	"trmnl-server-go/pkg/v1/db"
	"trmnl-server-go/pkg/v1/plugin"
	"trmnl-server-go/pkg/v1/screens"
)

type fakePlugin struct {
	name    string
	screens []string
}

func (f *fakePlugin) Name() string                                      { return f.name }
func (f *fakePlugin) Screens() []string                                 { return f.screens }
func (f *fakePlugin) Render(screen, path string, voltage float32) error { return nil }

// writingPlugin really writes a file at outputPath so /public/ has something
// to serve in end-to-end tests.
type writingPlugin struct {
	name    string
	screens []string
}

func (p *writingPlugin) Name() string      { return p.name }
func (p *writingPlugin) Screens() []string { return p.screens }
func (p *writingPlugin) Render(screen, outputPath string, voltage float32) error {
	return os.WriteFile(outputPath, []byte("png"), 0644)
}

// setupInTempDir is like setup but runs the test from a temporary working
// directory that has a public/ folder, with a plugin that writes real files.
// ServeFiles resolves "/public/..." against the CWD, so this lets a test
// follow the device's image_url all the way to the served bytes.
func setupInTempDir(t *testing.T) (*http.ServeMux, *db.Store) {
	t.Helper()
	dir := t.TempDir()
	t.Chdir(dir)
	// Registered after Chdir so it runs before the CWD is restored: background
	// renders must finish while "public/" still points at the temp dir.
	t.Cleanup(screens.WaitForRenders)
	if err := os.MkdirAll("public", 0755); err != nil {
		t.Fatalf("mkdir public: %v", err)
	}
	store, err := db.Open(filepath.Join(dir, "trmnl.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	c := &config.Config{}
	c.Common.Port = 8080
	c.Common.ExternalURL = "host:8080"
	c.Common.RefreshTime = 300

	plugins := []plugin.Plugin{
		&writingPlugin{name: "p", screens: []string{"weather", "crypto"}},
	}
	return NewMux("test-version", c, plugins, store), store
}

func doGet(mux http.Handler, path string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func setup(t *testing.T) (*http.ServeMux, *db.Store) {
	t.Helper()
	dbpath := filepath.Join(t.TempDir(), "trmnl.db")
	store, err := db.Open(dbpath)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	c := &config.Config{}
	c.Common.Port = 8080
	c.Common.ExternalURL = "host:8080"
	c.Common.RefreshTime = 300

	plugins := []plugin.Plugin{
		&fakePlugin{name: "p", screens: []string{"weather", "crypto"}},
	}
	return NewMux("test-version", c, plugins, store), store
}

func TestHealthz(t *testing.T) {
	mux, _ := setup(t)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "test-version") {
		t.Errorf("body = %q, want it to contain 'test-version'", body)
	}
}

func TestSetup_RegistersDeviceAndReturnsSetupResponse(t *testing.T) {
	mux, store := setup(t)
	req := httptest.NewRequest(http.MethodGet, "/api/setup", nil)
	req.Header.Set("Access-Token", "key-1")
	req.Header.Set("Id", "dev-1")
	req.Header.Set("Battery-Voltage", "4.1")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var resp SetupResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if resp.ApiKey != "key-1" {
		t.Errorf("ApiKey = %q, want key-1", resp.ApiKey)
	}
	if resp.Status != 200 {
		t.Errorf("Status = %d, want 200", resp.Status)
	}

	// Device should be registered with the first screen ("weather").
	screen, err := store.GetDeviceScreen("dev-1")
	if err != nil {
		t.Fatalf("GetDeviceScreen: %v", err)
	}
	if screen != "weather" {
		t.Errorf("registered screen = %q, want weather", screen)
	}
}

func TestDisplay_RegistersNewDeviceAndAdvances(t *testing.T) {
	mux, store := setup(t)
	req := httptest.NewRequest(http.MethodGet, "/api/display", nil)
	req.Header.Set("Access-Token", "key-1")
	req.Header.Set("Id", "dev-1")
	req.Header.Set("Battery-Voltage", "4.0")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	// New device: registered with first screen, advanced to second.
	screen, err := store.GetDeviceScreen("dev-1")
	if err != nil {
		t.Fatalf("GetDeviceScreen: %v", err)
	}
	if screen != "crypto" {
		t.Errorf("screen after display = %q, want crypto", screen)
	}
}

// A device that registers between worker ticks must still get a real image
// on its very first /api/display: the firmware fetches image_url immediately
// and shows "image download failed" on a 404.
func TestDisplay_NewDeviceImageIsServedWithoutWorkerTick(t *testing.T) {
	mux, _ := setupInTempDir(t)
	h := map[string]string{"Access-Token": "key-1", "Id": "dev-1", "Battery-Voltage": "4.1"}

	rec := doGet(mux, "/api/display", h)
	if rec.Code != 200 {
		t.Fatalf("display status = %d, want 200", rec.Code)
	}
	var resp screens.DisplayResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	imgPath := strings.TrimPrefix(resp.ImageURL, "http://host:8080")
	if imgPath == resp.ImageURL {
		t.Fatalf("unexpected image_url %q", resp.ImageURL)
	}

	rec = doGet(mux, imgPath, h)
	if rec.Code != 200 {
		t.Fatalf("GET %s right after /api/display = %d, want 200 (no worker tick has run)", imgPath, rec.Code)
	}
}

// The real firmware sends no Access-Token on /api/setup; the server generates
// a key and the device stores whatever api_key the response carries.
func TestSetup_WithoutAccessTokenReturnsStoredApiKey(t *testing.T) {
	mux, store := setup(t)

	rec := doGet(mux, "/api/setup", map[string]string{"Id": "dev-1", "Battery-Voltage": "4.1"})
	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var resp SetupResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	keys, err := store.GetDeviceList()
	if err != nil {
		t.Fatalf("GetDeviceList: %v", err)
	}
	if len(keys) != 1 {
		t.Fatalf("stored keys = %v, want exactly one", keys)
	}
	if resp.ApiKey == "" || resp.ApiKey != keys[0] {
		t.Errorf("api_key in setup response = %q, want the stored key %q", resp.ApiKey, keys[0])
	}
}

func TestLog_AcceptsPostAndReturnsOK(t *testing.T) {
	mux, _ := setup(t)
	req := httptest.NewRequest(http.MethodPost, "/api/log", strings.NewReader(`{"msg":"hi"}`))
	req.Header.Set("Access-Token", "key-1")
	req.Header.Set("Id", "dev-1")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Body.String(); got != "OK" {
		t.Errorf("body = %q, want OK", got)
	}
}

func TestLog_GetIsNotAllowed(t *testing.T) {
	mux, _ := setup(t)
	req := httptest.NewRequest(http.MethodGet, "/api/log", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	// The route is registered as `POST /api/log`; net/http's ServeMux 1.22+
	// returns 405 for the other methods.
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", rec.Code)
	}
}

// Sanity check that the body is fully read by the handler — exercising the
// `io.ReadAll(r.Body)` path. We use a body that returns an explicit EOF to
// make sure the handler doesn't crash if reading fails.
func TestLog_HandlesEmptyBody(t *testing.T) {
	mux, _ := setup(t)
	req := httptest.NewRequest(http.MethodPost, "/api/log", io.NopCloser(strings.NewReader("")))
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}
