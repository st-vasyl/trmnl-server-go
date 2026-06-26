# Google Fonts (Material Symbols) Icons Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace GitHub-hosted, pre-rendered PNG icons with the Material Symbols Outlined variable font fetched from Google at runtime, rendered as glyphs at a caller-chosen pixel size.

**Architecture:** The `icons` package fetches Google's Material Symbols variable font once (CSS2 API → woff2 → decode to SFNT → cache `./icons/MaterialSymbols.ttf`), parses it with `x/image/font/opentype`, and exposes `Render(name, size)` which rasterizes a glyph (looked up by a semantic-name → codepoint table) onto a transparent `size×size` image. `render.AddIcon`/`AddImageVoltage` gain a `size` parameter and composite the rendered glyph.

**Tech Stack:** Go 1.25.5, `golang.org/x/image/font/opentype` (existing), `github.com/tdewolff/font` (new, woff2→SFNT), `gonum`/`bild` (existing, unaffected).

## Global Constraints

- Module: `trmnl-server-go`, Go 1.25.5.
- Public icon name constants (`Battery0`…`WindGusts`) stay unchanged — they remain the package's public API.
- New dependency allowed: `github.com/tdewolff/font` (woff2 decoding only).
- Font cache path: `./icons/MaterialSymbols.ttf` (the `./icons` dir is reused; it no longer holds PNGs).
- CSS2 request must send a modern browser `User-Agent` so Google returns woff2.
- Offline / fetch failure behavior is unchanged: `AddIcon` logs a warning and skips the icon so the rest of the screen still renders.
- Do NOT re-tune icon placement offsets in this plan; the negative-offset points in `weather.go`/`render.go` stay as-is. Repositioning is a separate follow-up.
- Keep the build and `go test ./...` green at the end of every task.

## File Structure

- `pkg/v1/icons/icons.go` (modified) — name constants, `codepoints` map, `Render(name, size)`. Old `Load`/PNG logic removed in Task 5.
- `pkg/v1/icons/font.go` (new) — font acquisition: `getFont`, `downloadFont`, overridable vars, in-memory memoization.
- `pkg/v1/icons/icons_test.go` (modified) — font-acquisition + Render tests; old `Load` tests removed in Task 5.
- `pkg/v1/render/render.go` (modified) — `AddIcon`/`AddImageVoltage` gain `size`.
- `pkg/v1/render/render_test.go` (modified) — seed font cache in `TestMain`; update signatures.
- `pkg/v1/plugins/weather/weather.go` (modified) — pass a size to each icon call.
- `assets/icons/*.png` (deleted in Task 5) — 24 files.
- `go.mod` / `go.sum` (modified) — add `github.com/tdewolff/font`.

## Codepoint Reference (verbatim from Google's official `.codepoints` file)

| Name const | Material Symbol | Rune |
|---|---|---|
| `Battery0` | battery_0_bar | `0xebdc` |
| `Battery20` | battery_2_bar | `0xf09d` |
| `Battery40` | battery_3_bar | `0xf09e` |
| `Battery60` | battery_4_bar | `0xf09f` |
| `Battery80` | battery_5_bar | `0xf0a0` |
| `Battery100` | battery_full | `0xe1a5` |
| `Temperature` | thermostat | `0xf076` |
| `TemperatureHigh` | keyboard_arrow_up | `0xe316` |
| `TemperatureLow` | keyboard_arrow_down | `0xe313` |
| `HumidityHigh` | humidity_high | `0xf163` |
| `HumidityMid` | humidity_mid | `0xf165` |
| `HumidityLow` | humidity_low | `0xf164` |
| `Wind` | air | `0xefd8` |
| `WindGusts` | storm | `0xf070` |
| `WeatherCode0` | sunny | `0xe81a` |
| `WeatherCode1` | partly_cloudy_day | `0xf172` |
| `WeatherCode3` | cloud | `0xf15c` |
| `WeatherCode4` | foggy | `0xe818` |
| `WeatherCode5` | rainy | `0xf176` |
| `WeatherCode7` | rainy | `0xf176` |
| `WeatherCode8` | rainy | `0xf176` |
| `WeatherCode77` | snowing | `0xe80f` |
| `WeatherCode85` | snowing | `0xe80f` |
| `WeatherCode9` | thunderstorm | `0xebdb` |

