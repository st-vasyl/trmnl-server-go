# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

TRMNL Server Go is a backend server for TRMNL e-ink display devices (800×480px). It registers devices, fetches live data via plugins, renders PNG images, and serves them to devices on a schedule.

## Commands

```bash
# Run the server
go run main.go

# Build binary
go build -o server main.go

# Run tests
go test ./...

# Run a single package's tests
go test ./pkg/v1/render/...
```

No Makefile. Config is loaded from `config.yaml` (copy `example_config.yaml` to get started).

## Architecture

**Two goroutines:**
- **HTTP handler** (`pkg/v1/handler/`) — device registration (`/api/setup`), image serving (`/api/display`), log ingestion (`/api/log`), static files (`/public/*`).
- **Worker** (`pkg/v1/worker/`) — background loop that refreshes all plugin screens at `update_time` interval and writes PNGs to `/public/`.

**Data flow:**
1. Device POSTs to `/api/setup` → stored in SQLite with an auto-generated `api_key`.
2. Device GETs `/api/display` → handler returns redirect to current screen PNG and advances the screen rotation.
3. Worker periodically calls each enabled plugin → plugin renders `public/{api_key}_{screen}.png`.
4. Device downloads the PNG and displays it.

**Screen rotation** is handled by `pkg/v1/screens/` — it cycles through `enabled_plugins` in config order per device.

## Plugin Pattern

Each plugin lives under `pkg/v1/plugins/{name}/` and follows this structure:

1. **Fetch** — API call, JSON unmarshal into typed structs.
2. **Render** — `RenderScreen{Name}(apiKey string, params...) error`:
   - `render.NewImage(800, 480)` creates the canvas.
   - Use `render` package helpers for text (`DrawText`), charts (`gonum/plot`), icon overlays (base64 icons from `pkg/v1/icons/`).
   - Call `render.WriteFile(img, path)` to save the PNG.

Worker calls each plugin's render function and passes `public/{api_key}_{screen}.png` as the output path. Adding a new plugin requires:
- Implementing the fetch + render functions.
- Registering it in the worker's plugin dispatch (currently a switch/if block in `worker.go`).
- Adding it to `enabled_plugins` in `config.yaml`.

## Configuration

`config.yaml` (not committed — use `example_config.yaml` as template):

```yaml
common:
  external_url: "192.168.x.x:8080"  # URL devices use to download images
  port: 8080
  dbpath: "./trmnl.db"
  refresh_time: 300    # seconds between device display refreshes
  update_time: 3600    # seconds between worker data refreshes
  debug: true
  enabled_plugins: ["weather", "twelvedata", "coingecko"]

plugins:
  twelvedata:
    twelvedata_api_key: "..."
    symbols: ["googl", "nvda"]
  coingecko:
    symbols: ["bitcoin"]
  weather:
    location: "Wroclaw"
```

`external_url` must be reachable by physical devices — it's embedded in image URLs returned to devices.

## Key Packages

| Package | Role |
|---|---|
| `pkg/v1/config` | YAML config parsing |
| `pkg/v1/db` | SQLite ops (device CRUD, screen/voltage state) |
| `pkg/v1/render` | Canvas creation, text/chart/icon drawing, PNG output |
| `pkg/v1/screens` | Screen list generation and rotation logic |
| `pkg/v1/icons` | Base64-encoded weather and battery icons |
| `pkg/v1/plugins/stocks` | TwelveData API — 7-day OHLC chart |
| `pkg/v1/plugins/crypto` | CoinGecko API — 24h price chart |
| `pkg/v1/plugins/weather` | Open-Meteo API — forecast with icons |

## Image Rendering Notes

- Canvas is always 800×480 RGBA, converted to grayscale before writing PNG.
- `font.ttf` in the repo root is used for all text rendering via `golang.org/x/image/font`.
- Charts use `gonum.org/v1/plot`; crop/compose with `github.com/anthonynsimon/bild`.
- Rendered PNGs are written to `./public/` relative to the working directory at startup.

## Database

SQLite at the path set by `dbpath`. Schema is initialized automatically on startup. The `devices` table stores `device_id`, `api_key`, `screen` (current plugin name), and `voltage`.
