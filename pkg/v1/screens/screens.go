package screens

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strconv"
	"sync"
	"time"
	"trmnl-server-go/pkg/v1/config"
	"trmnl-server-go/pkg/v1/db"
	"trmnl-server-go/pkg/v1/plugin"

	"github.com/rs/zerolog/log"
)

type DisplayResponse struct {
	Status         int    `json:"status,omitempty"`
	ImageURL       string `json:"image_url,omitempty"`
	Filename       string `json:"filename"`
	UpdateFirmware bool   `json:"update_firmware"`
	FirmwareURL    string `json:"firmware_url"`
	RefreshRate    int    `json:"refresh_rate"`
	ResetFirmware  bool   `json:"reset_firmware"`
}

// ImagePath is where the rendered PNG for one device screen lives, relative to
// the working directory. The worker and the display handler must agree on it,
// which is why it is defined once here.
func ImagePath(apiKey, screen string) string {
	return fmt.Sprintf("public/%s_%s.png", apiKey, screen)
}

// renderMu serializes plugin renders. The worker tick and the on-demand render
// in RenderDisplay run on different goroutines, and plugins were written
// assuming one render at a time.
var renderMu sync.Mutex

// background tracks renders started by RenderDisplay so tests can wait for
// them instead of racing against the working directory being torn down.
var background sync.WaitGroup

// WaitForRenders blocks until every background render started by
// RenderDisplay has finished.
func WaitForRenders() { background.Wait() }

// RenderScreen renders a single screen for one device to its ImagePath.
func RenderScreen(plugins []plugin.Plugin, apiKey, screen string, voltage float32) error {
	p := pluginFor(plugins, screen)
	if p == nil {
		return fmt.Errorf("no plugin provides screen %q", screen)
	}
	renderMu.Lock()
	defer renderMu.Unlock()
	return p.Render(screen, ImagePath(apiKey, screen), voltage)
}

// RenderDevice renders every screen of every plugin for one device. Failures
// are logged per screen so one broken plugin does not stop the others.
func RenderDevice(plugins []plugin.Plugin, apiKey string, voltage float32) {
	for _, p := range plugins {
		for _, screen := range p.Screens() {
			path := ImagePath(apiKey, screen)
			renderMu.Lock()
			err := p.Render(screen, path, voltage)
			renderMu.Unlock()
			if err != nil {
				log.Error().Err(err).Str("plugin", p.Name()).Str("screen", screen).Msg("Failed to render screen")
			} else {
				log.Info().Str("plugin", p.Name()).Str("file", path).Msg("Updated plugin screen")
			}
		}
	}
}

func RenderDisplay(c *config.Config, plugins []plugin.Plugin, store *db.Store, deviceId, apiKey, voltage string) []byte {
	screen, err := store.GetDeviceScreen(deviceId)
	if err != nil {
		log.Debug().Err(err).Str("deviceId", deviceId).Msg("Device not found in DB, registering.")
		screen = firstScreen(plugins)
		store.RegisterDevice(deviceId, apiKey, screen)
	}

	filename := ImagePath(apiKey, screen)
	ensureRendered(plugins, apiKey, screen, parseVoltage(voltage))

	r := DisplayResponse{
		Status:         0,
		ImageURL:       fmt.Sprintf("http://%s/%s", c.Common.ExternalURL, filename),
		Filename:       time.Now().Format("2006-01-02 15:04:05"),
		UpdateFirmware: false,
		FirmwareURL:    "",
		RefreshRate:    c.Common.RefreshTime,
		ResetFirmware:  false,
	}

	res, err := json.Marshal(r)
	if err != nil {
		log.Error().Err(err).Str("apiKey", apiKey).Str("deviceId", deviceId).Msg("Failed to marshal display response")
	}

	screenList := GetScreenList(plugins)
	log.Debug().Strs("screens", screenList).Msg("Screen list")

	nextScreen := getNextScreen(screen, screenList)
	store.UpdateDevice(deviceId, voltage, nextScreen)

	return res
}

// ensureRendered makes sure the PNG a device is about to fetch exists. A
// device that registered after the last worker tick has no images yet, and the
// firmware downloads image_url immediately, so the requested screen is
// rendered before the response goes out and the rest of the rotation is
// filled in the background.
func ensureRendered(plugins []plugin.Plugin, apiKey, screen string, voltage float32) {
	if _, err := os.Stat(ImagePath(apiKey, screen)); err == nil {
		return
	}
	log.Info().Str("api-key", apiKey).Str("screen", screen).Msg("No image for device yet, rendering on demand")
	if err := RenderScreen(plugins, apiKey, screen, voltage); err != nil {
		log.Error().Err(err).Str("api-key", apiKey).Str("screen", screen).Msg("On-demand render failed")
	}
	background.Go(func() { RenderDevice(plugins, apiKey, voltage) })
}

// GetScreenList returns all screen names across all enabled plugins.
func GetScreenList(plugins []plugin.Plugin) []string {
	var screens []string
	for _, p := range plugins {
		screens = append(screens, p.Screens()...)
	}
	return screens
}

func getNextScreen(current string, screens []string) string {
	i := indexOf(current, screens)
	if i < 0 || i == len(screens)-1 {
		return screens[0]
	}
	return screens[i+1]
}

func indexOf(target string, screens []string) int {
	for i, s := range screens {
		if s == target {
			return i
		}
	}
	return -1
}

func firstScreen(plugins []plugin.Plugin) string {
	if len(plugins) > 0 && len(plugins[0].Screens()) > 0 {
		return plugins[0].Screens()[0]
	}
	return ""
}

func pluginFor(plugins []plugin.Plugin, screen string) plugin.Plugin {
	for _, p := range plugins {
		if slices.Contains(p.Screens(), screen) {
			return p
		}
	}
	return nil
}

// parseVoltage converts the Battery-Voltage header to the float the plugins
// take. An absent or malformed header renders as an empty battery, the same
// as a freshly registered row in the DB.
func parseVoltage(s string) float32 {
	v, err := strconv.ParseFloat(s, 32)
	if err != nil {
		return 0
	}
	return float32(v)
}
