package render

import (
	"image"
	"image/color"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

// drawText draws text at the given X and Y position
func DrawText(img *image.RGBA, x, y int, text string, col color.Color) {
	face := basicfont.Face7x13
	// face.Height = 104
	// face.Width = 48

	// Draw the text
	point := fixed.Point26_6{
		X: fixed.Int26_6(x * 64),
		Y: fixed.Int26_6(y * 64),
	}

	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(col),
		Face: face,
		Dot:  point,
	}
	d.DrawString(text)
}
