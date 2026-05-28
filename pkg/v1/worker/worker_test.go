package worker

import (
	"path/filepath"
	"sort"
	"sync"
	"testing"
	"trmnl-server-go/pkg/v1/config"
	"trmnl-server-go/pkg/v1/db"
	"trmnl-server-go/pkg/v1/plugin"
)

// recordingPlugin records each (screen, path) Render call.
type recordingPlugin struct {
	name    string
	screens []string

	mu    sync.Mutex
	calls []string // "screen:path"
}

func (p *recordingPlugin) Name() string      { return p.name }
func (p *recordingPlugin) Screens() []string { return p.screens }
func (p *recordingPlugin) Render(screen, path string, voltage float32) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls = append(p.calls, screen+":"+path)
	return nil
}

func setupConfig(t *testing.T) *config.Config {
	t.Helper()
	dbpath := filepath.Join(t.TempDir(), "trmnl.db")
	if err := db.InitDB(dbpath); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	c := &config.Config{}
	c.Common.Dbpath = dbpath
	return c
}

func TestTick_NoDevicesIsNoOp(t *testing.T) {
	c := setupConfig(t)
	p := &recordingPlugin{name: "p", screens: []string{"a", "b"}}

	Tick(c, []plugin.Plugin{p})

	if got := len(p.calls); got != 0 {
		t.Errorf("calls = %d, want 0 when there are no devices", got)
	}
}

func TestTick_RendersAllScreensForEachDevice(t *testing.T) {
	c := setupConfig(t)
	if err := db.RegisterDevice(c.Common.Dbpath, "dev-1", "key-1", "a"); err != nil {
		t.Fatalf("RegisterDevice 1: %v", err)
	}
	if err := db.RegisterDevice(c.Common.Dbpath, "dev-2", "key-2", "a"); err != nil {
		t.Fatalf("RegisterDevice 2: %v", err)
	}

	p1 := &recordingPlugin{name: "p1", screens: []string{"a", "b"}}
	p2 := &recordingPlugin{name: "p2", screens: []string{"c"}}

	Tick(c, []plugin.Plugin{p1, p2})

	// p1 has 2 screens × 2 devices = 4 calls. p2 has 1 screen × 2 devices = 2 calls.
	if got := len(p1.calls); got != 4 {
		t.Errorf("p1 calls = %d, want 4 (got %v)", got, p1.calls)
	}
	if got := len(p2.calls); got != 2 {
		t.Errorf("p2 calls = %d, want 2 (got %v)", got, p2.calls)
	}

	// Verify the output paths are the expected `public/<key>_<screen>.png` form.
	want := []string{
		"a:public/key-1_a.png",
		"a:public/key-2_a.png",
		"b:public/key-1_b.png",
		"b:public/key-2_b.png",
	}
	got := append([]string(nil), p1.calls...)
	sort.Strings(got)
	sort.Strings(want)
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("p1.calls[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
