// External test package so we can import the concrete plugin packages, which
// themselves depend on `plugin`. This catches contract regressions at compile
// time: if anyone changes the Plugin interface or breaks an implementation,
// `go test ./pkg/v1/plugin/...` fails.
package plugin_test

import (
	"testing"
	"trmnl-server-go/pkg/v1/plugin"
	"trmnl-server-go/pkg/v1/plugins/crypto"
	"trmnl-server-go/pkg/v1/plugins/random"
	"trmnl-server-go/pkg/v1/plugins/stocks"
	"trmnl-server-go/pkg/v1/plugins/weather"
)

// Compile-time assertions: each concrete plugin must satisfy plugin.Plugin.
// Build will fail here if any plugin drops or changes a required method.
var (
	_ plugin.Plugin = (*weather.WeatherPlugin)(nil)
	_ plugin.Plugin = (*stocks.StocksPlugin)(nil)
	_ plugin.Plugin = (*crypto.CryptoPlugin)(nil)
	_ plugin.Plugin = (*random.RandomPlugin)(nil)
)

// stubPlugin is a minimal Plugin used to verify the contract is callable from
// code that only knows about the interface — i.e. the worker/screens pattern.
type stubPlugin struct {
	name    string
	screens []string
	calls   int
}

func (s *stubPlugin) Name() string                                       { return s.name }
func (s *stubPlugin) Screens() []string                                  { return s.screens }
func (s *stubPlugin) Render(screen, outputPath string, voltage float32) error {
	s.calls++
	return nil
}

func TestPlugin_InterfaceCanBeImplementedAndInvoked(t *testing.T) {
	var p plugin.Plugin = &stubPlugin{name: "stub", screens: []string{"a", "b"}}

	if p.Name() != "stub" {
		t.Errorf("Name = %q, want stub", p.Name())
	}
	if got := p.Screens(); len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("Screens = %v, want [a b]", got)
	}
	if err := p.Render("a", "/tmp/x.png", 4.0); err != nil {
		t.Errorf("Render: %v", err)
	}
}

func TestPlugin_ConcretePluginsExposeNonEmptyName(t *testing.T) {
	// A plugin with no name would be unrouteable in the worker dispatch; pin
	// down that each concrete plugin returns a stable identifier.
	plugins := []plugin.Plugin{
		&weather.WeatherPlugin{},
		&stocks.StocksPlugin{},
		&crypto.CryptoPlugin{},
		&random.RandomPlugin{},
	}
	wantNames := map[string]bool{
		"weather":    true,
		"twelvedata": true,
		"coingecko":  true,
		"random":     true,
	}
	for _, p := range plugins {
		name := p.Name()
		if name == "" {
			t.Errorf("plugin %T returned empty Name()", p)
			continue
		}
		if !wantNames[name] {
			t.Errorf("plugin %T returned unexpected name %q", p, name)
		}
	}
}
