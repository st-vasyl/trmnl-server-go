package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"trmnl-server-go/pkg/v1/config"
	"trmnl-server-go/pkg/v1/db"
	"trmnl-server-go/pkg/v1/plugin"
)

type fakePlugin struct {
	name    string
	screens []string
}

func (f *fakePlugin) Name() string                                     { return f.name }
func (f *fakePlugin) Screens() []string                                { return f.screens }
func (f *fakePlugin) Render(screen, path string, voltage float32) error { return nil }

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
