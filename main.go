package main

import (
	"encoding/json"
	"fmt"
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
	port   = "8080"
	dbname = "./trmnl.db"
)

var plugins = []string{"crypto", "weather"}
var apiKey = "xxxxxxxxxx"

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

func renderDisplay(port, apiKey string) (res []byte) {
	screen, err := db.GetDevice(dbname, apiKey)
	filename := fmt.Sprintf("public/%s.png", screen)
	r := DisplayResponse{
		Status:         0,
		ImageURL:       fmt.Sprintf("http://172.16.30.187:%s/%s", port, filename),
		Filename:       "2024-09-20T00:00:00",
		UpdateFirmware: false,
		FirmwareUrl:    "",
		RefreshRate:    300,
		ResetFirmware:  false,
	}
	res, err = json.Marshal(r)
	if err != nil {
		log.Fatalf("Error occurred during marshalling: %s", err.Error())
	}
	nextScreen := getNextScreen(screen)
	err = db.UpdateDevice(dbname, apiKey, nextScreen)
	return res
}

func HandleHTTP(branch, commithash, version, port string) {
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		msg := fmt.Sprintf("App version: %s, commit: %s branch: %s ", version, commithash, branch)
		w.WriteHeader(200)
		w.Write([]byte(msg))
	})

	http.HandleFunc("/api/setup", func(w http.ResponseWriter, r *http.Request) {
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
		w.WriteHeader(200)
		w.Write([]byte(msg))
		db.RegisterDevice(dbname, apiKey, plugins[0])
	})

	http.HandleFunc("/public/", ServeFiles)

	http.HandleFunc("/api/display", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Rendering display for device: %s \n", r.Header.Get("Access-Token"))
		msg := renderDisplay(port, apiKey)
		w.WriteHeader(200)
		w.Write([]byte(msg))
	})
	log.Printf("Branch: %s, CommitHash: %s, Version: %s \n", branch, commithash, version)
	log.Printf("HTTP server started on port %s \n", port)

	err := http.ListenAndServe(fmt.Sprintf(":%s", port), nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

func ServeFiles(w http.ResponseWriter, r *http.Request) {
	log.Printf("Requested file %s from %s", r.URL.Path, r.Form)
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
	for {
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
			"public/crypto.png",
		)
		log.Printf("Update: Crypto data \n")

		weather.RenderScreenWeather(
			800,
			480,
			"Wroclaw",
			"public/weather.png",
		)
		log.Printf("Update: weather data \n")
		time.Sleep(900 * time.Second)
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
