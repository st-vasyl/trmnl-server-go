package screens

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"trmnl-server-go/pkg/v1/config"
	"trmnl-server-go/pkg/v1/db"
	"trmnl-server-go/pkg/v1/plugin"
)

// fakePlugin lets screens-package tests drive screen rotation without touching
// real HTTP/render logic.
type fakePlugin struct {
	name    string
	screens []string
}

func (f *fakePlugin) Name() string                                     { return f.name }
func (f *fakePlugin) Screens() []string                                { return f.screens }
func (f *fakePlugin) Render(screen, path string, voltage float32) error { return nil }

func TestGetScreenList_FlattensAllPlugins(t *testing.T) {
	plugins := []plugin.Plugin{
		&fakePlugin{name: "p1", screens: []string{"a", "b"}},
		&fakePlugin{name: "p2", screens: []string{"c"}},
	}
	got := GetScreenList(plugins)
	want := []string{"a", "b", "c"}
	if !equalStrings(got, want) {
		t.Errorf("GetScreenList = %v, want %v", got, want)
	}
}

func TestGetScreenList_EmptyPluginList(t *testing.T) {
	got := GetScreenList(nil)
	if len(got) != 0 {
		t.Errorf("expected empty list, got %v", got)
	}
}

func TestGetNextScreen_AdvancesThroughCycle(t *testing.T) {
	list := []string{"a", "b", "c"}
	tests := []struct {
		current, want string
	}{
		{"a", "b"},
		{"b", "c"},
		{"c", "a"}, // wraps
	}
	for _, tc := range tests {
		if got := getNextScreen(tc.current, list); got != tc.want {
			t.Errorf("getNextScreen(%q) = %q, want %q", tc.current, got, tc.want)
		}
	}
}

func TestGetNextScreen_UnknownCurrentReturnsFirst(t *testing.T) {
	list := []string{"a", "b", "c"}
	if got := getNextScreen("zzz", list); got != "a" {
		t.Errorf("getNextScreen with unknown current = %q, want %q", got, "a")
	}
}

func TestIndexOf(t *testing.T) {
	list := []string{"x", "y", "z"}
	if got := indexOf("y", list); got != 1 {
		t.Errorf("indexOf y = %d, want 1", got)
	}
	if got := indexOf("missing", list); got != -1 {
		t.Errorf("indexOf missing = %d, want -1", got)
	}
}

func TestFirstScreen(t *testing.T) {
	if got := firstScreen(nil); got != "" {
		t.Errorf("firstScreen(nil) = %q, want empty", got)
	}
	p := &fakePlugin{name: "p1", screens: []string{"first", "second"}}
	if got := firstScreen([]plugin.Plugin{p}); got != "first" {
		t.Errorf("firstScreen = %q, want first", got)
	}
}

func openTestStore(t *testing.T) *db.Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "trmnl.db")
	s, err := db.Open(path)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestRenderDisplay_NewDeviceGetsRegisteredAndAdvances(t *testing.T) {
	store := openTestStore(t)

	c := &config.Config{}
	c.Common.ExternalURL = "host:8080"
	c.Common.RefreshTime = 300

	plugins := []plugin.Plugin{
		&fakePlugin{name: "p", screens: []string{"weather", "crypto"}},
	}

	out := RenderDisplay(c, plugins, store, "dev-1", "key-1", "4.0")

	var resp DisplayResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		t.Fatalf("response not valid JSON: %v", err)
	}
	if resp.RefreshRate != 300 {
		t.Errorf("RefreshRate = %d, want 300", resp.RefreshRate)
	}
	if resp.ImageURL != "http://host:8080/public/key-1_weather.png" {
		t.Errorf("ImageURL = %q", resp.ImageURL)
	}

	// Device should be registered with current screen = "weather", and the
	// rotation advanced to "crypto" for next call.
	gotScreen, err := store.GetDeviceScreen("dev-1")
	if err != nil {
		t.Fatalf("GetDeviceScreen: %v", err)
	}
	if gotScreen != "crypto" {
		t.Errorf("after RenderDisplay screen = %q, want crypto", gotScreen)
	}
}

func TestRenderDisplay_AdvancesExistingDevice(t *testing.T) {
	store := openTestStore(t)
	if err := store.RegisterDevice("dev-1", "key-1", "crypto"); err != nil {
		t.Fatalf("RegisterDevice: %v", err)
	}

	c := &config.Config{}
	c.Common.ExternalURL = "host:8080"
	c.Common.RefreshTime = 300

	plugins := []plugin.Plugin{
		&fakePlugin{name: "p", screens: []string{"weather", "crypto"}},
	}

	out := RenderDisplay(c, plugins, store, "dev-1", "key-1", "4.0")

	var resp DisplayResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		t.Fatalf("response not valid JSON: %v", err)
	}
	// Current screen is "crypto", advancing wraps to "weather".
	if resp.ImageURL != "http://host:8080/public/key-1_crypto.png" {
		t.Errorf("ImageURL = %q, want crypto image", resp.ImageURL)
	}

	gotScreen, err := store.GetDeviceScreen("dev-1")
	if err != nil {
		t.Fatalf("GetDeviceScreen: %v", err)
	}
	if gotScreen != "weather" {
		t.Errorf("after rotation screen = %q, want weather", gotScreen)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
