package render

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"os"
	"time"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
)

type ChartRecords struct {
	ChartRecord []ChartRecord
}

type ChartRecord struct {
	T float64
	V float64
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

func drawLine(img *image.RGBA, startPosX, startPosY, EndPosX, EndPosY int) {
	for x := startPosX; x <= EndPosX; x++ {
		for y := startPosY; y <= EndPosY; y++ {
			img.Set(x, y, color.Black)
		}
	}
}

func genPoints(r ChartRecords) plotter.XYs {
	pts := make(plotter.XYs, len(r.ChartRecord))
	i := 0
	for _, v := range r.ChartRecord {

		t := time.UnixMilli(int64(v.T))
		pts[i].X = float64(t.Unix())
		pts[i].Y = v.V
		i++
	}
	return pts
}

func AddChart(img *image.RGBA, r ChartRecords, chartWidth, chartHeight int, point image.Point) error {
	p := plot.New()
	xticks := plot.TimeTicks{Format: "2006-01-02\n15:04"}
	p.X.Tick.Marker = xticks
	p.Add(plotter.NewGrid())
	data := genPoints(r)

	line, _, err := plotter.NewLinePoints(data)
	if err != nil {
		log.Panic(err)
	}
	line.Color = color.RGBA{A: 255}

	p.Add(line)

	buf := bytes.NewBuffer(nil)
	writerTo, err := p.WriterTo(vg.Points(float64(chartWidth)), vg.Points(float64(chartHeight)), "png")
	writerTo.WriteTo(buf)

	chart, _, _ := image.Decode(buf)
	draw.Draw(img, img.Bounds(), chart, point, draw.Over)
	return nil
}

// func addImage(img *image.RGBA, path string, point image.Point) error {
// 	f, err := os.Open(path)
// 	if err != nil {
// 		return err
// 	}

// 	chart, _, err := image.Decode(f)

// 	if err != nil {
// 		return err
// 	}

// 	draw.Draw(img, img.Bounds(), chart, point, draw.Over)

// 	return nil
// }
