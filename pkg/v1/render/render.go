package render

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
	"trmnl-server-go/pkg/v1/icons"

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

type BoxPlotRecords struct {
	BoxPlotRecord []BoxPlotRecord
}

type BoxPlotRecord struct {
	T    float64
	Vmin float64
	Vmax float64
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
	// TODO: Remove custom font
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
func WriteFile(filename string, img *image.RGBA, voltage float32) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}

	if err := AddImageVoltage(img, voltage, image.Point{-750, -5}); err != nil {
		return err
	}

	bw := ConvertToGray(img)
	if err := png.Encode(f, bw); err != nil {
		f.Close()
		return err
	}

	if err := f.Close(); err != nil {
		return err
	}

	return nil
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

func AddStocksChart(img *image.RGBA, records BoxPlotRecords, chartWidth, chartHeight int, point image.Point) error {
	p := plot.New()
	xticks := plot.TimeTicks{Format: "2006-01-02\n15:04"}
	p.X.Tick.Marker = xticks
	p.Add(plotter.NewGrid())
	var values []*plotter.BoxPlot

	w := vg.Points(2)
	for _, v := range records.BoxPlotRecord {
		t := time.UnixMilli(int64(v.T))
		box := make(plotter.Values, 2)
		box[0] = v.Vmin
		box[1] = v.Vmax
		b, _ := plotter.NewBoxPlot(w, float64(t.Unix()), box)
		values = append(values, b)
		p.Add(b)
	}

	// p.Add(values)

	buf := bytes.NewBuffer(nil)
	writerTo, _ := p.WriterTo(vg.Points(float64(chartWidth)), vg.Points(float64(chartHeight)), "png")
	writerTo.WriteTo(buf)

	chart, _, _ := image.Decode(buf)
	draw.Draw(img, img.Bounds(), chart, point, draw.Over)
	return nil
}

func AddWeatherChart(img *image.RGBA, daily_min, daily_max ChartRecords, chartWidth, chartHeight int, point image.Point) error {
	p := plot.New()
	xticks := plot.TimeTicks{Format: "2006-01-02"}
	p.X.Tick.Marker = xticks
	p.Add(plotter.NewGrid())
	daily_min_data := genPoints(daily_min)

	daily_min_line, _, err := plotter.NewLinePoints(daily_min_data)
	if err != nil {
		log.Panic(err)
	}
	daily_min_line.Color = color.RGBA{A: 255}

	p.Add(daily_min_line)

	daily_max_data := genPoints(daily_max)

	daily_max_line, _, err := plotter.NewLinePoints(daily_max_data)
	if err != nil {
		log.Panic(err)
	}
	daily_max_line.Color = color.RGBA{A: 255}

	p.Add(daily_max_line)
	buf := bytes.NewBuffer(nil)
	writerTo, err := p.WriterTo(vg.Points(float64(chartWidth)), vg.Points(float64(chartHeight)), "png")
	writerTo.WriteTo(buf)

	chart, _, _ := image.Decode(buf)
	draw.Draw(img, img.Bounds(), chart, point, draw.Over)
	return nil
}

func ConvertToGray(img *image.RGBA) *image.Gray {
	target := image.NewGray(img.Bounds())
	draw.Draw(target, target.Bounds(), img, img.Bounds().Min, draw.Src)
	return target
}

func AddImageFromBase64(img *image.RGBA, img64 string, point image.Point) error {
	data := base64.NewDecoder(base64.StdEncoding, strings.NewReader(img64))
	srcImg, err := png.Decode(data)
	if err != nil {
		log.Fatalln("Unable to decode png from base64 image")
		return err
	}
	draw.Draw(img, img.Bounds(), srcImg, point, draw.Over)
	return nil
}

func AddImageVoltage(img *image.RGBA, voltage float32, point image.Point) error {
	batteryPercentage := ((voltage - 3) / 0.012)
	var batteryImage string

	switch {
	case batteryPercentage > 90.0:
		batteryImage = icons.Battery100
	case batteryPercentage > 70.0 && batteryPercentage <= 90.0:
		batteryImage = icons.Battery80
	case batteryPercentage > 70.0 && batteryPercentage <= 90.0:
		batteryImage = icons.Battery80
	case batteryPercentage > 50.0 && batteryPercentage <= 70.0:
		batteryImage = icons.Battery60
	case batteryPercentage > 30.0 && batteryPercentage <= 50.0:
		batteryImage = icons.Battery40
	case batteryPercentage > 10.0 && batteryPercentage <= 30.0:
		batteryImage = icons.Battery20
	case batteryPercentage <= 10.0:
		batteryImage = icons.Battery0
	}

	if err := AddImageFromBase64(img, batteryImage, image.Point{-750, -5}); err != nil {
		return err
	}

	return nil
}

func GetImageByUrl(url string) (image.Image, error) {
	r, err := http.Get(url)
	if err != nil {
		return nil, err
	}

	img, err := jpeg.Decode(r.Body)
	if err != nil {
		return nil, err
	}

	return img, nil
}
