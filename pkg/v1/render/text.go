package render

import (
	"strings"

	"golang.org/x/image/font/sfnt"
)

// Printable drops the runes the loaded text font has no glyph for, such as
// emoji, joiners and variation selectors, which would otherwise draw as
// boxes. Without a loaded font the string is returned unchanged.
func Printable(s string) string {
	if cachedFont == nil {
		return s
	}
	var buf sfnt.Buffer
	var out strings.Builder
	changed := false
	for _, r := range s {
		if idx, err := cachedFont.GlyphIndex(&buf, r); err != nil || idx == 0 {
			changed = true
			continue
		}
		out.WriteRune(r)
	}
	if !changed {
		return s
	}
	return out.String()
}
