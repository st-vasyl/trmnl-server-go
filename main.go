package main

import (
	"os"
	"sync"
	"time"
	"trmnl-server-go/pkg/v1/config"
	"trmnl-server-go/pkg/v1/db"
	"trmnl-server-go/pkg/v1/fonts"
	"trmnl-server-go/pkg/v1/handler"
	"trmnl-server-go/pkg/v1/plugin"
	"trmnl-server-go/pkg/v1/plugins/crypto"
	"trmnl-server-go/pkg/v1/plugins/random"
	"trmnl-server-go/pkg/v1/plugins/stocks"
	"trmnl-server-go/pkg/v1/plugins/weather"
	"trmnl-server-go/pkg/v1/render"
	"trmnl-server-go/pkg/v1/worker"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var configFile = "config.yaml"

func init() {
	zerolog.TimeFieldFormat = ""
	zerolog.TimestampFunc = func() time.Time {
		return time.Date(2008, 1, 8, 17, 5, 05, 0, time.UTC)
	}
	log.Logger = zerolog.New(os.Stdout).With().Timestamp().Logger()
}

func main() {
	c, err := config.GetConfig(configFile)
	if err != nil {
		log.Error().Str("config", configFile).Err(err).Msg("Failed to load config")
		os.Exit(1)
	}
	if c.Common.Debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	log.Info().Str("config", configFile).Str("db", c.Common.Dbpath).Msg("Services starting")

	store, err := db.Open(c.Common.Dbpath)
	if err != nil {
		log.Error().Str("dbpath", c.Common.Dbpath).Err(err).Msg("Failed to open DB")
		os.Exit(1)
	}
	defer store.Close()

	fontName := c.Common.FontName
	if fontName == "" {
		fontName = fonts.DefaultFont
	}
	fontBytes, err := fonts.Load(fontName)
	if err != nil {
		log.Error().Str("font", fontName).Err(err).Msg("Failed to load font")
		os.Exit(1)
	}
	if err := render.SetFont(fontBytes); err != nil {
		log.Error().Str("font", fontName).Err(err).Msg("Failed to parse font")
		os.Exit(1)
	}

	plugins := buildPlugins(&c)

	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		defer wg.Done()
		handler.Serve("0.0.1", &c, plugins, store)
	}()
	go func() {
		defer wg.Done()
		worker.UpdateData(&c, plugins, store)
	}()
	wg.Wait()

	log.Info().Msg("Services shut down")
}

// buildPlugins constructs the active plugin list from config.
// To enable or disable a plugin, add or remove it from enabled_plugins in config.yaml.
func buildPlugins(c *config.Config) []plugin.Plugin {
	enabled := make(map[string]bool, len(c.Common.EnabledPlugins))
	for _, name := range c.Common.EnabledPlugins {
		enabled[name] = true
	}

	var plugins []plugin.Plugin

	if enabled["weather"] {
		plugins = append(plugins, &weather.WeatherPlugin{
			Location: c.Plugins.Weather.Location,
		})
	}
	if enabled["twelvedata"] {
		plugins = append(plugins, &stocks.StocksPlugin{
			Symbols: c.Plugins.Twelvedata.Symbols,
			ApiKey:  c.Plugins.Twelvedata.TwelveDataAPIKey,
		})
	}
	if enabled["coingecko"] {
		plugins = append(plugins, &crypto.CryptoPlugin{
			Symbols: c.Plugins.Coingecko.Symbols,
		})
	}
	if enabled["random"] {
		plugins = append(plugins, &random.RandomPlugin{
			ApiKey: c.Plugins.Random.APIKey,
		})
	}

	return plugins
}
