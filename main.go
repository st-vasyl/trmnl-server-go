package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"trmnl-server-go/pkg/v1/plugins/bitcoin"
)

type Response struct {
	Status         int    `json:"status,omitempty"`
	ImageURL       string `json:"image_url,omitempty"`
	Filename       string `json:"filename"`
	UpdateFirmware bool   `json:"update_firmware"`
	FirmwareUrl    string `json:"firmware_url"`
	RefreshRate    int    `json:"refresh_rate"`
	ResetFirmware  bool   `json:"reset_firmware"`
}

func renderDisplay() (res []byte) {
	// apiKey := "72Q6JP7LLFX31QUY"
	// filename := "public/stocks.png"
	// stocks.RenderStocks(
	// 	"GOOG",
	// 	apiKey,
	// 	800,
	// 	480,
	// 	50,
	// 	50,
	// 	filename,
	// )

	filename := "public/bitcoin.png"
	bitcoin.RenderBitconin(
		800,
		480,
		50,
		50,
		filename,
	)
	r := Response{
		Status:         0,
		ImageURL:       "http://172.16.30.187:8080/" + filename,
		Filename:       "2024-09-20T00:00:00",
		UpdateFirmware: false,
		FirmwareUrl:    "",
		RefreshRate:    300,
		ResetFirmware:  false,
	}

	res, err := json.Marshal(r)
	if err != nil {
		log.Fatalf("Error occurred during marshalling: %s", err.Error())
	}
	return res
}

func Run(branch, commithash, version, port string) {
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		msg := fmt.Sprintf("App version: %s, commit: %s branch: %s ", version, commithash, branch)
		w.WriteHeader(200)
		w.Write([]byte(msg))
	})

	http.HandleFunc("/public/", ServeFiles)

	http.HandleFunc("/api/display", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Rendering display for device: %s \n", r.Header.Get("ID"))
		// for k, v := range r.Header {
		// 	log.Printf("Header field %s, Value %s \n", k, v)
		// }

		msg := renderDisplay()
		w.WriteHeader(200)
		// w.Header.Set("Content-Type", "application/json")
		w.Write([]byte(msg))
	})
	log.Printf("Branch: %s, CommitHash: %s, Version: %s \n", branch, commithash, version)
	log.Printf("HTTP server started on port %s \n", port)

	err := http.ListenAndServe(":8080", nil)
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
	Run("main", "00000", "0.0.1", "8080")
}
