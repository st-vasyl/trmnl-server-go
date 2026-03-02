package screens

import (
	"encoding/json"
	"fmt"
	"time"
	"trmnl-server-go/pkg/v1/config"
	"trmnl-server-go/pkg/v1/db"

	"github.com/rs/zerolog/log"
)

type DisplayResponse struct {
	Status         int    `json:"status,omitempty"`
	ImageURL       string `json:"image_url,omitempty"`
	Filename       string `json:"filename"`
	UpdateFirmware bool   `json:"update_firmware"`
	FirmwareUrl    string `json:"firmware_url"`
	RefreshRate    int    `json:"refresh_rate"`
	ResetFirmware  bool   `json:"reset_firmware"`
}

func RenderDisplay(c *config.Config, deviceId, apiKey, voltage string) (res []byte) {
	screen, err := db.GetDeviceScreen(c.Common.Dbpath, deviceId)
	if err != nil {
		log.Debug().
			Str("deviceId", deviceId).
			Err(err).
			Msg("Device not found in the DB. Registreing new device.")
		db.RegisterDevice(c.Common.Dbpath, deviceId, apiKey, c.Common.EnabledPlugins[0])
	}
	filename := fmt.Sprintf("public/%s_%s.png", apiKey, screen)
	r := DisplayResponse{
		Status:         0,
		ImageURL:       fmt.Sprintf("http://%s/%s", c.Common.ExternalUrl, filename),
		Filename:       time.Now().Format("2006-01-02 15:04:05"),
		UpdateFirmware: false,
		FirmwareUrl:    "",
		RefreshRate:    c.Common.RefreshTime,
		ResetFirmware:  false,
	}
	res, err = json.Marshal(r)
	if err != nil {
		log.Error().
			Str("func", "renderDisplay").
			Str("api-key", apiKey).
			Str("id", deviceId).
			Str("voltage", voltage).
			Err(err).
			Msg("Unable to marshal display responce")
	}
	screenList := GetScreenList(c)
	log.Debug().
		Str("func", "renderDisplay").
		Strs("list", screenList).
		Msg("Generated screen list")
	nextScreen := GetNextScreen(screen, screenList)
	err = db.UpdateDevice(c.Common.Dbpath, deviceId, voltage, nextScreen)
	return res
}

func GetScreenList(c *config.Config) []string {
	var screens []string

	for _, v := range c.Common.EnabledPlugins {
		switch {
		case v == "twelvedata":
			for _, jv := range c.Plugins.Twelvedata.Symbols {
				screens = append(screens, fmt.Sprintf("%s_%s", v, jv))
			}
		case v == "coingecko":
			for _, jv := range c.Plugins.Coingecko.Symbols {
				screens = append(screens, fmt.Sprintf("%s_%s", v, jv))
			}
		default:
			screens = append(screens, v)
		}
	}
	return screens
}

func GetNextScreen(c string, screens []string) string {
	i := indexOf(c, screens)
	if i == len(screens)-1 {
		return screens[0]
	}
	return screens[i+1]
}

func indexOf(element string, screens []string) int {
	for k, v := range screens {
		if element == v {
			return k
		}
	}
	return -1 //not found.
}

func PluginEnabled(p string, plugins []string) bool {
	for _, v := range plugins {
		if p == v {
			return true
		}
	}
	return false
}
