package icons

import (
	"fmt"
	"image"
	"image/color"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// Icon name keys. Each maps to a Material Symbols glyph via the codepoints
// table below.
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
	Battery0:   0xf306, // battery_0_bar
	Battery20:  0xf256, // battery_2_bar
	Battery40:  0xf254, // battery_3_bar
	Battery60:  0xf253, // battery_4_bar
	Battery80:  0xf252, // battery_5_bar
	Battery100: 0xf304, // battery_full

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
	TemperatureLow:  0xf37a, // keyboard_arrow_down
	TemperatureHigh: 0xf379, // keyboard_arrow_up

	HumidityHigh: 0xf163, // humidity_high
	HumidityMid:  0xf165, // humidity_mid
	HumidityLow:  0xf164, // humidity_low

	Wind:      0xefd8, // wind
	WindGusts: 0xec0c, // windGusts
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

// cacheDir is the on-disk cache location for the downloaded icon font.
// Overridden in tests.
var cacheDir = "./icons"
