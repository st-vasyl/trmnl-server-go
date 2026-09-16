package main

import (
	"strings"
	"testing"
	"trmnl-server-go/pkg/v1/config"
)

func TestBuildPlugins_EnablesCurrencyFromConfig(t *testing.T) {
	c := &config.Config{}
	c.Common.EnabledPlugins = []string{"weather", "currency"}
	c.Plugins.Weather.Location = "Kyiv"
	c.Plugins.Currency.Screens = []config.CurrencyScreen{{Pairs: []string{"EUR/PLN", "USD/PLN"}}}

	plugins, err := buildPlugins(c)
	if err != nil {
		t.Fatalf("buildPlugins: %v", err)
	}
	var names []string
	for _, p := range plugins {
		names = append(names, p.Name())
	}
	if got := strings.Join(names, ","); got != "weather,currency" {
		t.Errorf("plugins = %v, want [weather currency]", names)
	}
}

func TestBuildPlugins_InvalidCurrencyConfigIsAnError(t *testing.T) {
	c := &config.Config{}
	c.Common.EnabledPlugins = []string{"currency"}
	c.Plugins.Currency.Screens = []config.CurrencyScreen{{Pairs: []string{"EUR/XXX"}}}

	if _, err := buildPlugins(c); err == nil {
		t.Fatal("expected error for an unknown currency code")
	}
}

func TestBuildPlugins_EnablesCalendarFromConfig(t *testing.T) {
	c := &config.Config{}
	c.Common.EnabledPlugins = []string{"calendar", "weather"}
	c.Plugins.Weather.Location = "Kyiv"
	c.Plugins.Calendar.Timezone = "Europe/Kyiv"
	c.Plugins.Calendar.Calendars = []config.CalendarSource{
		{Name: "Work", URL: "https://example.com/work.ics"},
		{Name: "Home", URL: "webcal://example.com/home.ics"},
	}

	plugins, err := buildPlugins(c)
	if err != nil {
		t.Fatalf("buildPlugins: %v", err)
	}
	var names []string
	for _, p := range plugins {
		names = append(names, p.Name())
	}
	// Rotation order is the fixed order of buildPlugins, not enabled_plugins.
	if got := strings.Join(names, ","); got != "weather,calendar" {
		t.Errorf("plugins = %v, want [weather calendar]", names)
	}
}

func TestBuildPlugins_InvalidCalendarConfigIsAnError(t *testing.T) {
	c := &config.Config{}
	c.Common.EnabledPlugins = []string{"calendar"}
	c.Plugins.Calendar.Calendars = []config.CalendarSource{{Name: "Work", URL: "ftp://example.com/work.ics"}}

	if _, err := buildPlugins(c); err == nil {
		t.Fatal("expected error for a non-http feed URL")
	}
}

func TestBuildPlugins_UnknownCalendarLayoutIsAnError(t *testing.T) {
	c := &config.Config{}
	c.Common.EnabledPlugins = []string{"calendar"}
	c.Plugins.Calendar.Layout = "grid"
	c.Plugins.Calendar.Calendars = []config.CalendarSource{{Name: "Work", URL: "https://example.com/work.ics"}}

	if _, err := buildPlugins(c); err == nil {
		t.Fatal("expected error for an unknown layout")
	}
}

func TestBuildPlugins_CalendarEnabledWithoutFeedsIsAnError(t *testing.T) {
	c := &config.Config{}
	c.Common.EnabledPlugins = []string{"calendar"}

	if _, err := buildPlugins(c); err == nil {
		t.Fatal("expected error when calendar is enabled with no feeds")
	}
}

func TestBuildPlugins_CurrencyEnabledWithoutScreensIsAnError(t *testing.T) {
	c := &config.Config{}
	c.Common.EnabledPlugins = []string{"currency"}

	if _, err := buildPlugins(c); err == nil {
		t.Fatal("expected error when currency is enabled with no screens")
	}
}
