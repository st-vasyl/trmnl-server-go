package icons

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sync"

	"github.com/rs/zerolog/log"
	"github.com/tdewolff/font"
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
