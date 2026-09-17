package screens

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
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

func (f *fakePlugin) Name() string                                      { return f.name }
func (f *fakePlugin) Screens() []string                                 { return f.screens }
func (f *fakePlugin) Render(screen, path string, voltage float32) error { return nil }

type renderCall struct {
	screen  string
	voltage float32
}

// writingPlugin records every Render call and writes a placeholder file, so
// tests can check both what was rendered and that the PNG path exists.
type writingPlugin struct {
	name    string
	screens []string

	mu    sync.Mutex
	calls []renderCall
}

func (p *writingPlugin) Name() string      { return p.name }
func (p *writingPlugin) Screens() []string { return p.screens }
func (p *writingPlugin) Render(screen, outputPath string, voltage float32) error {
	p.mu.Lock()
	p.calls = append(p.calls, renderCall{screen, voltage})
	p.mu.Unlock()
	return os.WriteFile(outputPath, []byte("png"), 0644)
}

func (p *writingPlugin) recorded() []renderCall {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]renderCall(nil), p.calls...)
}

// chdirWithPublic runs the rest of the test from a temp working directory
// containing public/, mirroring the server's CWD-relative layout. Background
// renders are waited for before the CWD is restored so nothing lands in the
// package directory.
func chdirWithPublic(t *testing.T) {
	t.Helper()
	t.Chdir(t.TempDir())
	t.Cleanup(WaitForRenders)
	if err := os.MkdirAll("public", 0755); err != nil {
		t.Fatalf("mkdir public: %v", err)
	}
}

func fileExists(t *testing.T, path string) bool {
	t.Helper()
	_, err := os.Stat(path)
	return err == nil
}

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

func TestRenderDisplay_RendersRequestedScreenBeforeReturning(t *testing.T) {
	chdirWithPublic(t)
	store := openTestStore(t)

	c := &config.Config{}
	c.Common.ExternalURL = "host:8080"
	c.Common.RefreshTime = 300
	p := &writingPlugin{name: "p", screens: []string{"weather", "crypto"}}

	RenderDisplay(c, []plugin.Plugin{p}, store, "dev-1", "key-1", "4.1")

	// The screen named in image_url must exist the moment the response is
	// built: the device downloads it immediately.
	if !fileExists(t, "public/key-1_weather.png") {
		t.Fatal("public/key-1_weather.png missing right after RenderDisplay returned")
	}
	calls := p.recorded()
	if len(calls) == 0 || calls[0].screen != "weather" {
		t.Fatalf("first Render call = %v, want the weather screen", calls)
	}
	if got := calls[0].voltage; got < 4.09 || got > 4.11 {
		t.Errorf("voltage passed to Render = %v, want 4.1 from the Battery-Voltage header", got)
	}

	// The rest of the rotation is filled in the background so the next
	// /api/display also finds its file.
	WaitForRenders()
	if !fileExists(t, "public/key-1_crypto.png") {
		t.Error("public/key-1_crypto.png not rendered in the background")
	}
}

func TestRenderDisplay_SkipsRenderWhenImageExists(t *testing.T) {
	chdirWithPublic(t)
	store := openTestStore(t)
	if _, err := store.RegisterDevice("dev-1", "key-1", "crypto"); err != nil {
		t.Fatalf("RegisterDevice: %v", err)
	}
	if err := os.WriteFile("public/key-1_crypto.png", []byte("old"), 0644); err != nil {
		t.Fatalf("seed image: %v", err)
	}

	c := &config.Config{}
	c.Common.ExternalURL = "host:8080"
	p := &writingPlugin{name: "p", screens: []string{"weather", "crypto"}}

	RenderDisplay(c, []plugin.Plugin{p}, store, "dev-1", "key-1", "4.1")
	WaitForRenders()

	if calls := p.recorded(); len(calls) != 0 {
		t.Errorf("Render called %d times although the image already exists: %v", len(calls), calls)
	}
}

func TestRenderScreen_UnknownScreenReturnsError(t *testing.T) {
	p := &writingPlugin{name: "p", screens: []string{"a"}}
	if err := RenderScreen([]plugin.Plugin{p}, "key-1", "zzz", 4.0); err == nil {
		t.Fatal("expected an error for a screen no plugin provides")
	}
	if calls := p.recorded(); len(calls) != 0 {
		t.Errorf("Render called for an unknown screen: %v", calls)
	}
}

func TestRenderDevice_RendersEveryScreenOfEveryPlugin(t *testing.T) {
	chdirWithPublic(t)
	p1 := &writingPlugin{name: "p1", screens: []string{"a", "b"}}
	p2 := &writingPlugin{name: "p2", screens: []string{"c"}}

	RenderDevice([]plugin.Plugin{p1, p2}, "key-1", 4.0)

	for _, f := range []string{"public/key-1_a.png", "public/key-1_b.png", "public/key-1_c.png"} {
		if !fileExists(t, f) {
			t.Errorf("%s not rendered", f)
		}
	}
}

func TestImagePath(t *testing.T) {
	if got := ImagePath("key-1", "weather"); got != "public/key-1_weather.png" {
		t.Errorf("ImagePath = %q, want public/key-1_weather.png", got)
	}
}

func TestParseVoltage(t *testing.T) {
	tests := []struct {
		in   string
		want float32
	}{
		{"4.1", 4.1},
		{"", 0},
		{"garbage", 0},
	}
	for _, tc := range tests {
		got := parseVoltage(tc.in)
		if d := got - tc.want; d > 0.001 || d < -0.001 {
			t.Errorf("parseVoltage(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestRenderDisplay_AdvancesExistingDevice(t *testing.T) {
	store := openTestStore(t)
	if _, err := store.RegisterDevice("dev-1", "key-1", "crypto"); err != nil {
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
