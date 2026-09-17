package config

import (
	"os"

	"github.com/rs/zerolog/log"
	yaml "sigs.k8s.io/yaml/goyaml.v2"
)

// common:
//   external_url: "10.0.0.1:8080"
//   port: 8080
//   dbpath: "./trmnl.db"
// 	 refresh_time: 300
// 	 update_time: 3600
//   enabled_plugins: ["twelvedata", "coingecko", "weather"]

// plugins:
//   twelvedata:
// 		twelvedata_api_key: demo
// 		symbols: ["aapl", "nvda"]
//   coingecko:
//     symbols: ["bitcoin"]

type Config struct {
	Common  Common  `yaml:"common"`
	Plugins Plugins `yaml:"plugins"`
}

type Common struct {
	ExternalURL    string   `yaml:"external_url"`
	Port           int      `yaml:"port"`
	Dbpath         string   `yaml:"dbpath"`
	RefreshTime    int      `yaml:"refresh_time"`
	UpdateTime     int      `yaml:"update_time"`
	Debug          bool     `yaml:"debug"`
	EnabledPlugins []string `yaml:"enabled_plugins"`
	FontName       string   `yaml:"font_name"`
}

type Plugins struct {
	Twelvedata Twelvedata `yaml:"twelvedata"`
	Coingecko  Coingecko  `yaml:"coingecko"`
	Weather    Weather    `yaml:"weather"`
	Currency   Currency   `yaml:"currency"`
	Calendar   Calendar   `yaml:"calendar"`
}

type Twelvedata struct {
	TwelveDataAPIKey string   `yaml:"twelvedata_api_key"`
	Symbols          []string `yaml:"symbols"`
}

type Coingecko struct {
	Symbols []string `yaml:"symbols"`
}

// Weather configures the weather plugin: the city to geocode and the units
// the screen shows. TemperatureUnit is "celsius" (default) or "fahrenheit";
// WindSpeedUnit is "ms" (default), "kmh", "mph" or "kn".
type Weather struct {
	Location        string `yaml:"location"`
	TemperatureUnit string `yaml:"temperature_unit"`
	WindSpeedUnit   string `yaml:"wind_speed_unit"`
}

// Currency lists the screens of the currency plugin. Each screen becomes one
// rendered image in the device rotation.
type Currency struct {
	Screens []CurrencyScreen `yaml:"screens"`
}

// CurrencyScreen holds up to four pairs such as "EUR/UAH".
type CurrencyScreen struct {
	Pairs []string `yaml:"pairs"`
}

// Calendar configures the calendar plugin: the zone that defines "today", the
// screen layout ("timeline" by default, or "list") and the ICS feeds merged
// into the day view.
type Calendar struct {
	Timezone  string           `yaml:"timezone"`
	Layout    string           `yaml:"layout"`
	Calendars []CalendarSource `yaml:"calendars"`
}

// CalendarSource is one ICS feed shown under a short label.
type CalendarSource struct {
	Name string `yaml:"name"`
	URL  string `yaml:"url"`
}

func GetConfig(filename string) (Config, error) {
	var c Config

	file, err := os.Open(filename)
	if err != nil {
		log.Error().Err(err).Msg("Unable to open config file")
		return c, err
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&c); err != nil {
		log.Error().Err(err).Msg("Error decoding file")
		return c, err
	}

	return c, nil
}
