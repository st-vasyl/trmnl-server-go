package icons

import (
	"bytes"
	"fmt"
	"os"
	"trmnl-server-go/pkg/v1/httpclient"

	"github.com/rs/zerolog/log"
)

// Icon name keys. Each value is the PNG filename stem, used both for the
// on-disk cache (./icons/<key>.png) and the remote source URL.
const (
	Battery0   = "battery0"
	Battery20  = "battery20"
	Battery40  = "battery40"
	Battery60  = "battery60"
	Battery80  = "battery80"
	Battery100 = "battery100"

	WeatherCode0  = "weathercode0"
	WeatherCode1  = "weathercode1"
	WeatherCode3  = "weathercode3"
	WeatherCode4  = "weathercode4"
	WeatherCode5  = "weathercode5"
	WeatherCode7  = "weathercode7"
	WeatherCode77 = "weathercode77"
	WeatherCode8  = "weathercode8"
	WeatherCode85 = "weathercode85"
	WeatherCode9  = "weathercode9"

	Temperature     = "temperature"
	TemperatureLow  = "temperaturelow"
	TemperatureHigh = "temperaturehigh"

	HumidityHigh = "humidityhigh"
	HumidityMid  = "humiditymid"
	HumidityLow  = "humiditylow"

	Wind      = "wind"
	WindGusts = "windgusts"
)

// Remote source and on-disk cache location. Overridden in tests.
var (
	rawBaseURL = "https://raw.githubusercontent.com/st-vasyl/trmnl-server-go/main/assets/icons/"
	cacheDir   = "./icons"
)

// pngMagic is the 8-byte PNG file signature.
var pngMagic = []byte("\x89PNG\r\n\x1a\n")

// Load returns PNG bytes for the given icon name.
// On first call it downloads the PNG and caches it at ./icons/{name}.png.
// Subsequent calls read from the cache without network access.
func Load(name string) ([]byte, error) {
	path := cachePath(name)

	if data, err := os.ReadFile(path); err == nil {
		return data, nil
	}

	log.Info().Str("icon", name).Msg("Downloading icon")

	data, err := httpclient.Get(rawBaseURL + name + ".png")
	if err != nil {
		return nil, fmt.Errorf("download icon %q: %w", name, err)
	}
	if !bytes.HasPrefix(data, pngMagic) {
		return nil, fmt.Errorf("download icon %q: response is not a PNG", name)
	}

	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("create icons dir: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		log.Warn().Str("path", path).Err(err).Msg("Failed to cache icon to disk")
	}

	log.Info().Str("icon", name).Str("path", path).Msg("Icon downloaded and cached")
	return data, nil
}

func cachePath(name string) string {
	return fmt.Sprintf("%s/%s.png", cacheDir, name)
}
