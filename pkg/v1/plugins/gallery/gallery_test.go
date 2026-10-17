package gallery

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// seedIconFont copies the repo-root font.ttf into the icon cache so the
// battery stamp in render.WriteFile never hits the network. Skips when the
// font is missing.
func seedIconFont(t *testing.T) {
	t.Helper()
	ttf, err := os.ReadFile("../../../../font.ttf")
	if err != nil {
		t.Skipf("font.ttf not available: %v", err)
	}
	if err := os.MkdirAll("icons", 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join("icons", "MaterialSymbols.ttf"), ttf, 0644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll("icons") })
}

// writePNG writes a w×h PNG filled with c to dir/name.
func writePNG(t *testing.T, dir, name string, w, h int, c color.Color) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), &image.Uniform{c}, image.Point{}, draw.Src)
	f, err := os.Create(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

// writeFile writes raw bytes to dir/name.
func writeFile(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
}

// withPick replaces the random index choice. pick receives the number of
// candidate images and returns the index to show.
func withPick(t *testing.T, pick func(n int) int) {
	t.Helper()
	old := randIntN
	randIntN = pick
	t.Cleanup(func() { randIntN = old })
}

// decodeOutput opens the PNG Render wrote.
func decodeOutput(t *testing.T, path string) image.Image {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open output: %v", err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatalf("decode output: %v", err)
	}
	return img
}

// isDark reports whether the output pixel at x,y is closer to black than white.
func isDark(img image.Image, x, y int) bool {
	return color.GrayModel.Convert(img.At(x, y)).(color.Gray).Y < 128
}

func newPlugin(t *testing.T, folder string) *Plugin {
	t.Helper()
	p, err := New(folder)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return p
}

func TestNew_RejectsEmptyFolderSetting(t *testing.T) {
	if _, err := New(""); err == nil {
		t.Fatal("expected error when no folder is configured")
	}
}

func TestNew_RejectsMissingFolder(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope")
	_, err := New(missing)
	if err == nil {
		t.Fatal("expected error for a folder that does not exist")
	}
	if !strings.Contains(err.Error(), missing) {
		t.Errorf("error should name the folder, got %q", err)
	}
}

func TestNew_RejectsFolderWithoutPNGs(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".DS_Store", "junk")
	writeFile(t, dir, "notes.txt", "hello")
	writeFile(t, dir, "._1.png", "macOS resource fork")
	_, err := New(dir)
	if err == nil {
		t.Fatal("expected error for a folder with no usable PNGs")
	}
	if !strings.Contains(err.Error(), dir) {
		t.Errorf("error should name the folder, got %q", err)
	}
}

func TestRender_WritesFullScreenGrayPNG(t *testing.T) {
	seedIconFont(t)
	dir := t.TempDir()
	writePNG(t, dir, "1.png", 800, 480, color.Black)

	out := filepath.Join(t.TempDir(), "gallery.png")
	if err := newPlugin(t, dir).Render("gallery", out, 4.0); err != nil {
		t.Fatalf("Render: %v", err)
	}

	img := decodeOutput(t, out)
	if img.Bounds().Dx() != 800 || img.Bounds().Dy() != 480 {
		t.Errorf("output is %v, want 800x480", img.Bounds())
	}
	if _, ok := img.(*image.Gray); !ok {
		t.Errorf("output should be grayscale like every other screen, got %T", img)
	}
	if !isDark(img, 10, 470) {
		t.Error("output should show the black source image")
	}
}

func TestRender_ShowsThePickedImage(t *testing.T) {
	seedIconFont(t)
	dir := t.TempDir()
	writePNG(t, dir, "a.png", 800, 480, color.White)
	writePNG(t, dir, "b.png", 800, 480, color.Black)
	p := newPlugin(t, dir)

	for _, tc := range []struct {
		index    int
		wantDark bool
	}{{0, false}, {1, true}} {
		withPick(t, func(int) int { return tc.index })
		out := filepath.Join(t.TempDir(), "gallery.png")
		if err := p.Render("gallery", out, 4.0); err != nil {
			t.Fatalf("Render index %d: %v", tc.index, err)
		}
		if got := isDark(decodeOutput(t, out), 10, 470); got != tc.wantDark {
			t.Errorf("index %d: dark = %v, want %v", tc.index, got, tc.wantDark)
		}
	}
}

func TestRender_OnlyPicksAmongVisiblePNGs(t *testing.T) {
	seedIconFont(t)
	dir := t.TempDir()
	writePNG(t, dir, "a.png", 800, 480, color.Black)
	writePNG(t, dir, "B.PNG", 800, 480, color.Black)
	writeFile(t, dir, ".DS_Store", "junk")
	writeFile(t, dir, "._a.png", "macOS resource fork")
	writeFile(t, dir, "notes.txt", "hello")
	writeFile(t, dir, "photo.jpg", "not used")
	if err := os.Mkdir(filepath.Join(dir, "sub.png"), 0755); err != nil {
		t.Fatal(err)
	}
	p := newPlugin(t, dir)

	var candidates int
	withPick(t, func(n int) int { candidates = n; return n - 1 })
	if err := p.Render("gallery", filepath.Join(t.TempDir(), "gallery.png"), 4.0); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if candidates != 2 {
		t.Errorf("picked among %d files, want 2 (a.png and B.PNG only)", candidates)
	}
}

