package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
	"trmnl-server-go/pkg/v1/db"
	"trmnl-server-go/pkg/v1/plugins/crypto"
	"trmnl-server-go/pkg/v1/plugins/weather"
)

const (
	hostname   = "172.16.30.187"
	port       = "8080"
	dbname     = "./trmnl.db"
	timeout    = 300
	updateTime = 3600
)

var plugins = []string{"crypto", "weather"}
var log_level = "info"

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

func renderDisplay(port, deviceId, apiKey, voltage string) (res []byte) {
	screen, err := db.GetDeviceScreen(dbname, deviceId)
	if err != nil {
		log.Printf("ERROR: no device with ID %s in the DB. Adding ..", deviceId)
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
		log.Fatalf("Error occurred during marshalling: %s", err.Error())
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
		log.Printf("Getting device registration: %s \n", r.Header.Get("Access-Token"))
		apiKey := r.Header.Get("Access-Token")
		deviceId := r.Header.Get("Id")
		db.RegisterDevice(dbname, deviceId, apiKey, plugins[0])

		s := SetupResponse{
			Status:     200,
			ApiKey:     apiKey,
			FriendlyID: "OLOLO",
			ImageURL:   "https://usetrmnl.com/images/setup/setup-logo.bmp",
			Message:    "Register at TRMNL GO",
		}
		msg, err := json.Marshal(s)
		if err != nil {
			log.Fatalf("Error occurred during marshalling: %s", err.Error())
		}

		if log_level == "debug" {
			log.Printf("DEBUG: setup responce %s \n", msg)
		}
		w.WriteHeader(200)
		w.Write([]byte(msg))
	})

	http.HandleFunc("/api/display", func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("Access-Token")
		deviceId := r.Header.Get("Id")
		voltage := r.Header.Get("Battery-Voltage")
		log.Printf("Rendering display for device: %s \n", r.Header.Get("Access-Token"))

		if log_level == "debug" {
			log.Printf("DEBUG: recieved headers from device %s \n", r.Header.Get("Access-Token"))
			for k, v := range r.Header {
				log.Printf("Header field %s, Value %s \n", k, v)
			}
		}

		msg := renderDisplay(port, deviceId, apiKey, voltage)

		if log_level == "debug" {
			log.Printf("DEBUG: display responce %s \n", msg)
		}

		w.WriteHeader(200)
		w.Write([]byte(msg))
	})

	http.HandleFunc("POST /api/log", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		r.Body.Close()
		if err != nil {
			log.Fatal(err)
		}

		log.Printf("Recieving logs from device: %s \n", r.Header.Get("Access-Token"))

		if log_level == "debug" || log_level == "info" {
			log.Printf("DEBUG: Logse %s \n", string(body))
		}
		w.WriteHeader(200)
		w.Write([]byte("OK"))
	})

	log.Printf("Branch: %s, CommitHash: %s, Version: %s \n", branch, commithash, version)
	log.Printf("HTTP server started on port %s \n", port)

	err := http.ListenAndServe(fmt.Sprintf(":%s", port), nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

func ServeFiles(w http.ResponseWriter, r *http.Request) {
	log.Printf("Requested file %s", r.RequestURI)
	p := "." + r.URL.Path
	http.ServeFile(w, r, p)
}

func main() {
	err := db.InitDB(dbname)
	if err != nil {
		log.Printf("FATAL: Failed to init DB %s", err)
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
	log.Printf("Services has shut down \n")
}

func UpdateData() {
	keys, _ := db.GetDeviceList(dbname)
	for {
		for _, key := range keys {
			prefix := fmt.Sprintf("public/%s", key)
			voltage, _ := db.GetDeviceVoltage(dbname, key)
			// stocks.RenderStocks(
			// 	"AAPL",
			// 	"72Q6JP7LLFX31QUY",
			// 	800,
			// 	480,
			// 	"public/stocks.png",
			// )
			// log.Printf("Update: Stock data \n")

			crypto.RenderScreenCrypto(
				800,
				480,
				"bitcoin",
				fmt.Sprintf("%s_crypto.png", prefix),
				voltage,
			)
			log.Printf("Update data for plugin: crypto \n")

			weather.RenderScreenWeather(
				800,
				480,
				"Wroclaw",
				fmt.Sprintf("%s_weather.png", prefix),
				voltage,
			)
			log.Printf("Update data for plugin: weather \n")

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
