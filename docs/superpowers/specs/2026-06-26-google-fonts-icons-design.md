# Design: Icons from Google Fonts (Material Symbols variable font)

**Date:** 2026-06-26
**Status:** Approved
**Component:** `pkg/v1/icons`, `pkg/v1/render`, weather plugin

## Goal

Stop shipping pre-rendered PNG icons in the repository (`assets/icons/`) and
stop downloading them from GitHub. Instead, fetch the **Material Symbols
Outlined** variable font from Google at runtime, cache it once, and render any
icon glyph on demand at a **caller-chosen pixel size**. This lets the same icon
be drawn at different sizes in different places.

## Non-goals

- Re-tuning icon positions. The hand-tuned negative-offset placements in
  `weather.go`/`render.go` are calibrated to today's PNG dimensions and will
  drift once icons become glyph-rendered and resizable. Repositioning is a
  follow-up pass, not part of this change.
- Tuning variable-font axes (weight, fill, grade, optical size). `x/image`
  renders the font's default instance (weight 400, FILL 0); that is accepted.
- Changing the screen-rotation, worker, or device flows.

## Approach

Chosen: **B-woff2** — fetch the variable font from Google's CSS2 API (modern
User-Agent → woff2), decode woff2 → SFNT in memory, render glyphs with the
existing `golang.org/x/image/font/opentype` stack.

Rejected alternatives:
- **SVG per-icon + rasterizer** — robust and arbitrary-size, but adds an SVG
  rasterizer dependency and uses per-icon files instead of reusing the font
  machinery.
- **B-ttf (legacy User-Agent trick)** — no new dependency, but relies on
  Google's unofficial User-Agent content-negotiation to serve TTF.
- **B-vendor (`go:embed` the TTF)** — bakes a ~3 MB font into the repo,
  contradicting the no-files-in-repo goal.

## Architecture

### `pkg/v1/icons` (rewritten)

Responsibilities:

1. **Name → glyph mapping.** Keep the existing semantic name constants
   (`Battery0`…`Battery100`, `WeatherCode*`, `Temperature*`, `Humidity*`,
   `Wind`, `WindGusts`) as the public API. Add an internal table mapping each
   name to its Material Symbols glyph codepoint (a Private-Use-Area rune).
2. **Font acquisition (lazy, on first `Render`).**
   1. `GET https://fonts.googleapis.com/css2?family=Material+Symbols+Outlined`
      with a modern browser `User-Agent` header → CSS containing a
      `fonts.gstatic.com` `.woff2` URL.
   2. Extract the `src: url(…woff2)` value via regex.
   3. Download the woff2 bytes.
   4. `font.ToSFNT(woff2)` (from `github.com/tdewolff/font`) → SFNT/TTF bytes.
   5. Write the TTF to the on-disk cache (`./icons/MaterialSymbols.ttf`).
   6. Parse once into an in-memory `*opentype.Font`.
   - On subsequent runs, read the cached TTF directly; no network access.
3. **Glyph rendering.** `Render(name string, size int) (image.Image, error)`:
   - Look up the codepoint for `name`.
   - Build a face: `opentype.NewFace(font, &opentype.FaceOptions{Size:
     float64(size), DPI: 72, Hinting: font.HintingFull})`.
   - Draw the glyph (black) onto a transparent `size×size` `*image.RGBA` with a
     `font.Drawer`. Material Symbols glyphs fill the em above the baseline
     (viewBox `0 -960 960 960`), so the baseline sits at the bottom of the box
     (`Dot ≈ (0, size)`); horizontal centering uses the glyph advance.
   - Return the image.

### `pkg/v1/render` (updated)

- `AddIcon(img *image.RGBA, name string, point image.Point, size int) error` —
  gains `size`; calls `icons.Render(name, size)` and composites the result with
  `draw.Over`. On error it keeps the current behavior: log a warning and skip
  the icon so rendering continues (first-run-offline renders without icons,
  exactly as today).