---

### Task 1: Font acquisition (`font.go`)

Fetch + cache + parse the Material Symbols font. The old `icons.go` (with `Load`) is left untouched so the build stays green.

**Files:**
- Create: `pkg/v1/icons/font.go`
- Modify: `pkg/v1/icons/icons_test.go` (add tests + helper; leave existing tests in place)
- Modify: `go.mod`, `go.sum`

**Interfaces:**
- Produces: `getFont() (*opentype.Font, error)`; overridable package vars `cssURL string`, `userAgent string`, `fontFileName string`, and memoization var `parsedFont *opentype.Font`. Reuses the existing `cacheDir` var declared in `icons.go`.

- [ ] **Step 1: Add the woff2 decoder dependency**

```bash
go get github.com/tdewolff/font@latest
```
Expected: `go.mod`/`go.sum` updated with `github.com/tdewolff/font`.

- [ ] **Step 2: Write the failing font-acquisition tests**

Add to `pkg/v1/icons/icons_test.go` (keep the existing imports/tests; add these imports if missing: `fmt`, `path/filepath` — `os`, `net/http`, `net/http/httptest`, `testing`, `bytes` are already imported):

```go
// readTestTTF loads the repo's font.ttf (a real TTF) to stand in for the
// downloaded font in tests. From this package's dir it is three levels up.
func readTestTTF(t *testing.T) []byte {
	t.Helper()
	ttf, err := os.ReadFile("../../../font.ttf")
	if err != nil {
		t.Skipf("font.ttf not available: %v", err)
	}
	return ttf
}

// withFontOverrides points font acquisition at a temp cache dir and clears the
// in-memory parsed font, restoring originals on cleanup.
func withFontOverrides(t *testing.T) string {
	t.Helper()
	origURL, origDir, origFont := cssURL, cacheDir, parsedFont
	dir := t.TempDir()
	cssURL, cacheDir, parsedFont = "", dir, nil
	t.Cleanup(func() { cssURL, cacheDir, parsedFont = origURL, origDir, origFont })
	return dir
}

func TestGetFont_DownloadsDecodesAndCaches(t *testing.T) {
	ttf := readTestTTF(t)
	mux := http.NewServeMux()
	mux.HandleFunc("/css", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "@font-face{src: url(http://%s/font) format('woff2');}", r.Host)
	})
	mux.HandleFunc("/font", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(ttf)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	dir := withFontOverrides(t)
	cssURL = srv.URL + "/css"

	f, err := getFont()
	if err != nil {
		t.Fatalf("getFont: %v", err)
	}
	if f == nil {
		t.Fatal("getFont returned nil font")
	}
	if _, err := os.Stat(filepath.Join(dir, "MaterialSymbols.ttf")); err != nil {
		t.Errorf("font not cached: %v", err)
	}
}

func TestGetFont_CacheHitSkipsNetwork(t *testing.T) {
	ttf := readTestTTF(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("network hit despite cache: %s", r.URL.Path)
	}))
	defer srv.Close()

	dir := withFontOverrides(t)
	cssURL = srv.URL
	if err := os.WriteFile(filepath.Join(dir, "MaterialSymbols.ttf"), ttf, 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := getFont(); err != nil {
		t.Fatalf("getFont: %v", err)
	}
}

func TestGetFont_NoFontURLReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("/* css with no url */"))
	}))
	defer srv.Close()

	withFontOverrides(t)
	cssURL = srv.URL

	if _, err := getFont(); err == nil {
		t.Fatal("expected error when CSS has no font url")
	}
}
```

- [ ] **Step 3: Run the tests to verify they fail to compile**

Run: `go test ./pkg/v1/icons/...`
Expected: FAIL — undefined: `cssURL`, `parsedFont`, `getFont`.

- [ ] **Step 4: Create `pkg/v1/icons/font.go`**

