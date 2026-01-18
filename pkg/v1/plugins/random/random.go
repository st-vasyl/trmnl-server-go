package random

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type Result struct {
	Urls []Urls `json:"urls"`
}

type Urls struct {
	Full    string  `json:"full"`
	Regular float64 `json:"regular"`
	Small   float64 `json:"small"`
	Thumb   string  `json:"thumb"`
}

func getRandomImage(api_key string) (Result, error) {
	var res Result

	r, err := http.Get("https://api.unsplash.com/photos/random")
	r.Header.Set("Accept", "application/json")
	r.Header.Set("Accept-Language", "en-US")
	r.Header.Set("Authorization", fmt.Sprintf("Client-ID %s", api_key))
	if err != nil {
		return res, err
	}
	body, err := io.ReadAll(r.Body)
	r.Body.Close()
	if err != nil {
		return res, err
	}

	err = json.Unmarshal([]byte(body), &res)
	if err != nil {
		panic(err)
	}
	return res, nil
}
