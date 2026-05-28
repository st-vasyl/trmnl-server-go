package random

import (
	"encoding/json"
	"fmt"
	"image"
	"trmnl-server-go/pkg/v1/httpclient"
	"trmnl-server-go/pkg/v1/render"

	"github.com/anthonynsimon/bild/transform"
	"github.com/rs/zerolog/log"
)

// baseURL is the Unsplash API root. Overridden in tests.
var baseURL = "https://api.unsplash.com"

// RandomPlugin fetches a random landscape nature photo from Unsplash.
type RandomPlugin struct {
	ApiKey string
}

func (p *RandomPlugin) Name() string      { return "random" }
func (p *RandomPlugin) Screens() []string { return []string{"random"} }
func (p *RandomPlugin) Render(_ string, outputPath string, voltage float32) error {
	return renderScreen(p.ApiKey, outputPath, voltage)
}

type result struct {
	Urls urls `json:"urls"`
}

type urls struct {
	Full    string `json:"full"`
	Regular string `json:"regular"`
	Small   string `json:"small"`
	Thumb   string `json:"thumb"`
}

func getImageURL(apiKey string) (string, error) {
	url := fmt.Sprintf("%s/photos/random?client_id=%s&orientation=landscape&topics=nature&query=black%%26white", baseURL, apiKey)
	body, err := httpclient.Get(url)
	if err != nil {
		return "", err
	}
	var res result
	if err := json.Unmarshal(body, &res); err != nil {
		return "", err
	}
	return res.Urls.Regular, nil
}

func renderScreen(apiKey, outputPath string, voltage float32) error {
	url, err := getImageURL(apiKey)
	if err != nil {
		return err
	}
	log.Debug().Str("plugin", "random").Str("url", url).Msg("Fetched image URL")

	im, err := render.GetImageByUrl(url)
	if err != nil {
		return err
	}

	img := transform.Crop(im, image.Rect(0, 0, 800, 480))
	rgbaImg := render.NewImage(800, 480)
	for y := 0; y < 480; y++ {
		for x := 0; x < 800; x++ {
			rgbaImg.Set(x, y, img.At(x, y))
		}
	}
	return render.WriteFile(outputPath, rgbaImg, voltage)
}