```go
package icons

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sync"

	"github.com/tdewolff/font"
	"github.com/rs/zerolog/log"
	"golang.org/x/image/font/opentype"
)

// Material Symbols delivery + cache. Overridable in tests.
var (
	cssURL       = "https://fonts.googleapis.com/css2?family=Material+Symbols+Outlined"
	userAgent    = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	fontFileName = "MaterialSymbols.ttf"
)

// In-memory parsed font, populated on first use.
var (
	fontMu     sync.Mutex
	parsedFont *opentype.Font
)

// fontURLRe extracts the first url(...) value from a CSS2 @font-face block.
var fontURLRe = regexp.MustCompile(`url\(\s*['"]?(https?://[^'")\s]+)['"]?\s*\)`)

func fontPath() string { return filepath.Join(cacheDir, fontFileName) }

// getFont returns the parsed Material Symbols font, fetching and caching it on
// first use. Subsequent calls reuse the in-memory or on-disk copy.
func getFont() (*opentype.Font, error) {
	fontMu.Lock()
	defer fontMu.Unlock()
	if parsedFont != nil {
		return parsedFont, nil
	}

	ttf, err := os.ReadFile(fontPath())
	if err != nil {
		log.Info().Msg("Downloading Material Symbols font")
		ttf, err = downloadFont()
		if err != nil {
			return nil, err
		}
		if err := os.MkdirAll(cacheDir, 0755); err != nil {
			return nil, fmt.Errorf("create icons dir: %w", err)
		}
		if err := os.WriteFile(fontPath(), ttf, 0644); err != nil {
			log.Warn().Str("path", fontPath()).Err(err).Msg("Failed to cache font to disk")
		} else {
			log.Info().Str("path", fontPath()).Msg("Icon font downloaded and cached")
		}
	}

	f, err := opentype.Parse(ttf)
	if err != nil {
		return nil, fmt.Errorf("parse icon font: %w", err)
	}
	parsedFont = f
	return parsedFont, nil
}

// downloadFont fetches the CSS2 stylesheet, extracts the font URL, downloads the
// font, and decodes it (woff2/woff/ttf) to SFNT/TTF bytes.
func downloadFont() ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, cssURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	css, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, err
	}

	m := fontURLRe.FindSubmatch(css)
	if m == nil {
		return nil, fmt.Errorf("no font url in css2 response")
	}

	fresp, err := http.Get(string(m[1]))
	if err != nil {
		return nil, err
	}
	raw, err := io.ReadAll(fresp.Body)
	fresp.Body.Close()
	if err != nil {
		return nil, err
	}

	ttf, err := font.ToSFNT(raw)
	if err != nil {
		return nil, fmt.Errorf("decode icon font: %w", err)
	}
	return ttf, nil
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./pkg/v1/icons/...`
Expected: PASS (existing `Load` tests still pass too).

- [ ] **Step 6: Commit**

```bash
git add pkg/v1/icons/font.go pkg/v1/icons/icons_test.go go.mod go.sum
git commit -m "Add Material Symbols font acquisition (fetch, decode, cache)

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01ADggkikuBYbwK29mHnn8HY"
```

---

### Task 2: Glyph rendering (`Render` + codepoints)

**Files:**
- Modify: `pkg/v1/icons/icons.go`
- Modify: `pkg/v1/icons/icons_test.go`

**Interfaces:**
- Consumes: `getFont()` (Task 1).
- Produces: `Render(name string, size int) (image.Image, error)` and the unexported `codepoints map[string]rune`.

- [ ] **Step 1: Write the failing Render tests**

Add to `pkg/v1/icons/icons_test.go` (add imports `image` is already present; add `image/color` only if you assert color — not needed here):

```go
func TestRender_UnknownIconReturnsError(t *testing.T) {
	if _, err := Render("definitely-not-an-icon", 24); err == nil {
		t.Fatal("expected error for unknown icon name")
	}
}

func TestRender_DrawsGlyphPixels(t *testing.T) {
	ttf := readTestTTF(t)
	dir := withFontOverrides(t)
	if err := os.WriteFile(filepath.Join(dir, "MaterialSymbols.ttf"), ttf, 0644); err != nil {
		t.Fatal(err)
	}
	// Map a temporary name to a glyph that exists in font.ttf ('A').
	codepoints["__test_glyph__"] = 'A'
	t.Cleanup(func() { delete(codepoints, "__test_glyph__") })

	img, err := Render("__test_glyph__", 48)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	b := img.Bounds()
	if b.Dx() != 48 || b.Dy() != 48 {
		t.Fatalf("bounds = %v, want 48x48", b)
	}
	rgba, ok := img.(*image.RGBA)
	if !ok {
		t.Fatalf("image type = %T, want *image.RGBA", img)
	}
	drawn := false
	for i := 3; i < len(rgba.Pix) && !drawn; i += 4 {
		if rgba.Pix[i] != 0 { // any non-transparent pixel
			drawn = true
		}
	}
	if !drawn {
		t.Error("Render produced a fully transparent image")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./pkg/v1/icons/... -run TestRender`
