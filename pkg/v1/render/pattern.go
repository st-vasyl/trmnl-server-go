package render

import (
	"image"
	"image/color"
)

// AddDitherRect fills r with a regular grid of black dots, one every step
// pixels in both directions starting at r.Min. It reads as light grey on a
// 1-bit panel without relying on greyscale support. r is clipped to the image.
func AddDitherRect(img *image.RGBA, r image.Rectangle, step int) {
	if step < 1 {
		step = 1
	}
	clip := r.Intersect(img.Bounds())
	for y := clip.Min.Y; y < clip.Max.Y; y++ {
		if (y-r.Min.Y)%step != 0 {
			continue
		}
		for x := clip.Min.X; x < clip.Max.X; x++ {
			if (x-r.Min.X)%step == 0 {
				img.Set(x, y, color.Black)
			}
		}
	}
}

// AddDottedLine draws a horizontal dotted line on row y from x0 up to (not
// including) x1, one black pixel every step pixels.
func AddDottedLine(img *image.RGBA, x0, x1, y, step int) {
	if step < 1 {
		step = 1
	}
	for x := x0; x < x1; x += step {
		img.Set(x, y, color.Black)
	}
}