func TestRender_SeesImagesAddedAfterStartup(t *testing.T) {
	seedIconFont(t)
	dir := t.TempDir()
	writePNG(t, dir, "1.png", 800, 480, color.Black)
	p := newPlugin(t, dir)
	writePNG(t, dir, "2.png", 800, 480, color.Black)

	var candidates int
	withPick(t, func(n int) int { candidates = n; return 0 })
	if err := p.Render("gallery", filepath.Join(t.TempDir(), "gallery.png"), 4.0); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if candidates != 2 {
		t.Errorf("picked among %d files, want 2 (the folder is re-read on every render)", candidates)
	}
}

func TestRender_UsesRealRandomness(t *testing.T) {
	seedIconFont(t)
	dir := t.TempDir()
	writePNG(t, dir, "a.png", 800, 480, color.White)
	writePNG(t, dir, "b.png", 800, 480, color.Black)
	p := newPlugin(t, dir)

	// 32 fair picks between two images all landing on one has odds of 2^-31.
	out := filepath.Join(t.TempDir(), "gallery.png")
	seen := map[bool]bool{}
	for range 32 {
		if err := p.Render("gallery", out, 4.0); err != nil {
			t.Fatalf("Render: %v", err)
		}
		seen[isDark(decodeOutput(t, out), 10, 470)] = true
	}
	if len(seen) != 2 {
		t.Errorf("32 renders always showed the same image, want both to appear")
	}
}

func TestRender_EmptiedFolderIsAnErrorAndWritesNothing(t *testing.T) {
	dir := t.TempDir()
	writePNG(t, dir, "1.png", 800, 480, color.Black)
	p := newPlugin(t, dir)
	if err := os.Remove(filepath.Join(dir, "1.png")); err != nil {
		t.Fatal(err)
	}

	out := filepath.Join(t.TempDir(), "gallery.png")
	if err := p.Render("gallery", out, 4.0); err == nil {
		t.Fatal("expected error once the folder has no PNGs left")
	}
	if _, err := os.Stat(out); err == nil {
		t.Error("no PNG should be written when there is nothing to show")
	}
}

func TestRender_CorruptPNGIsAnErrorAndWritesNothing(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "broken.png", "not really a png")
	p := newPlugin(t, dir)

	out := filepath.Join(t.TempDir(), "gallery.png")
	err := p.Render("gallery", out, 4.0)
	if err == nil {
		t.Fatal("expected error for a file that does not decode")
	}
	if !strings.Contains(err.Error(), "broken.png") {
		t.Errorf("error should name the file, got %q", err)
	}
	if _, statErr := os.Stat(out); statErr == nil {
		t.Error("no PNG should be written when the image does not decode")
	}
}

func TestRender_ScalesImageThatIsNotScreenSize(t *testing.T) {
	seedIconFont(t)
	dir := t.TempDir()
	writePNG(t, dir, "small.png", 400, 240, color.Black)

	out := filepath.Join(t.TempDir(), "gallery.png")
	if err := newPlugin(t, dir).Render("gallery", out, 4.0); err != nil {
		t.Fatalf("Render: %v", err)
	}
	// Same aspect ratio, so the whole canvas is covered. Check away from the
	// battery icon in the top-right corner.
	img := decodeOutput(t, out)
	if !isDark(img, 10, 470) || !isDark(img, 790, 470) || !isDark(img, 10, 10) {
		t.Error("a 400x240 image should be scaled to cover the 800x480 canvas")
	}
}

func TestRender_UnknownScreenIsAnError(t *testing.T) {
	dir := t.TempDir()
	writePNG(t, dir, "1.png", 800, 480, color.Black)
	p := newPlugin(t, dir)

	out := filepath.Join(t.TempDir(), "x.png")
	for _, screen := range []string{"weather", "gallery_1", ""} {
		if err := p.Render(screen, out, 4.0); err == nil {
			t.Errorf("Render(%q): expected error", screen)
		}
	}
	if _, err := os.Stat(out); err == nil {
		t.Error("no PNG should be written for an unknown screen")
	}
}

func TestNameAndScreens(t *testing.T) {
	dir := t.TempDir()
	writePNG(t, dir, "1.png", 800, 480, color.Black)
	p := newPlugin(t, dir)
	if p.Name() != "gallery" {
		t.Errorf("Name = %q, want gallery", p.Name())
	}
	if got := strings.Join(p.Screens(), ","); got != "gallery" {
		t.Errorf("Screens = %v, want [gallery] (one screen, whatever the folder holds)", p.Screens())
	}
}