Expected: FAIL — undefined: `Render`, `codepoints`.

- [ ] **Step 3: Add `codepoints` and `Render` to `pkg/v1/icons/icons.go`**

Add these imports to `icons.go`'s import block: `"image"`, `"image/color"`, `"golang.org/x/image/font"`, `"golang.org/x/image/font/opentype"`, `"golang.org/x/image/math/fixed"`. (Keep the existing imports for now; they are removed in Task 5.)

Add after the name constants:

```go
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
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./pkg/v1/icons/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/v1/icons/icons.go pkg/v1/icons/icons_test.go
git commit -m "Render icon glyphs from the Material Symbols font by size

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01ADggkikuBYbwK29mHnn8HY"
```

---

### Task 3: `render` package — size-aware icon drawing

**Files:**
- Modify: `pkg/v1/render/render.go:274-309` (`AddIcon`, `AddImageVoltage`) and the caller at `render.go:123`
- Modify: `pkg/v1/render/render_test.go`

**Interfaces:**
- Consumes: `icons.Render(name, size)` (Task 2).
- Produces: `AddIcon(img *image.RGBA, name string, point image.Point, size int) error`; `AddImageVoltage(img *image.RGBA, voltage float32, point image.Point, size int) error`.

- [ ] **Step 1: Update `render_test.go` — seed the font cache and fix signatures**

In `TestMain` (`render_test.go:20-30`), after the successful `SetFont` branch, seed the icon font cache so `AddIcon` resolves offline; clean it up after the run. Replace the function body with:

```go
func TestMain(m *testing.M) {
	ttf, err := os.ReadFile("../../../font.ttf")
	if err != nil {
		os.Stderr.WriteString("render tests: could not read ../../../font.ttf: " + err.Error() + "\n")
	} else {
		if err := SetFont(ttf); err != nil {
			os.Stderr.WriteString("render tests: SetFont failed: " + err.Error() + "\n")
		}
		// Seed the icon font cache (./icons/MaterialSymbols.ttf) so AddIcon
		// renders without any network access. font.ttf lacks the Material
		// Symbols glyphs, so icons draw empty — AddIcon must still not error.
		_ = os.MkdirAll("icons", 0755)
		_ = os.WriteFile(filepath.Join("icons", "MaterialSymbols.ttf"), ttf, 0644)
	}
	code := m.Run()
	os.RemoveAll("icons")
	os.Exit(code)
}
```

Delete the `seedIconCache` helper (`render_test.go:144-158`). Keep `smallPNG` (still used by the `AddImageFromBytes` tests).

Replace `TestAddIcon_DrawsCachedIcon` (`render_test.go:174-180`) with:

```go
func TestAddIcon_DrawsWithoutError(t *testing.T) {
	img := NewImage(800, 480)
	if err := AddIcon(img, icons.Wind, image.Point{0, 0}, 48); err != nil {
		t.Fatalf("AddIcon: %v", err)
	}
}
```

Replace the body of `TestAddImageVoltage_DispatchesByThreshold` (`render_test.go:182-202`) — drop the `seedIconCache(...)` call and add a size to `AddImageVoltage`:

```go
func TestAddImageVoltage_DispatchesByThreshold(t *testing.T) {
	voltages := []float32{4.20, 4.00, 3.80, 3.55, 3.20, 3.00, 2.50}
	for _, v := range voltages {
		img := NewImage(800, 480)
		if err := AddImageVoltage(img, v, image.Point{-750, -5}, 40); err != nil {
			t.Errorf("voltage %v: %v", v, err)
		}
	}
}
```

