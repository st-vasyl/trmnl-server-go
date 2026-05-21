package screens

import (
	"encoding/json"
	"fmt"
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

func RenderDisplay(c *config.Config, plugins []plugin.Plugin, deviceId, apiKey, voltage string) []byte {
	screen, err := db.GetDeviceScreen(c.Common.Dbpath, deviceId)
	if err != nil {
		log.Debug().Err(err).Str("deviceId", deviceId).Msg("Device not found in DB, registering.")
		screen = firstScreen(plugins)
		db.RegisterDevice(c.Common.Dbpath, deviceId, apiKey, screen)
	}

	filename := fmt.Sprintf("public/%s_%s.png", apiKey, screen)
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
	db.UpdateDevice(c.Common.Dbpath, deviceId, voltage, nextScreen)

	return res
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
