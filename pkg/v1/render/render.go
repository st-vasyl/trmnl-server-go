package render

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"net/http"
	"os"
	"time"
	"trmnl-server-go/pkg/v1/icons"

	"github.com/rs/zerolog/log"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
)

var cachedFont *opentype.Font

// SetFont parses and caches the TTF bytes for all subsequent AddText calls.
// Must be called once at startup before rendering begins.
func SetFont(ttfBytes []byte) error {
	f, err := opentype.Parse(ttfBytes)
	if err != nil {
		return err
	}
	cachedFont = f
	return nil
}

func getFont() (*opentype.Font, error) {
	if cachedFont == nil {
		return nil, fmt.Errorf("font not initialized: call render.SetFont first")
	}
	return cachedFont, nil
}

type ChartRecords struct {
	ChartRecord []ChartRecord
}

type ChartRecord struct {
	T float64
	V float64
}

type BoxPlotRecords struct {
	BoxPlotRecord []BoxPlotRecord
	XLabels       map[float64]string // sequential index → date label for X axis
}

type BoxPlotRecord struct {
	T    float64 // sequential index, not a timestamp
	Vmin float64
	Vmax float64
}

type sparseTicks struct{ labels map[float64]string }

func (s sparseTicks) Ticks(min, max float64) []plot.Tick {
	var ticks []plot.Tick
	for pos, label := range s.labels {
		if pos >= min && pos <= max {
			ticks = append(ticks, plot.Tick{Value: pos, Label: label})
		}
	}
	return ticks
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
	ttf, err := getFont()
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

	if err := AddImageVoltage(img, voltage, image.Point{-750, -1}, 40); err != nil {
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
		return err
	}
	line.Color = color.RGBA{A: 255}
	p.Add(line)

	buf := bytes.NewBuffer(nil)
	writerTo, err := p.WriterTo(vg.Points(float64(chartWidth)), vg.Points(float64(chartHeight)), "png")
	writerTo.WriteTo(buf)

	chart, _, err := image.Decode(buf)
	if err != nil {
		return err
	}

	draw.Draw(img, img.Bounds(), chart, point, draw.Over)
	return nil
}

func AddStocksChart(img *image.RGBA, records BoxPlotRecords, chartWidth, chartHeight int, point image.Point) error {
	p := plot.New()
	if n := len(records.BoxPlotRecord); n > 0 {
		p.X.Min = 0
		p.X.Max = float64(n - 1)
	}
	if len(records.XLabels) > 0 {
		p.X.Tick.Marker = sparseTicks{labels: records.XLabels}
	}
	p.Add(plotter.NewGrid())
	var values []*plotter.BoxPlot

	w := vg.Points(2)
	for _, v := range records.BoxPlotRecord {
		box := make(plotter.Values, 2)
		box[0] = v.Vmin
		box[1] = v.Vmax
		b, err := plotter.NewBoxPlot(w, v.T, box)
		if err != nil {
			return err
		}

		values = append(values, b)
		p.Add(b)
	}

	// p.Add(values)

	buf := bytes.NewBuffer(nil)
	writerTo, err := p.WriterTo(vg.Points(float64(chartWidth)), vg.Points(float64(chartHeight)), "png")
	if err != nil {
		return err
	}
	writerTo.WriteTo(buf)

	chart, _, err := image.Decode(buf)
	if err != nil {
		return err
	}
	draw.Draw(img, img.Bounds(), chart, point, draw.Over)
	return nil
}

func ConvertToGray(img *image.RGBA) *image.Gray {
	target := image.NewGray(img.Bounds())
	draw.Draw(target, target.Bounds(), img, img.Bounds().Min, draw.Src)
	return target
}

func AddImageFromBytes(img *image.RGBA, data []byte, point image.Point) error {
	srcImg, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return err
	}
	draw.Draw(img, img.Bounds(), srcImg, point, draw.Over)
	return nil
}

// AddIcon renders an icon by name at the given size and draws it at point. A
// failure to render the icon (e.g. offline on first run) is logged and skipped
// so rendering continues.
func AddIcon(img *image.RGBA, name string, point image.Point, size int) error {
	ic, err := icons.Render(name, size)
	if err != nil {
		log.Warn().Str("icon", name).Err(err).Msg("Skipping icon")
		return nil
	}
	draw.Draw(img, img.Bounds(), ic, point, draw.Over)
	return nil
}

func AddImageVoltage(img *image.RGBA, voltage float32, point image.Point, size int) error {
	batteryPercentage := ((voltage - 3) / 0.012)
	var batteryImage string

	switch {
	case batteryPercentage > 90.0:
		batteryImage = icons.Battery100
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

	if err := AddIcon(img, batteryImage, point, size); err != nil {
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
