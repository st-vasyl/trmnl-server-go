package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"
	"trmnl-server-go/pkg/v1/db"
	"trmnl-server-go/pkg/v1/plugins/crypto"
	"trmnl-server-go/pkg/v1/plugins/stocks"
	"trmnl-server-go/pkg/v1/plugins/weather"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	hostname         = "172.16.30.187"
	port             = "8080"
	dbname           = "./trmnl.db"
	timeout          = 300
	updateTime       = 3600
	twelvedataApiKey = "d79ba574546146b8b49def6c048988e4"
)

var plugins = []string{"crypto", "weather", "stocks_aapl", "stocks_nvda"}

type DisplayResponse struct {
	Status         int    `json:"status,omitempty"`
	ImageURL       string `json:"image_url,omitempty"`
	Filename       string `json:"filename"`
	UpdateFirmware bool   `json:"update_firmware"`
	FirmwareUrl    string `json:"firmware_url"`
	RefreshRate    int    `json:"refresh_rate"`
	ResetFirmware  bool   `json:"reset_firmware"`
}

type SetupResponse struct {
	Status     int    `json:"status,omitempty"`
	ApiKey     string `json:"api_key,omitempty"`
	FriendlyID string `json:"friendly_id,omitempty"`
	ImageURL   string `json:"image_url,omitempty"`
	Message    string `json:"message"`
}

func init() {
	zerolog.TimeFieldFormat = ""
	zerolog.TimestampFunc = func() time.Time {
		return time.Date(2008, 1, 8, 17, 5, 05, 0, time.UTC)
	}
	log.Logger = zerolog.New(os.Stdout).With().Timestamp().Logger()
}

func main() {
	err := db.InitDB(dbname)
	if err != nil {
		log.Error().
			Str("dbname", dbname).
			Err(err).
			Msg("Failed to init DB")
		os.Exit(1)
	}

	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		defer wg.Done()
		HandleHTTP("main", "00000", "0.0.1", port)
	}()
	go func() {
		defer wg.Done()
		UpdateData()
	}()
	wg.Wait()
	log.Info().Msg("Services has shut down")
}

func renderDisplay(port, deviceId, apiKey, voltage string) (res []byte) {
	screen, err := db.GetDeviceScreen(dbname, deviceId)
	if err != nil {
		log.Debug().
			Str("deviceId", deviceId).
			Err(err).
			Msg("Device not found in the DB. Registreing new device.")
		db.RegisterDevice(dbname, deviceId, apiKey, plugins[0])
	}
	filename := fmt.Sprintf("public/%s_%s.png", apiKey, screen)
	r := DisplayResponse{
		Status:         0,
		ImageURL:       fmt.Sprintf("http://%s:%s/%s", hostname, port, filename),
		Filename:       time.Now().Format("2006-01-02 15:04:05"),
		UpdateFirmware: false,
		FirmwareUrl:    "",
		RefreshRate:    timeout,
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
	nextScreen := getNextScreen(screen)
	err = db.UpdateDevice(dbname, deviceId, voltage, nextScreen)
	return res
}

func HandleHTTP(branch, commithash, version, port string) {
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		msg := fmt.Sprintf("App version: %s, commit: %s branch: %s ", version, commithash, branch)
		w.WriteHeader(200)
		w.Write([]byte(msg))
	})

	http.HandleFunc("/public/", ServeFiles)

	http.HandleFunc("/api/setup", func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("Access-Token")
		deviceId := r.Header.Get("Id")
		voltage := r.Header.Get("Battery-Voltage")
		db.RegisterDevice(dbname, deviceId, apiKey, plugins[0])

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

			// log.Printf("DEBUG: recieved headers from device %s \n", r.Header.Get("Access-Token"))
			// for k, v := range r.Header {
			// 	log.Printf("Header field %s, Value %s \n", k, v)
			// }

		msg := renderDisplay(port, deviceId, apiKey, voltage)

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
		Str("branch", branch).
		Str("commit", commithash).
		Str("version", version).
		Str("port", port).
		Msg("HTTP server started successfully")

	err := http.ListenAndServe(fmt.Sprintf(":%s", port), nil)
	if err != nil {
		log.Error().
			Str("func", "HandleHTTP").
			Str("branch", branch).
			Str("commit", commithash).
			Str("version", version).
			Str("port", port).
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

func UpdateData() {
	keys, _ := db.GetDeviceList(dbname)
	for {
		for _, key := range keys {
			prefix := fmt.Sprintf("public/%s", key)
			voltage, _ := db.GetDeviceVoltage(dbname, key)
			stocks.RenderScreenStocks(
				800,
				480,
				"AAPL",
				"demo",
				fmt.Sprintf("%s_stocks_aapl.png", prefix),
				voltage,
			)
			log.Info().
				Str("func", "update").
				Str("file", fmt.Sprintf("%s_stocks_aapl.png", prefix)).
				Str("plugin", "stocks").
				Msg("Updated data for plugin successfully")

			stocks.RenderScreenStocks(
				800,
				480,
				"NVDA",
				twelvedataApiKey,
				fmt.Sprintf("%s_stocks_nvda.png", prefix),
				voltage,
			)
			log.Info().
				Str("func", "update").
				Str("file", fmt.Sprintf("%s_stocks_nvda.png", prefix)).
				Str("plugin", "stocks").
				Msg("Updated data for plugin successfully")

			crypto.RenderScreenCrypto(
				800,
				480,
				"bitcoin",
				fmt.Sprintf("%s_crypto.png", prefix),
				voltage,
			)
			log.Info().
				Str("func", "update").
				Str("file", fmt.Sprintf("%s_crypto.png", prefix)).
				Str("plugin", "crypto").
				Msg("Updated data for plugin successfully")

			weather.RenderScreenWeather(
				800,
				480,
				"Wroclaw",
				fmt.Sprintf("%s_weather.png", prefix),
				voltage,
			)
			log.Info().
				Str("func", "update").
				Str("file", fmt.Sprintf("%s_weather.png", prefix)).
				Str("plugin", "weather").
				Msg("Updated data for plugin successfully")

			// random.RenderRandomImage(
			// 	800,
			// 	480,
			// 	"JG308I6uXMpRErxkMzzAy8tRuRSM50yjwGPhtjWvO1g",
			// 	"public/random_image.png",
			// 	log_level,
			// )
			// log.Printf("Update data for plugin: random image \n")
		}
		time.Sleep(updateTime * time.Second)
	}
}

func indexOf(element string, data []string) int {
	for k, v := range data {
		if element == v {
			return k
		}
	}
	return -1 //not found.
}

func getNextScreen(c string) string {
	i := indexOf(c, plugins)
	if i == len(plugins)-1 {
		return plugins[0]
	}
	return plugins[i+1]
}
