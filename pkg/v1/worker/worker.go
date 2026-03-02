package worker

import (
	"fmt"
	"time"
	"trmnl-server-go/pkg/v1/config"
	"trmnl-server-go/pkg/v1/db"
	"trmnl-server-go/pkg/v1/plugins/crypto"
	"trmnl-server-go/pkg/v1/plugins/stocks"
	"trmnl-server-go/pkg/v1/plugins/weather"
	"trmnl-server-go/pkg/v1/screens"

	"github.com/rs/zerolog/log"
)

func UpdateData(c *config.Config) {
	keys, _ := db.GetDeviceList(c.Common.Dbpath)
	for {
		for _, key := range keys {
			prefix := fmt.Sprintf("public/%s", key)
			voltage, _ := db.GetDeviceVoltage(c.Common.Dbpath, key)

			if screens.PluginEnabled("twelvedata", c.Common.EnabledPlugins) {
				for _, v := range c.Plugins.Twelvedata.Symbols {
					stocks.RenderScreenStocks(
						800,
						480,
						v,
						c.Plugins.Twelvedata.TwelveDataApiKey,
						fmt.Sprintf("%s_twelvedata_%s.png", prefix, v),
						voltage,
					)

					log.Info().
						Str("func", "update").
						Str("file", fmt.Sprintf("%s_twelvedata_%s.png", prefix, v)).
						Str("plugin", "twelvedata").
						Msg("Updated data for plugin successfully")
				}
			}

			if screens.PluginEnabled("coingecko", c.Common.EnabledPlugins) {
				for _, v := range c.Plugins.Coingecko.Symbols {
					crypto.RenderScreenCrypto(
						800,
						480,
						v,
						fmt.Sprintf("%s_coingecko_%s.png", prefix, v),
						voltage,
					)
					log.Info().
						Str("func", "update").
						Str("file", fmt.Sprintf("%s_coingecko_%s.png", prefix, v)).
						Str("plugin", "coingecko").
						Msg("Updated data for plugin successfully")
				}
			}

			if screens.PluginEnabled("weather", c.Common.EnabledPlugins) {
				weather.RenderScreenWeather(
					800,
					480,
					c.Plugins.Weather.Location,
					fmt.Sprintf("%s_weather.png", prefix),
					voltage,
				)
				log.Info().
					Str("func", "update").
					Str("file", fmt.Sprintf("%s_weather.png", prefix)).
					Str("plugin", "weather").
					Str("location", c.Plugins.Weather.Location).
					Msg("Updated data for plugin successfully")
			}

		}
		time.Sleep(time.Duration(c.Common.UpdateTime) * time.Second)
	}
}
