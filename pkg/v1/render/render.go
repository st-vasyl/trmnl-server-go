package render

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"net/http"
	"os"
	"path/filepath"
	"trmnl-server-go/pkg/v1/icons"

	"github.com/rs/zerolog/log"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
	"gonum.org/v1/plot"
)

var cachedFont *opentype.Font

// SetFont parses and caches the TTF bytes for all subsequent AddText calls.
// Must be called once at startup before rendering begins.
func SetFont(ttfBytes []byte) error {
	f, err := opentype.Parse(ttfBytes)
	if err != nil {
		return err
	}
	cachedFont = f
	return nil
}

func getFont() (*opentype.Font, error) {
	if cachedFont == nil {
		return nil, fmt.Errorf("font not initialized: call render.SetFont first")
	}
	return cachedFont, nil
}

// sparseTicks is a plot.Ticker that places a labelled major tick only at the
// positions present in labels.
type sparseTicks struct{ labels map[float64]string }

func (s sparseTicks) Ticks(min, max float64) []plot.Tick {
	var ticks []plot.Tick
	for pos, label := range s.labels {
		if pos >= min && pos <= max {
			ticks = append(ticks, plot.Tick{Value: pos, Label: label})
		}
	}
	return ticks
}

// Generate an empty image with given width and height
func NewImage(width, height int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	white := color.RGBA{255, 255, 255, 255}
	draw.Draw(img, img.Bounds(), &image.Uniform{white}, image.Point{}, draw.Src)
	return img
}

// Add a text to the image with given string, start point and a font size
func AddText(img *image.RGBA, text string, point image.Point, col color.Color, fontSize float64) error {
	ttf, err := getFont()
	if err != nil {
		return err
	}

	face, err := opentype.NewFace(ttf, &opentype.FaceOptions{
		Size:    fontSize,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return err
	}

	drawer := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(col),
		Face: face,
		Dot: fixed.Point26_6{
			X: fixed.I(point.X),
			Y: fixed.I(point.Y),
		},
	}

	drawer.DrawString(Printable(text))

	return nil
}

// Write image changes to the file
// WriteFile stamps the battery icon, converts to grayscale and writes the PNG.
// The file is written to a temp name in the same directory and renamed into
// place, so a device downloading the previous PNG never sees a truncated or
// half-written file while the worker or an on-demand render replaces it.
func WriteFile(filename string, img *image.RGBA, voltage float32) error {
	if err := AddImageVoltage(img, voltage, image.Point{-750, -1}, 40); err != nil {
		return err
	}
	bw := ConvertToGray(img)

	f, err := os.CreateTemp(filepath.Dir(filename), filepath.Base(filename)+".*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	if err := png.Encode(f, bw); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	// CreateTemp uses 0600; keep the served files world-readable like Create did.
	if err := f.Chmod(0644); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, filename); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

func ConvertToGray(img *image.RGBA) *image.Gray {
	target := image.NewGray(img.Bounds())
	draw.Draw(target, target.Bounds(), img, img.Bounds().Min, draw.Src)
	return target
}

func AddImageFromBytes(img *image.RGBA, data []byte, point image.Point) error {
	srcImg, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return err
	}
	draw.Draw(img, img.Bounds(), srcImg, point, draw.Over)
	return nil
}

// AddIcon renders an icon by name at the given size and draws it at point. A
// failure to render the icon (e.g. offline on first run) is logged and skipped
// so rendering continues.
func AddIcon(img *image.RGBA, name string, point image.Point, size int) error {
	ic, err := icons.Render(name, size)
	if err != nil {
		log.Warn().Str("icon", name).Err(err).Msg("Skipping icon")
		return nil
	}
	draw.Draw(img, img.Bounds(), ic, point, draw.Over)
	return nil
}

func AddImageVoltage(img *image.RGBA, voltage float32, point image.Point, size int) error {
	batteryPercentage := ((voltage - 3) / 0.012)
	var batteryImage string

	switch {
	case batteryPercentage > 90.0:
		batteryImage = icons.Battery100
	case batteryPercentage > 70.0 && batteryPercentage <= 90.0:
		batteryImage = icons.Battery80
	case batteryPercentage > 50.0 && batteryPercentage <= 70.0:
		batteryImage = icons.Battery60
	case batteryPercentage > 30.0 && batteryPercentage <= 50.0:
		batteryImage = icons.Battery40
	case batteryPercentage > 10.0 && batteryPercentage <= 30.0:
		batteryImage = icons.Battery20
	case batteryPercentage <= 10.0:
		batteryImage = icons.Battery0
	}

	if err := AddIcon(img, batteryImage, point, size); err != nil {
		return err
	}

	return nil
}

func GetImageByUrl(url string) (image.Image, error) {
	r, err := http.Get(url)
	if err != nil {
		return nil, err
	}

	img, err := jpeg.Decode(r.Body)
	if err != nil {
		return nil, err
	}

	return img, nil
}
