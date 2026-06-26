package icons

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"os"
	"trmnl-server-go/pkg/v1/httpclient"

	"github.com/rs/zerolog/log"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
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

// codepoints maps each semantic icon name to its Material Symbols Outlined
// glyph (a Private-Use-Area rune). Values come from Google's official
// MaterialSymbolsOutlined.codepoints file.
var codepoints = map[string]rune{
	Battery0:   0xebdc, // battery_0_bar
	Battery20:  0xf09d, // battery_2_bar
	Battery40:  0xf09e, // battery_3_bar
	Battery60:  0xf09f, // battery_4_bar
	Battery80:  0xf0a0, // battery_5_bar
	Battery100: 0xe1a5, // battery_full

	WeatherCode0:  0xe81a, // sunny
	WeatherCode1:  0xf172, // partly_cloudy_day
	WeatherCode3:  0xf15c, // cloud
	WeatherCode4:  0xe818, // foggy
	WeatherCode5:  0xf176, // rainy
	WeatherCode7:  0xf176, // rainy
	WeatherCode77: 0xe80f, // snowing
	WeatherCode8:  0xf176, // rainy
	WeatherCode85: 0xe80f, // snowing
	WeatherCode9:  0xebdb, // thunderstorm

	Temperature:     0xf076, // thermostat
	TemperatureLow:  0xe313, // keyboard_arrow_down
	TemperatureHigh: 0xe316, // keyboard_arrow_up

	HumidityHigh: 0xf163, // humidity_high
	HumidityMid:  0xf165, // humidity_mid
	HumidityLow:  0xf164, // humidity_low

	Wind:      0xefd8, // air
	WindGusts: 0xf070, // storm
}

// Render rasterizes the named icon glyph at size×size pixels onto a transparent
// RGBA image (black glyph), suitable for compositing onto a canvas.
func Render(name string, size int) (image.Image, error) {
	r, ok := codepoints[name]
	if !ok {
		return nil, fmt.Errorf("unknown icon %q", name)
	}
	f, err := getFont()
	if err != nil {
		return nil, err
	}
	face, err := opentype.NewFace(f, &opentype.FaceOptions{
		Size:    float64(size),
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return nil, err
	}
	defer face.Close()

	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	d := font.Drawer{Dst: dst, Src: image.NewUniform(color.Black), Face: face}
	// Material Symbols glyphs fill the em above the baseline; place the baseline
	// at the bottom of the box and centre horizontally by advance width.
	adv := d.MeasureString(string(r))
	x := (fixed.I(size) - adv) / 2
	if x < 0 {
		x = 0
	}
	d.Dot = fixed.Point26_6{X: x, Y: fixed.I(size)}
	d.DrawString(string(r))
	return dst, nil
}

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
