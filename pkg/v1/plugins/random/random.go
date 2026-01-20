package random

import (
	"encoding/json"
	"fmt"
	"image"
	"io"
	"log"
	"net/http"
	"trmnl-server-go/pkg/v1/render"

	"github.com/anthonynsimon/bild/transform"
)

type Result struct {
	Urls Urls `json:"urls"`
}

type Urls struct {
	Full    string `json:"full"`
	Regular string `json:"regular"`
	Small   string `json:"small"`
	Thumb   string `json:"thumb"`
}

var orientation = "landscape"
var topics = "nature"
var query = "black&white"

func getImageUrl(api_key string) (string, error) {
	var res Result
	url := fmt.Sprintf("https://api.unsplash.com/photos/random?client_id=%s&orientation=%s&topics=%s&query=%s", api_key, orientation, topics, query)
	r, err := http.Get(url)
	r.Header.Set("Accept", "application/json")
	r.Header.Set("Accept-Language", "en-US")

	if err != nil {
		return "", err
	}
	body, err := io.ReadAll(r.Body)
	r.Body.Close()
	if err != nil {
		return "", err
	}

	err = json.Unmarshal([]byte(body), &res)
	if err != nil {
		panic(err)
	}

	return res.Urls.Regular, nil
}

func RenderRandomImage(width, height int, api_key, filename, log_level string, voltage float32) error {
	url, err := getImageUrl(api_key)
	if err != nil {
		return err
	}
	if log_level == "debug" {
		log.Printf("DEBUG: Random image URL %s \n", url)
	}

	im, err := render.GetImageByUrl(url)
	if err != nil {
		return err
	}

	img := transform.Crop(im, image.Rect(0, 0, width, height))

	if err := render.WriteFile(filename, img, voltage); err != nil {
		return err
	}
	return nil
}
