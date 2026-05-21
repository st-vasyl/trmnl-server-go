package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"trmnl-server-go/pkg/v1/config"
	"trmnl-server-go/pkg/v1/db"
	"trmnl-server-go/pkg/v1/plugin"
	"trmnl-server-go/pkg/v1/screens"

	"github.com/rs/zerolog/log"
)

type SetupResponse struct {
	Status     int    `json:"status,omitempty"`
	ApiKey     string `json:"api_key,omitempty"`
	FriendlyID string `json:"friendly_id,omitempty"`
	ImageURL   string `json:"image_url,omitempty"`
	Message    string `json:"message"`
}

func Serve(version string, c *config.Config, plugins []plugin.Plugin) {
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(fmt.Sprintf("App version: %s", version)))
	})

	http.HandleFunc("/public/", ServeFiles)

	http.HandleFunc("/api/setup", func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("Access-Token")
		deviceId := r.Header.Get("Id")
		voltage := r.Header.Get("Battery-Voltage")

		firstScreen := screens.GetScreenList(plugins)
		defaultScreen := ""
		if len(firstScreen) > 0 {
			defaultScreen = firstScreen[0]
		}
		db.RegisterDevice(c.Common.Dbpath, deviceId, apiKey, defaultScreen)

		log.Info().Str("func", "setup").Str("api-key", apiKey).Str("id", deviceId).Str("voltage", voltage).Msg("Device setup requested")

		s := SetupResponse{
			Status:     200,
			ApiKey:     apiKey,
			FriendlyID: "OLOLO",
			ImageURL:   "https://usetrmnl.com/images/setup/setup-logo.bmp",
			Message:    "Register at TRMNL GO",
		}
		msg, err := json.Marshal(s)
		if err != nil {
			log.Error().Str("func", "setup").Str("api-key", apiKey).Str("id", deviceId).Err(err).Msg("Failed to marshal setup response")
		}

		log.Debug().Str("func", "setup").Str("api-key", apiKey).Str("id", deviceId).Str("response", string(msg)).Msg("Setup response sent")

		w.WriteHeader(200)
		w.Write(msg)
	})

	http.HandleFunc("/api/display", func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("Access-Token")
		deviceId := r.Header.Get("Id")
		voltage := r.Header.Get("Battery-Voltage")

		log.Info().Str("func", "display").Str("api-key", apiKey).Str("id", deviceId).Str("voltage", voltage).Msg("Display requested")

		msg := screens.RenderDisplay(c, plugins, deviceId, apiKey, voltage)
		log.Debug().Str("func", "display").Str("api-key", apiKey).Str("id", deviceId).Str("response", string(msg)).Msg("Display response sent")

		w.WriteHeader(200)
		w.Write(msg)
	})

	http.HandleFunc("POST /api/log", func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("Access-Token")
		deviceId := r.Header.Get("Id")
		voltage := r.Header.Get("Battery-Voltage")

		body, err := io.ReadAll(r.Body)
		r.Body.Close()
		if err != nil {
			log.Error().Str("func", "log").Str("api-key", apiKey).Str("id", deviceId).Err(err).Msg("Failed to read device log")
		}

		log.Info().Str("func", "log").Str("api-key", apiKey).Str("id", deviceId).Str("voltage", voltage).Str("logs", string(body)).Msg("Device log received")

		w.WriteHeader(200)
		w.Write([]byte("OK"))
	})

	log.Info().Str("version", version).Int("port", c.Common.Port).Msg("HTTP server started")

	if err := http.ListenAndServe(fmt.Sprintf(":%d", c.Common.Port), nil); err != nil {
		log.Error().Str("version", version).Int("port", c.Common.Port).Err(err).Msg("HTTP server failed")
	}
}

func ServeFiles(w http.ResponseWriter, r *http.Request) {
	apiKey := r.Header.Get("Access-Token")
	deviceId := r.Header.Get("Id")
	voltage := r.Header.Get("Battery-Voltage")
	path := "." + r.URL.Path

	log.Info().Str("func", "ServeFiles").Str("api-key", apiKey).Str("id", deviceId).Str("voltage", voltage).Str("file", r.RequestURI).Msg("File requested")

	http.ServeFile(w, r, path)
}
