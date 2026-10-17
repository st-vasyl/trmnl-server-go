// Package gallery shows a random picture from a local folder of ready-made
// PNGs. It adds one screen to the rotation; every time the worker draws it a
// new image is picked, so the picture changes once per update_time.
package gallery

import (
	"fmt"
	"image"
	"image/png"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"trmnl-server-go/pkg/v1/render"

	"github.com/rs/zerolog/log"
)

const (
	pluginName = "gallery"

	screenWidth  = 800
	screenHeight = 480
)

// randIntN picks the index of the image to show. Overridden in tests.
var randIntN = rand.IntN

// Plugin renders a single screen with a random image from folder.
type Plugin struct {
	folder string
}

// New checks that folder can be read and holds at least one PNG, so a typo in
// the config stops the server at startup instead of failing every render.
func New(folder string) (*Plugin, error) {
	if folder == "" {
		return nil, fmt.Errorf("gallery: no folder configured")
	}
	if _, err := listImages(folder); err != nil {
		return nil, fmt.Errorf("gallery: %w", err)
	}
	return &Plugin{folder: folder}, nil
}

func (p *Plugin) Name() string { return pluginName }

// Screens returns the one "gallery" screen, however many images the folder holds.
func (p *Plugin) Screens() []string { return []string{pluginName} }

// Render picks a random PNG from the folder and writes it to outputPath. The
// folder is read again on every call, so images added or removed while the
// server runs are picked up on the next render. Nothing is written when the
// folder is empty or the picked file does not decode, so the previous image
// keeps serving.
func (p *Plugin) Render(screen, outputPath string, voltage float32) error {
	if screen != pluginName {
		return fmt.Errorf("gallery: unknown screen %q", screen)
	}
	names, err := listImages(p.folder)
	if err != nil {
		return fmt.Errorf("gallery: %w", err)
	}
	path := filepath.Join(p.folder, names[randIntN(len(names))])
	log.Debug().Str("image", path).Msg("Gallery picked image")

	src, err := readPNG(path)
	if err != nil {
		return fmt.Errorf("gallery: %w", err)
	}
	if b := src.Bounds(); b.Dx() != screenWidth || b.Dy() != screenHeight {
		log.Warn().Str("image", path).Int("width", b.Dx()).Int("height", b.Dy()).
			Msgf("Gallery image is not %dx%d, scaling to fit", screenWidth, screenHeight)
	}

	canvas := render.NewImage(screenWidth, screenHeight)
	render.AddImageFit(canvas, src)
	return render.WriteFile(outputPath, canvas, voltage)
}

// listImages returns the names of the .png files in folder (any case). Hidden
// files are skipped, which covers .DS_Store and the ._name.png companions
// macOS leaves when copying to network shares and USB drives.
func listImages(folder string) ([]string, error) {
	entries, err := os.ReadDir(folder)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || strings.HasPrefix(name, ".") || !strings.EqualFold(filepath.Ext(name), ".png") {
			continue
		}
		names = append(names, name)
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("no PNG images in %s", folder)
	}
	return names, nil
}

// readPNG opens and decodes one image file.
func readPNG(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	return img, nil
}
