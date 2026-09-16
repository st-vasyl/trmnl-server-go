package render

import (
	"os"
	"testing"
)

// withRepoFont installs the repo-root font.ttf as the text font, restoring
// whatever was cached before. Skips when the file is missing.
func withRepoFont(t *testing.T) {
	t.Helper()
	ttf, err := os.ReadFile("../../../font.ttf")
	if err != nil {
		t.Skipf("font.ttf not available: %v", err)
	}
	orig := cachedFont
	t.Cleanup(func() { cachedFont = orig })
	if err := SetFont(ttf); err != nil {
		t.Fatalf("SetFont: %v", err)
	}
}

func TestPrintable_DropsRunesTheFontLacks(t *testing.T) {
	withRepoFont(t)
	if got := Printable("a\U0001F527b‍️"); got != "ab" {
		t.Errorf("Printable = %q, want ab", got)
	}
	if got, want := Printable("plain ASCII - text"), "plain ASCII - text"; got != want {
		t.Errorf("Printable = %q, want %q", got, want)
	}
}

func TestPrintable_WithoutFontReturnsInput(t *testing.T) {
	orig := cachedFont
	cachedFont = nil
	t.Cleanup(func() { cachedFont = orig })
	if got := Printable("a\U0001F527b"); got != "a\U0001F527b" {
		t.Errorf("Printable = %q, want the input unchanged", got)
	}
}

func TestTextWidth_IgnoresRunesTheFontLacks(t *testing.T) {
	withRepoFont(t)
	plain, err := TextWidth("ab", 20)
	if err != nil {
		t.Fatalf("TextWidth: %v", err)
	}
	mixed, err := TextWidth("a\U0001F527b", 20)
	if err != nil {
		t.Fatalf("TextWidth: %v", err)
	}
	if plain != mixed || plain == 0 {
		t.Errorf("width with emoji = %d, want %d (the emoji must not count)", mixed, plain)
	}
}