- [ ] **Step 2: Run tests to verify they fail to compile**

Run: `go test ./pkg/v1/render/...`
Expected: FAIL — too many arguments to `AddIcon`/`AddImageVoltage` (signatures not yet updated).

- [ ] **Step 3: Update `AddIcon` and `AddImageVoltage` in `render.go`**

Replace `AddIcon` (`render.go:274-283`) with:

```go
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
```

In `AddImageVoltage` (`render.go:285-309`), change the signature and the `AddIcon` call:

```go
func AddImageVoltage(img *image.RGBA, voltage float32, point image.Point, size int) error {
```

and

```go
	if err := AddIcon(img, batteryImage, point, size); err != nil {
		return err
	}
```

Update the in-package caller at `render.go:123`:

```go
	if err := AddImageVoltage(img, voltage, image.Point{-750, -5}, 40); err != nil {
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./pkg/v1/render/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/v1/render/render.go pkg/v1/render/render_test.go
git commit -m "Make render.AddIcon/AddImageVoltage size-aware via glyph rendering

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01ADggkikuBYbwK29mHnn8HY"
```

---

### Task 4: Weather plugin call sites — pass sizes

**Files:**
- Modify: `pkg/v1/plugins/weather/weather.go:142-178`

**Interfaces:**
- Consumes: `render.AddIcon(img, name, point, size)` (Task 3).

Starting sizes (placement is unchanged; these can be tuned later): main weather icon `150`, all adornment icons `50`.

- [ ] **Step 1: Add a size argument to each `AddIcon` call**

Edit the seven calls in `renderScreen` so each ends with a size:

```go
	if err := render.AddIcon(img, weatherIconByCode(w.Current.WeatherCode), image.Point{-50, 0}, 150); err != nil {
```
```go
	if err := render.AddIcon(img, icons.Temperature, image.Point{-293, -20}, 50); err != nil {
```
```go
	if err := render.AddIcon(img, icons.TemperatureHigh, image.Point{-300, -70}, 50); err != nil {
```
```go
	if err := render.AddIcon(img, icons.TemperatureLow, image.Point{-300, -120}, 50); err != nil {
```
```go
	if err := render.AddIcon(img, humidityIcon(w.Current.RelativeHumidity2m), image.Point{-530, -20}, 50); err != nil {
```
```go
	if err := render.AddIcon(img, icons.Wind, image.Point{-530, -70}, 50); err != nil {
```
```go
	if err := render.AddIcon(img, icons.WindGusts, image.Point{-530, -120}, 50); err != nil {
```

- [ ] **Step 2: Build and test**

