package config

import (
	"os"
	"path/filepath"
	"testing"
)

const validYAML = `
common:
  external_url: "10.0.0.5:8080"
  port: 8080
  dbpath: "./trmnl.db"
  refresh_time: 300
  update_time: 3600
  debug: true
  enabled_plugins:
    - weather
    - twelvedata
  font_name: "MyFont"

plugins:
  twelvedata:
    twelvedata_api_key: "td-key"
    symbols: ["aapl", "nvda"]
  coingecko:
    symbols: ["bitcoin"]
  weather:
    location: "Wroclaw"
`

func writeTempFile(t *testing.T, name, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	return path
}

func TestGetConfig_ParsesValidYAML(t *testing.T) {
	path := writeTempFile(t, "config.yaml", validYAML)

	c, err := GetConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if c.Common.ExternalURL != "10.0.0.5:8080" {
		t.Errorf("ExternalURL = %q, want %q", c.Common.ExternalURL, "10.0.0.5:8080")
	}
	if c.Common.Port != 8080 {
		t.Errorf("Port = %d, want 8080", c.Common.Port)
	}
	if c.Common.RefreshTime != 300 {
		t.Errorf("RefreshTime = %d, want 300", c.Common.RefreshTime)
	}
	if c.Common.UpdateTime != 3600 {
		t.Errorf("UpdateTime = %d, want 3600", c.Common.UpdateTime)
	}
	if !c.Common.Debug {
		t.Error("Debug = false, want true")
	}
	if c.Common.FontName != "MyFont" {
		t.Errorf("FontName = %q, want %q", c.Common.FontName, "MyFont")
	}
	if got, want := c.Common.EnabledPlugins, []string{"weather", "twelvedata"}; !equalStrings(got, want) {
		t.Errorf("EnabledPlugins = %v, want %v", got, want)
	}

	if c.Plugins.Twelvedata.TwelveDataAPIKey != "td-key" {
		t.Errorf("Twelvedata.APIKey = %q, want %q", c.Plugins.Twelvedata.TwelveDataAPIKey, "td-key")
	}
	if got, want := c.Plugins.Twelvedata.Symbols, []string{"aapl", "nvda"}; !equalStrings(got, want) {
		t.Errorf("Twelvedata.Symbols = %v, want %v", got, want)
	}
	if got, want := c.Plugins.Coingecko.Symbols, []string{"bitcoin"}; !equalStrings(got, want) {
		t.Errorf("Coingecko.Symbols = %v, want %v", got, want)
	}
	if c.Plugins.Weather.Location != "Wroclaw" {
		t.Errorf("Weather.Location = %q, want %q", c.Plugins.Weather.Location, "Wroclaw")
	}
}

func TestGetConfig_MissingFileReturnsError(t *testing.T) {
	_, err := GetConfig(filepath.Join(t.TempDir(), "does-not-exist.yaml"))
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestGetConfig_MalformedYAMLReturnsError(t *testing.T) {
	path := writeTempFile(t, "bad.yaml", "common: [this is not valid: yaml")

	_, err := GetConfig(path)
	if err == nil {
		t.Fatal("expected error for malformed YAML, got nil")
	}
}

const currencyYAML = `
common:
  enabled_plugins: ["currency"]
plugins:
  currency:
    screens:
      - pairs: ["EUR/PLN", "USD/PLN", "GBP/PLN", "CHF/PLN"]
      - pairs: ["EUR/USD", "USD/JPY"]
`

func TestGetConfig_ParsesCurrencyScreens(t *testing.T) {
	path := writeTempFile(t, "config.yaml", currencyYAML)

	c, err := GetConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	screens := c.Plugins.Currency.Screens
	if len(screens) != 2 {
		t.Fatalf("Currency.Screens len = %d, want 2", len(screens))
	}
	if got, want := screens[0].Pairs, []string{"EUR/PLN", "USD/PLN", "GBP/PLN", "CHF/PLN"}; !equalStrings(got, want) {
		t.Errorf("Screens[0].Pairs = %v, want %v", got, want)
	}
	if got, want := screens[1].Pairs, []string{"EUR/USD", "USD/JPY"}; !equalStrings(got, want) {
		t.Errorf("Screens[1].Pairs = %v, want %v", got, want)
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
