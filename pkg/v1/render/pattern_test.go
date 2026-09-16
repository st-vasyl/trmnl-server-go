package render

import (
	"image"
	"testing"
)

func isBlack(img *image.RGBA, x, y int) bool {
	r, g, b, _ := img.At(x, y).RGBA()
	return r == 0 && g == 0 && b == 0
}

func TestAddDitherRect_SetsEveryStepthPixelInsideOnly(t *testing.T) {
	img := NewImage(20, 20)
	r := image.Rect(4, 6, 14, 16)
	AddDitherRect(img, r, 3)

	for y := 0; y < 20; y++ {
		for x := 0; x < 20; x++ {
			inside := image.Pt(x, y).In(r)
			want := inside && (x-r.Min.X)%3 == 0 && (y-r.Min.Y)%3 == 0
			if got := isBlack(img, x, y); got != want {
				t.Errorf("pixel (%d,%d) black = %v, want %v", x, y, got, want)
			}
		}
	}
}

func TestAddDitherRect_ClipsToImage(t *testing.T) {
	img := NewImage(10, 10)
	AddDitherRect(img, image.Rect(-6, -6, 30, 30), 2) // must not panic
	if !isBlack(img, 0, 0) {
		t.Error("origin lies on the dot grid and should be black")
	}
	if isBlack(img, 1, 1) {
		t.Error("(1,1) lies between dots and should stay white")
	}
}

func TestAddDottedLine_DrawsEveryStepthPixelOnOneRow(t *testing.T) {
	img := NewImage(20, 5)
	AddDottedLine(img, 2, 14, 3, 4)

	for x := 0; x < 20; x++ {
		want := x >= 2 && x < 14 && (x-2)%4 == 0
		if got := isBlack(img, x, 3); got != want {
			t.Errorf("pixel (%d,3) black = %v, want %v", x, got, want)
		}
		if isBlack(img, x, 2) || isBlack(img, x, 4) {
			t.Errorf("row %d touched at x=%d", 2, x)
		}
	}
}
