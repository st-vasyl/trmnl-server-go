package render

import (
	"image"
	"image/color"
	"testing"
)

func TestAddSparkline_DrawsOnlyInsideTargetRect(t *testing.T) {
	img := NewImage(200, 100)

	if err := AddSparkline(img, []float64{1, 2, 3, 2, 4}, 100, 50, image.Point{50, 25}); err != nil {
		t.Fatalf("AddSparkline: %v", err)
	}

	white := color.RGBA{255, 255, 255, 255}

	inside := false
	for y := 25; y < 75 && !inside; y++ {
		for x := 50; x < 150; x++ {
			if img.RGBAAt(x, y) != white {
				inside = true
				break
			}
		}
	}
	if !inside {
		t.Error("no pixels drawn inside the 100x50 target rect at (50,25)")
	}

	// Corners of the canvas and the first row/column just outside the target
	// rect on every side must remain untouched.
	for _, p := range []image.Point{{0, 0}, {199, 99}, {49, 50}, {150, 50}, {100, 24}, {100, 75}} {
		if got := img.RGBAAt(p.X, p.Y); got != white {
			t.Errorf("pixel %v = %v, want untouched white", p, got)
		}
	}
}

func TestAddSparkline_RejectsFewerThanTwoValues(t *testing.T) {
	img := NewImage(50, 50)

	if err := AddSparkline(img, []float64{1}, 40, 40, image.Point{5, 5}); err == nil {
		t.Fatal("expected error for a single value")
	}
	if err := AddSparkline(img, nil, 40, 40, image.Point{5, 5}); err == nil {
		t.Fatal("expected error for no values")
	}
}
