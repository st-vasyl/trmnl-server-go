package fonts

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"trmnl-server-go/pkg/v1/httpclient"

	"github.com/rs/zerolog/log"
)

const DefaultFont = "Anonymous Pro"

// Load returns TTF bytes for the given Google Fonts family name.
// On first call it downloads the TTF and caches it at ./fonts/{name}.ttf.
// Subsequent calls read from the cache without network access.
func Load(name string) ([]byte, error) {
	if name == "" {
		name = DefaultFont
	}

	path := cachePath(name)

	if data, err := os.ReadFile(path); err == nil {
		log.Info().Str("font", name).Str("path", path).Msg("Font loaded from cache")
		return data, nil
	}

	log.Info().Str("font", name).Msg("Downloading font from Google Fonts")

	data, err := download(name)
	if err != nil {
		return nil, fmt.Errorf("download font %q: %w", name, err)
	}

	if err := os.MkdirAll("./fonts", 0755); err != nil {
		return nil, fmt.Errorf("create fonts dir: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		log.Warn().Str("path", path).Err(err).Msg("Failed to cache font to disk")
	}

	log.Info().Str("font", name).Str("path", path).Msg("Font downloaded and cached")
	return data, nil
}

func cachePath(name string) string {
	safe := strings.ToLower(strings.ReplaceAll(name, " ", "_"))
	// filepath.Base strips any directory separators or traversal segments, so a
	// crafted font name cannot escape the ./fonts cache directory.
	safe = filepath.Base(safe)
	return fmt.Sprintf("./fonts/%s.ttf", safe)
}

func download(name string) ([]byte, error) {
	cssURL := "https://fonts.googleapis.com/css2?family=" + url.QueryEscape(name)

	css, err := httpclient.Get(cssURL)
	if err != nil {
		return nil, fmt.Errorf("fetch CSS: %w", err)
	}

	ttfURL, err := extractTTFURL(string(css))
	if err != nil {
		return nil, err
	}
	if err := validateFontURL(ttfURL); err != nil {
		return nil, err
	}

	return httpclient.Get(ttfURL)
}

// validateFontURL ensures the font file URL extracted from Google's CSS points
// at Google's font CDN over HTTPS, bounding where the server will fetch from.
func validateFontURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("parse font url: %w", err)
	}
	if u.Scheme != "https" || u.Hostname() != "fonts.gstatic.com" {
		return fmt.Errorf("unexpected font url %q", raw)
	}
	return nil
}

func extractTTFURL(css string) (string, error) {
	start := strings.Index(css, "url(")
	if start < 0 {
		return "", fmt.Errorf("no url() found in Google Fonts CSS response")
	}
	start += len("url(")
	end := strings.Index(css[start:], ")")
	if end < 0 {
		return "", fmt.Errorf("malformed url() in Google Fonts CSS response")
	}
	return css[start : start+end], nil
}
