package worker

import (
	"fmt"
	"time"
	"trmnl-server-go/pkg/v1/config"
	"trmnl-server-go/pkg/v1/db"
	"trmnl-server-go/pkg/v1/plugin"

	"github.com/rs/zerolog/log"
)

// Tick runs one refresh pass: for every device in the DB, every plugin's
// Render is called for every screen the plugin declares. It is the inner body
// of UpdateData and is exposed so tests can exercise a single iteration.
func Tick(c *config.Config, plugins []plugin.Plugin) {
	keys, _ := db.GetDeviceList(c.Common.Dbpath)
	for _, key := range keys {
		voltage, _ := db.GetDeviceVoltage(c.Common.Dbpath, key)
		for _, p := range plugins {
			for _, screen := range p.Screens() {
				path := fmt.Sprintf("public/%s_%s.png", key, screen)
				if err := p.Render(screen, path, voltage); err != nil {
					log.Error().Err(err).Str("plugin", p.Name()).Str("screen", screen).Msg("Failed to render screen")
				} else {
					log.Info().Str("plugin", p.Name()).Str("file", path).Msg("Updated plugin screen")
				}
			}
		}
	}
}

func UpdateData(c *config.Config, plugins []plugin.Plugin) {
	for {
		Tick(c, plugins)
		time.Sleep(time.Duration(c.Common.UpdateTime) * time.Second)
	}
}
