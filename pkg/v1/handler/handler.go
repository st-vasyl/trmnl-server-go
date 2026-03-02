package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"trmnl-server-go/pkg/v1/config"
	"trmnl-server-go/pkg/v1/db"
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

func Serve(version string, c *config.Config) {
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		msg := fmt.Sprintf("App version: %s", version)
		w.WriteHeader(200)
		w.Write([]byte(msg))
	})

	http.HandleFunc("/public/", ServeFiles)

	http.HandleFunc("/api/setup", func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("Access-Token")
		deviceId := r.Header.Get("Id")
		voltage := r.Header.Get("Battery-Voltage")
		db.RegisterDevice(c.Common.Dbpath, deviceId, apiKey, c.Common.EnabledPlugins[0])

		log.Info().
			Str("func", "setup").
			Str("api-key", apiKey).
			Str("id", deviceId).
			Str("voltage", voltage).
			Msg("Requested setup from device")

		s := SetupResponse{
			Status:     200,
			ApiKey:     apiKey,
			FriendlyID: "OLOLO",
			ImageURL:   "https://usetrmnl.com/images/setup/setup-logo.bmp",
			Message:    "Register at TRMNL GO",
		}
		msg, err := json.Marshal(s)
		if err != nil {
			log.Error().
				Str("func", "setup").
				Str("api-key", apiKey).
				Str("id", deviceId).
				Str("voltage", voltage).
				Err(err).
				Msg("Unable to marshal setup responce")
		}

		log.Debug().
			Str("func", "setup").
			Str("api-key", apiKey).
			Str("id", deviceId).
			Str("voltage", voltage).
			Str("responce", string(msg)).
			Msg("Setup responce to device")
		w.WriteHeader(200)
		w.Write([]byte(msg))
	})

	http.HandleFunc("/api/display", func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("Access-Token")
		deviceId := r.Header.Get("Id")
		voltage := r.Header.Get("Battery-Voltage")

		log.Info().
			Str("func", "display").
			Str("api-key", apiKey).
			Str("id", deviceId).
			Str("voltage", voltage).
			Msg("Requested display from device")

		msg := screens.RenderDisplay(c, deviceId, apiKey, voltage)

		log.Debug().
			Str("func", "display").
			Str("api-key", apiKey).
			Str("id", deviceId).
			Str("voltage", voltage).
			Str("responce", string(msg)).
			Msg("Display responce to device")

		w.WriteHeader(200)
		w.Write([]byte(msg))
	})

	http.HandleFunc("POST /api/log", func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("Access-Token")
		deviceId := r.Header.Get("Id")
		voltage := r.Header.Get("Battery-Voltage")

		body, err := io.ReadAll(r.Body)
		r.Body.Close()
		if err != nil {
			log.Error().
				Str("func", "log").
				Str("api-key", apiKey).
				Str("id", deviceId).
				Str("voltage", voltage).
				Err(err).
				Msg("Unable to read the log record")
		}

		log.Info().
			Str("func", "log").
			Str("api-key", apiKey).
			Str("id", deviceId).
			Str("voltage", voltage).
			Str("logs", string(body)).
			Msg("Requested display from device")

		w.WriteHeader(200)
		w.Write([]byte("OK"))
	})

	log.Info().
		Str("func", "HandleHTTP").
		Str("version", version).
		Int("port", c.Common.Port).
		Msg("HTTP server started successfully")

	err := http.ListenAndServe(fmt.Sprintf(":%d", c.Common.Port), nil)
	if err != nil {
		log.Error().
			Str("func", "HandleHTTP").
			Str("version", version).
			Int("port", c.Common.Port).
			Err(err).
			Msg("Cannot start HTTP server")
	}
}

func ServeFiles(w http.ResponseWriter, r *http.Request) {
	apiKey := r.Header.Get("Access-Token")
	deviceId := r.Header.Get("Id")
	voltage := r.Header.Get("Battery-Voltage")
	path := "." + r.URL.Path
	log.Info().
		Str("func", "ServeFiles").
		Str("api-key", apiKey).
		Str("id", deviceId).
		Str("voltage", voltage).
		Str("file", r.RequestURI).
		Msg("Requested file for device")

	http.ServeFile(w, r, path)
}
