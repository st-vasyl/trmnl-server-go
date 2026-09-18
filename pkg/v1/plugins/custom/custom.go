// Package custom shows images that another system has already rendered. Each
// configured URL becomes one screen in the device rotation; the server
// downloads the image, fits it onto the 800×480 canvas and writes it through
// the same pipeline as every other plugin.
package custom

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
	"trmnl-server-go/pkg/v1/render"

	// Register the decoders image.Decode can use for the downloaded bytes.
	_ "image/jpeg"
	_ "image/png"

	"github.com/rs/zerolog/log"
	xdraw "golang.org/x/image/draw"
)

const (
	pluginName = "custom"

	screenWidth  = 800
	screenHeight = 480
)

// cacheTTL is how long a downloaded image is reused before it is fetched
// again. The worker renders every device within seconds of each other, so one
// minute means one download per tick shared by all devices, while the next
// tick (update_time is normally far longer) always sees a fresh image.
// Overridden in tests.
var cacheTTL = time.Minute

// maxBodyBytes caps a download. A screen-sized PNG is tens of kilobytes and a
// photo JPEG a few megabytes, so anything larger is a misconfigured URL.
// Overridden in tests.
var maxBodyBytes int64 = 16 << 20

// client bounds every download. Render runs under the global render mutex,
// so a hung server must not be allowed to stall every other plugin.
// Overridden in tests.
var client = &http.Client{Timeout: 30 * time.Second}

// Plugin renders one screen per configured image URL.
type Plugin struct {
	urls []string

	mu    sync.Mutex
	cache map[string]cached
}

// cached is one downloaded image and when it was fetched.
type cached struct {
	img       image.Image
	fetchedAt time.Time
}

// New validates the configured URLs and returns the plugin. It fails on an
// empty list or on any entry that is not an http or https URL with a host.
func New(urls []string) (*Plugin, error) {
	if len(urls) == 0 {
		return nil, fmt.Errorf("custom: no urls configured")
	}
	for i, raw := range urls {
		u, err := url.Parse(raw)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return nil, fmt.Errorf("custom: url %d %q must start with http:// or https://", i+1, raw)
		}
	}
	return &Plugin{urls: urls, cache: make(map[string]cached)}, nil
}

func (p *Plugin) Name() string { return pluginName }

// Screens returns "custom_1" through "custom_N" in configured order.
func (p *Plugin) Screens() []string {
	names := make([]string, len(p.urls))
	for i := range p.urls {
		names[i] = fmt.Sprintf("%s_%d", pluginName, i+1)
	}
	return names
}

// screenIndex maps a screen name such as "custom_3" to its zero-based index.
func screenIndex(screen string) (int, error) {
	num, ok := strings.CutPrefix(screen, pluginName+"_")
	if !ok {
		return 0, fmt.Errorf("custom: unknown screen %q", screen)
	}
	n, err := strconv.Atoi(num)
	if err != nil || n < 1 {
		return 0, fmt.Errorf("custom: unknown screen %q", screen)
	}
	return n - 1, nil
}

// Render downloads the screen's image, places it on the canvas and writes the
// PNG to outputPath. Nothing is written when the download or decode fails, so
// the previous image keeps serving.
func (p *Plugin) Render(screen, outputPath string, voltage float32) error {
	i, err := screenIndex(screen)
	if err != nil {
		return err
	}
	if i >= len(p.urls) {
		return fmt.Errorf("custom: screen %q is not configured (have %d)", screen, len(p.urls))
	}

	src, err := p.image(p.urls[i])
	if err != nil {
		return fmt.Errorf("custom: %s: %w", screen, err)
	}

	canvas := render.NewImage(screenWidth, screenHeight)
	placeOnCanvas(canvas, src)
	return render.WriteFile(outputPath, canvas, voltage)
}

// placeOnCanvas draws src onto the canvas. An image that already matches the
// screen is copied as is; anything else is scaled to fit, keeping its aspect
// ratio, and centered on the white background.
func placeOnCanvas(canvas *image.RGBA, src image.Image) {
	b := src.Bounds()
	if b.Dx() == screenWidth && b.Dy() == screenHeight {
		draw.Draw(canvas, canvas.Bounds(), src, b.Min, draw.Over)
		return
	}
	scale := min(float64(screenWidth)/float64(b.Dx()), float64(screenHeight)/float64(b.Dy()))
	w := int(float64(b.Dx())*scale + 0.5)
	h := int(float64(b.Dy())*scale + 0.5)
	x := (screenWidth - w) / 2
	y := (screenHeight - h) / 2
	xdraw.CatmullRom.Scale(canvas, image.Rect(x, y, x+w, y+h), src, b, draw.Over, nil)
}

// image returns the decoded image for u, downloading it unless a copy younger
// than cacheTTL is cached. Renders for several devices in one worker tick
// therefore share a single download.
func (p *Plugin) image(u string) (image.Image, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if c, ok := p.cache[u]; ok && time.Since(c.fetchedAt) < cacheTTL {
		return c.img, nil
	}
	img, err := fetchImage(u)
	if err != nil {
		return nil, err
	}
	if b := img.Bounds(); b.Dx() != screenWidth || b.Dy() != screenHeight {
		log.Warn().Str("url", u).Int("width", b.Dx()).Int("height", b.Dy()).
			Msgf("Custom image is not %dx%d, scaling to fit", screenWidth, screenHeight)
	}
	p.cache[u] = cached{img: img, fetchedAt: time.Now()}
	return img, nil
}

// fetchImage downloads and decodes one image.
func fetchImage(u string) (image.Image, error) {
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "image/png, image/jpeg;q=0.9, image/*;q=0.8")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", u, resp.Status)
	}

	// Read one byte past the cap so an oversized body is detected instead of
	// silently truncated into a decode error.
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > maxBodyBytes {
		return nil, fmt.Errorf("GET %s: body is too large (over %d bytes)", u, maxBodyBytes)
	}
	img, _, err := image.Decode(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", u, err)
	}
	return img, nil
}
