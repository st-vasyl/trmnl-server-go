package render

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"os"
	"slices"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

type Records struct {
	Prices [][]float64 `json:"prices"`
}

type Point struct {
	X, Y float64
}

// Generate an empty image with given width and height
func NewImage(width, height int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	white := color.RGBA{255, 255, 255, 255}
	draw.Draw(img, img.Bounds(), &image.Uniform{white}, image.Point{}, draw.Src)
	return img
}

// Add a text to the image with given string, start point and a font size
func AddText(img *image.RGBA, text string, point image.Point, col color.Color, fontSize float64) error {
	fontBytes, err := os.ReadFile("font.ttf")
	if err != nil {
		return err
	}

	ttf, err := opentype.Parse(fontBytes)
	if err != nil {
		return err
	}

	face, err := opentype.NewFace(ttf, &opentype.FaceOptions{
		Size:    fontSize,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return err
	}

	drawer := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(col),
		Face: face,
		Dot: fixed.Point26_6{
			X: fixed.I(point.X),
			Y: fixed.I(point.Y),
		},
	}

	drawer.DrawString(text)

	return nil
}

// Write image changes to the file
func WriteFile(filename string, img *image.RGBA) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}

	if err := png.Encode(f, img); err != nil {
		f.Close()
		return err
	}

	if err := f.Close(); err != nil {
		return err
	}

	return nil
}

func AddChart(img *image.RGBA, r Records, width, height, startPosX, startPosY, EndPosX, EndPosY int) error {
	drawLine(img, startPosX, EndPosY, EndPosX, EndPosY)
	drawLine(img, startPosX, startPosY, startPosX, EndPosY)
	drawChart(img, r, startPosX, startPosY, EndPosX, EndPosY)
	return nil
}

func drawLine(img *image.RGBA, startPosX, startPosY, EndPosX, EndPosY int) {
	for x := startPosX; x <= EndPosX; x++ {
		for y := startPosY; y <= EndPosY; y++ {
			img.Set(x, y, color.Black)
		}
	}
}

func drawChart(img *image.RGBA, r Records, startPosX, startPosY, EndPosX, EndPosY int) {
	var prices []int
	for _, v := range r.Prices {
		// log.Printf("Index: %d, value: %d", i, int(v[1]))
		prices = append(prices, int(v[1]))
	}

	lengthY := EndPosY - startPosY
	min_price := slices.Min(prices)
	max_price := slices.Max(prices)

	log.Printf("Coordinates Y: %d, %d \n", startPosY, EndPosY)
	log.Printf("Amount of pixels: %d, and amount of data %d \n", lengthY, len(prices))
	log.Printf("Min price: %d, and Max price: %d \n", min_price, max_price)
	stepX := int((EndPosX - startPosX) / len(prices))
	pointX := startPosX
	log.Printf("Step X: %d \n", stepX)
	for i, _ := range prices {
		price_percentage := (max_price - prices[i]) * 100 / (max_price - min_price)
		pointY := EndPosY - ((EndPosY - startPosY) * price_percentage / 100)
		// img.Set(pointX, pointY, color.Black)
		SetBoldPixel(img, pointX, pointY)
		pointX = pointX + 2
		// log.Printf("Point: {%d,%d} value: %d", pointX, pointY, prices[i])
	}
}

func SetBoldPixel(img *image.RGBA, x, y int) {
	img.Set(x-1, y-1, color.Black)
	img.Set(x-1, y+1, color.Black)
	img.Set(x, y-1, color.Black)
	img.Set(x, y, color.Black)
	img.Set(x, y+1, color.Black)
	img.Set(x-1, y, color.Black)
	img.Set(x+1, y, color.Black)
	img.Set(x+1, y+1, color.Black)
	img.Set(x+1, y-1, color.Black)
}
