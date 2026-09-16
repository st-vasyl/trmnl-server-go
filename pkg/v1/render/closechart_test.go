package render

import (
	"image"
	"image/color"
	"testing"
)

var testWhite = color.RGBA{255, 255, 255, 255}

// countDark returns the number of non-white pixels in img.
func countDark(img *image.RGBA) int {
	n := 0
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if img.RGBAAt(x, y) != testWhite {
				n++
			}
		}
	}
	return n
}

func TestTextWidth_ScalesWithLengthAndSize(t *testing.T) {
	if cachedFont == nil {
		t.Skip("font not loaded (font.ttf missing)")
	}
	short, err := TextWidth("AAPL", 30)
	if err != nil {
		t.Fatalf("TextWidth: %v", err)
	}
	long, _ := TextWidth("AAPLAAPL", 30)
	big, _ := TextWidth("AAPL", 60)

	if short <= 0 {
		t.Errorf("width of AAPL@30 = %d, want > 0", short)
	}
	if long <= short {
		t.Errorf("longer text width %d should exceed %d", long, short)
	}
	if big <= short {
		t.Errorf("bigger font width %d should exceed %d", big, short)
	}
}

func TestTextWidth_ErrorWithoutFont(t *testing.T) {
	withoutFont(t)
	if _, err := TextWidth("x", 10); err == nil {
		t.Fatal("expected error when no font is set")
	}
}

func TestAddRangeBar_MarkerFollowsValue(t *testing.T) {
	img := NewImage(300, 40)
	bar := image.Rect(50, 10, 250, 30)

	// 25 of [0,100] puts the marker a quarter of the way along: x = 50 + 50.
	if err := AddRangeBar(img, 0, 100, 25, bar); err != nil {
		t.Fatalf("AddRangeBar: %v", err)
	}

	if img.RGBAAt(100, 12) == testWhite {
		t.Error("marker should span the full bar height at x=100")
	}
	if img.RGBAAt(200, 20) == testWhite {
		t.Error("track should be drawn through the middle of the bar")
	}
	if img.RGBAAt(200, 12) != testWhite {
		t.Error("track should be thin: top rows away from the marker stay white")
	}
	for _, p := range []image.Point{{49, 20}, {250, 20}, {100, 9}, {100, 30}, {0, 0}, {299, 39}} {
		if img.RGBAAt(p.X, p.Y) != testWhite {
			t.Errorf("pixel %v outside the bar was drawn", p)
		}
	}
}

func TestAddRangeBar_ClampsValueToRange(t *testing.T) {
	bar := image.Rect(50, 10, 250, 30)

	high := NewImage(300, 40)
	if err := AddRangeBar(high, 0, 100, 150, bar); err != nil {
		t.Fatalf("AddRangeBar above range: %v", err)
	}
	if high.RGBAAt(246, 12) == testWhite {
		t.Error("value above the range should pin the marker to the right end")
	}
	if high.RGBAAt(100, 12) != testWhite {
		t.Error("marker must not also appear at the proportional position")
	}

	low := NewImage(300, 40)
	if err := AddRangeBar(low, 0, 100, -5, bar); err != nil {
		t.Fatalf("AddRangeBar below range: %v", err)
	}
	if low.RGBAAt(53, 12) == testWhite {
		t.Error("value below the range should pin the marker to the left end")
	}
}

func TestAddRangeBar_RejectsEmptyRange(t *testing.T) {
	img := NewImage(100, 20)
	if err := AddRangeBar(img, 5, 5, 5, image.Rect(0, 0, 100, 20)); err == nil {
		t.Error("expected error when low == high")
	}
	if err := AddRangeBar(img, 10, 5, 7, image.Rect(0, 0, 100, 20)); err == nil {
		t.Error("expected error when low > high")
	}
}

func TestAddCloseChart_DrawsOnlyInsideTargetRect(t *testing.T) {
	img := NewImage(400, 200)
	s := CloseSeries{Closes: []float64{10, 12, 11, 13, 12, 14}}

	if err := AddCloseChart(img, s, 300, 150, image.Point{50, 25}); err != nil {
		t.Fatalf("AddCloseChart: %v", err)
	}

	inside := 0
	for y := 25; y < 175; y++ {
		for x := 50; x < 350; x++ {
			if img.RGBAAt(x, y) != testWhite {
				inside++
			}
		}
	}
	if inside == 0 {
		t.Error("nothing drawn inside the target rect")
	}
	for _, p := range []image.Point{{0, 0}, {399, 199}, {49, 100}, {350, 100}, {200, 24}, {200, 175}} {
		if img.RGBAAt(p.X, p.Y) != testWhite {
			t.Errorf("pixel %v outside the target rect was drawn", p)
		}
	}
}

func TestAddCloseChart_ReferenceAndSeparatorsAddInk(t *testing.T) {
	closes := []float64{10, 12, 11, 13, 12, 14, 13, 15}
	draw := func(s CloseSeries) int {
		img := NewImage(400, 200)
		if err := AddCloseChart(img, s, 300, 150, image.Point{50, 25}); err != nil {
			t.Fatalf("AddCloseChart: %v", err)
		}
		return countDark(img)
	}

	base := draw(CloseSeries{Closes: closes})
	withRef := draw(CloseSeries{Closes: closes, Reference: 11.5})
	withSep := draw(CloseSeries{Closes: closes, Separators: map[int]string{0: "06/22", 4: "06/23"}})

	if withRef <= base {
		t.Errorf("reference line added no ink: base %d, with reference %d", base, withRef)
	}
	if withSep <= base {
		t.Errorf("separators added no ink: base %d, with separators %d", base, withSep)
	}
}

func TestAddCloseChart_RejectsFewerThanTwoPoints(t *testing.T) {
	img := NewImage(100, 50)
	if err := AddCloseChart(img, CloseSeries{Closes: []float64{1}}, 80, 40, image.Point{}); err == nil {
		t.Error("expected error for a single point")
	}
	if err := AddCloseChart(img, CloseSeries{}, 80, 40, image.Point{}); err == nil {
		t.Error("expected error for no points")
	}
}
