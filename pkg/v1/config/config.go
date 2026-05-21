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
	Random     Random     `yaml:"random"`
}

type Twelvedata struct {
	TwelveDataAPIKey string   `yaml:"twelvedata_api_key"`
	Symbols          []string `yaml:"symbols"`
}

type Coingecko struct {
	Symbols []string `yaml:"symbols"`
}

type Weather struct {
	Location string `yaml:"location"`
}

type Random struct {
	APIKey string `yaml:"api_key"`
}

func GetConfig(filename string) (Config, error) {
	var c Config

	// Open the configuration file.
	file, err := os.Open(filename)
	if err != nil {
		log.Error().Err(err).Msg("Unable to open config file")
		os.Exit(1)
	}
	defer file.Close()

	// Decode the YAML configuration into the config struct.
	decoder := yaml.NewDecoder(file)
	err = decoder.Decode(&c)
	if err != nil {
		log.Error().Err(err).Msg("Error decoding file")
		os.Exit(1)
	}

	return c, nil
}