Run: `go build ./... && go test ./pkg/v1/plugins/weather/...`
Expected: build succeeds; weather tests PASS (they don't exercise rendering).

- [ ] **Step 3: Commit**

```bash
git add pkg/v1/plugins/weather/weather.go
git commit -m "Pass icon sizes from the weather plugin

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01ADggkikuBYbwK29mHnn8HY"
```

---

### Task 5: Remove dead PNG-icon code and assets

**Files:**
- Modify: `pkg/v1/icons/icons.go` (remove `Load`, `rawBaseURL`, `pngMagic`, `cachePath`, and now-unused imports)
- Modify: `pkg/v1/icons/icons_test.go` (remove old `Load` tests + `withOverrides` + `samplePNG`)
- Delete: `assets/icons/*.png`
- Modify: `go.mod`/`go.sum` via `go mod tidy`

- [ ] **Step 1: Remove the old Load tests from `icons_test.go`**

Delete `TestLoad_CacheHitSkipsNetwork`, `TestLoad_DownloadsAndCaches`, `TestLoad_NonPNGResponseReturnsErrorAndDoesNotCache`, `TestLoad_NetworkErrorReturnsError`, and the helpers `withOverrides` and `samplePNG`. Remove any imports left unused (e.g. `image/png` if no longer referenced; `bytes` is still used by Render tests' nothing — verify and drop unused imports so the file compiles).

- [ ] **Step 2: Remove `Load` and PNG plumbing from `icons.go`**

Delete the `Load` function, `cachePath`, the `rawBaseURL` var, and the `pngMagic` var. Keep `cacheDir` (now used by `font.go`). Update the import block to drop `bytes`, `os`, and `trmnl-server-go/pkg/v1/httpclient` if they are no longer referenced (they are used only by the removed code). The resulting `icons.go` imports should be: `fmt`, `image`, `image/color`, `golang.org/x/image/font`, `golang.org/x/image/font/opentype`, `golang.org/x/image/math/fixed`, plus `github.com/rs/zerolog/log` only if still referenced (it is not after `Load` is removed — drop it). The `cacheDir` var declaration stays in `icons.go`:

```go
// cacheDir is the on-disk cache location for the downloaded icon font.
// Overridden in tests.
var cacheDir = "./icons"
```

- [ ] **Step 3: Delete the committed PNG assets**

```bash
git rm assets/icons/*.png
```

- [ ] **Step 4: Tidy modules and verify the whole build + tests**

```bash
go mod tidy
go build ./...
go test ./...
```
Expected: build succeeds; all packages PASS.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "Remove GitHub PNG icons and dead Load path

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01ADggkikuBYbwK29mHnn8HY"
```

---

### Task 6: End-to-end verification spike

Confirm a real Material Symbols glyph renders crisply (validates the variable-font default-instance assumption from the spec). This task is exploratory and may be reverted.

**Files:**
- Create (temporary): `pkg/v1/icons/spike_test.go`

- [ ] **Step 1: Write a network-gated spike test that renders a real icon and saves a PNG**

```go
package icons

import (
	"image/png"
	"os"
	"testing"
)

// TestSpike_RealGlyph hits the live Google endpoint; run explicitly with
// `go test ./pkg/v1/icons/ -run TestSpike_RealGlyph -tags spike` style intent.
func TestSpike_RealGlyph(t *testing.T) {
	if os.Getenv("ICON_SPIKE") == "" {
		t.Skip("set ICON_SPIKE=1 to run the live render spike")
	}
	img, err := Render(WeatherCode9, 120) // thunderstorm
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	f, err := os.Create("spike_thunderstorm.png")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	t.Log("wrote spike_thunderstorm.png — inspect it visually")
}
```

- [ ] **Step 2: Run the spike against the live endpoint**

Run: `ICON_SPIKE=1 go test ./pkg/v1/icons/ -run TestSpike_RealGlyph -v`
Expected: PASS; `pkg/v1/icons/spike_thunderstorm.png` is written. Open it and confirm a recognizable, crisp thunderstorm glyph.

- [ ] **Step 3: Clean up the spike**

```bash
rm pkg/v1/icons/spike_test.go pkg/v1/icons/spike_thunderstorm.png
rm -f icons/MaterialSymbols.ttf  # remove cache created by the live run, if present
```

No commit (spike removed). If the glyph looked wrong, stop and revisit the FaceOptions/baseline math before considering the feature done.

---

## Self-Review

**Spec coverage:**
- Runtime fetch from Google (no GitHub) → Task 1 (`downloadFont`).
- woff2 decode via `tdewolff/font` → Task 1 (`font.ToSFNT`).
- Cache font once at `./icons/MaterialSymbols.ttf` → Task 1 (`getFont`/`fontPath`).
- Per-call configurable size → Tasks 2–4 (`Render(name, size)`, size args).
- Semantic names kept as public API + codepoint mapping → Task 2 (`codepoints`).
- Offline/failure skips icon → Task 3 (`AddIcon` warn+continue).
- Remove repo PNGs + dead code → Task 5.
- Variable-font verification → Task 6 spike.
- Non-goal (no repositioning) honored: points unchanged in Task 4.

**Placeholder scan:** No TBD/TODO; every code step has complete code; codepoints are concrete hex values.

**Type consistency:** `Render(name string, size int) (image.Image, error)` is defined in Task 2 and consumed identically in Task 3. `AddIcon(..., size int)` / `AddImageVoltage(..., size int)` defined in Task 3 and called with a size in Tasks 3–4. `getFont() (*opentype.Font, error)` defined in Task 1, consumed in Task 2. `cacheDir` is shared between `icons.go` and `font.go` (declared once in `icons.go`).