- `AddImageVoltage(img *image.RGBA, voltage float32, point image.Point, size
  int) error` — gains `size`, forwarded to `AddIcon`.

### Call sites

- `pkg/v1/plugins/weather/weather.go` — pass an explicit size to each
  `AddIcon`/`AddImageVoltage` call (preserving current visual sizes as the
  starting values).
- `pkg/v1/render/render.go` — the battery-voltage call passes a size.

## Name → Material Symbol mapping

Glyph codepoints are taken verbatim from Google's official
`MaterialSymbolsOutlined[...].codepoints` file at implementation time (pulled
with `curl`+`grep`, not transcribed by hand).

| Semantic name | Material Symbol |
|---|---|
| `battery0` | `battery_0_bar` |
| `battery20` | `battery_2_bar` |
| `battery40` | `battery_3_bar` |
| `battery60` | `battery_4_bar` |
| `battery80` | `battery_5_bar` |
| `battery100` | `battery_full` |
| `temperature` | `thermostat` |
| `temperaturehigh` | `keyboard_arrow_up` |
| `temperaturelow` | `keyboard_arrow_down` |
| `humiditylow` | `humidity_low` |
| `humiditymid` | `humidity_mid` |
| `humidityhigh` | `humidity_high` |
| `wind` | `air` |
| `windgusts` | `storm` |
| `weathercode0` | `sunny` |
| `weathercode1` | `partly_cloudy_day` |
| `weathercode3` | `cloud` |
| `weathercode4` | `foggy` |
| `weathercode5` | `rainy` |
| `weathercode7` | `rainy` |
| `weathercode8` | `rainy` |
| `weathercode77` | `snowing` |
| `weathercode85` | `snowing` |
| `weathercode9` | `thunderstorm` |

`temperaturehigh`/`temperaturelow`, `windgusts`, and the `rainy`/`snowing`
groupings are judgment calls (no exact 1:1 glyph exists); they are easy to
revise since they are only table entries. If a chosen glyph is absent from the
shipped font, implementation substitutes the nearest available name and records
it here.

## Caching & offline behavior

- Cache location stays `./icons/`, but it now holds one font file
  (`MaterialSymbols.ttf`) instead of per-icon PNGs.
- The parsed `*opentype.Font` is memoized in-process for the run.
- If font acquisition fails (e.g. offline first run), `Render` returns an error
  and `AddIcon` skips the icon — rendering of the rest of the screen continues.

## Files removed

- `assets/icons/*.png` (24 files).
- In `icons.go`: the GitHub `rawBaseURL` constant, the PNG-magic signature
  check, and the per-icon download/cache path logic.

## New dependency

- `github.com/tdewolff/font` — woff2 → SFNT decoding (`ToSFNT`). Glyph
  rasterization reuses the existing `golang.org/x/image/font/opentype` stack; no
  additional rendering dependency.

## Testing strategy

- **icons:**
  - Name → codepoint lookup returns the expected rune for known names and an
    error for unknown names.
  - Font acquisition: inject the CSS source URL and font bytes via overridable
    package vars (mirroring the current test-overridable `rawBaseURL`/`cacheDir`
    pattern); serve a small woff2/ttf fixture from `httptest` and assert the TTF
    is cached and parsed.
  - `Render(name, size)` returns a `size×size` image containing some
    non-transparent pixels for a known glyph.
- **render:** `AddIcon`/`AddImageVoltage` with the new `size` parameter
  composite onto the canvas; update existing tests for the new signatures.
- **weather:** update call-site tests for the new `AddIcon` signature.

## Risks / verification

- **Variable-font rendering via `x/image`.** Spike first: confirm a Material
  Symbols glyph rasterizes crisply at e-ink sizes (the default instance, weight
  400) before wiring all call sites.
- **CSS2 response shape.** Use a modern `User-Agent` so Google returns woff2;
  `font.ToSFNT` also accepts ttf/woff defensively if the response differs.
- **Icon placement drift.** Expected; addressed in a follow-up positioning pass
  (see Non-goals).
