package worker

import (
	"time"
	"trmnl-server-go/pkg/v1/config"
	"trmnl-server-go/pkg/v1/db"
	"trmnl-server-go/pkg/v1/plugin"
	"trmnl-server-go/pkg/v1/screens"
)

// Tick runs one refresh pass: for every device in the DB, every plugin's
// Render is called for every screen the plugin declares. It is the inner body
// of UpdateData and is exposed so tests can exercise a single iteration.
func Tick(c *config.Config, plugins []plugin.Plugin, store *db.Store) {
	keys, _ := store.GetDeviceList()
	for _, key := range keys {
		voltage, _ := store.GetDeviceVoltage(key)
		screens.RenderDevice(plugins, key, voltage)
	}
}

func UpdateData(c *config.Config, plugins []plugin.Plugin, store *db.Store) {
	for {
		Tick(c, plugins, store)
		time.Sleep(time.Duration(c.Common.UpdateTime) * time.Second)
	}
}
