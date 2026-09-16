package render

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"strconv"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/text"
	"gonum.org/v1/plot/vg"
	vgdraw "gonum.org/v1/plot/vg/draw"
	"gonum.org/v1/plot/vg/vgimg"
)

// CloseSeries is the input to AddCloseChart: one close per bar in
// chronological order plus optional decorations.
type CloseSeries struct {
	Closes     []float64
	Separators map[int]string // bar index → X label; each also gets a thin vertical line
	Reference  float64        // dashed horizontal line (e.g. previous close); 0 disables it
}

// TextWidth returns the advance width in pixels of text as AddText would draw
// it at fontSize.
func TextWidth(s string, fontSize float64) (int, error) {
	ttf, err := getFont()
	if err != nil {
		return 0, err
	}
	face, err := opentype.NewFace(ttf, &opentype.FaceOptions{Size: fontSize, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		return 0, err
	}
	defer face.Close()
	return font.MeasureString(face, s).Ceil(), nil
}

// AddRangeBar draws a thin horizontal track across r with a full-height marker
// at value's position between lo and hi. Values outside [lo, hi] pin the
// marker to the nearest end. r is in destination coordinates.
func AddRangeBar(img *image.RGBA, lo, hi, value float64, r image.Rectangle) error {
	if hi <= lo {
		return fmt.Errorf("range bar needs lo < hi, got %v and %v", lo, hi)
	}
	if r.Empty() {
		return fmt.Errorf("range bar needs a non-empty rectangle")
	}
	black := image.NewUniform(color.Black)

	mid := (r.Min.Y + r.Max.Y) / 2
	track := image.Rect(r.Min.X, mid-2, r.Max.X, mid+2).Intersect(r)
	draw.Draw(img, track, black, image.Point{}, draw.Src)

	const markerW = 8
	frac := math.Max(0, math.Min(1, (value-lo)/(hi-lo)))
	cx := r.Min.X + int(math.Round(frac*float64(r.Dx()-1)))
	x0 := cx - markerW/2
	if x0 < r.Min.X {
		x0 = r.Min.X
	}
	if x0+markerW > r.Max.X {
		x0 = r.Max.X - markerW
	}
	draw.Draw(img, image.Rect(x0, r.Min.Y, x0+markerW, r.Max.Y), black, image.Point{}, draw.Src)
	return nil
}

// AddCloseChart draws a thick close-price line exactly width×height pixels
// with its top-left corner at point (destination coordinates). Separators
// become thin vertical lines with an X tick label, Reference becomes a dashed
// horizontal line, and the highest and lowest closes are labelled.
func AddCloseChart(img *image.RGBA, s CloseSeries, width, height int, point image.Point) error {
	n := len(s.Closes)
	if n < 2 {
		return fmt.Errorf("close chart needs at least 2 points, got %d", n)
	}

	loIdx, hiIdx := 0, 0
	for i, v := range s.Closes {
		if v < s.Closes[loIdx] {
			loIdx = i
		}
		if v > s.Closes[hiIdx] {
			hiIdx = i
		}
	}
	loClose, hiClose := s.Closes[loIdx], s.Closes[hiIdx]

	lo, hi := loClose, hiClose
	if s.Reference != 0 {
		lo, hi = math.Min(lo, s.Reference), math.Max(hi, s.Reference)
	}
	pad := (hi - lo) * 0.15
	if pad == 0 {
		pad = math.Max(math.Abs(hi)*0.01, 1)
	}
	yMin, yMax := lo-pad, hi+pad

	p := plot.New()
	p.X.Min, p.X.Max = 0, float64(n-1)
	p.Y.Min, p.Y.Max = yMin, yMax
	p.X.Padding, p.Y.Padding = 0, 0
	p.X.Tick.Label.Font.Size = vg.Points(14)
	p.Y.Tick.Label.Font.Size = vg.Points(14)
	p.X.Tick.Marker = sparseTicks{labels: indexLabels(s.Separators)}
	p.Y.Tick.Marker = niceTicks{n: 4}

	for idx := range s.Separators {
		if idx <= 0 || idx >= n {
			continue // the first bar sits on the Y axis already
		}
		sep, err := plotter.NewLine(plotter.XYs{{X: float64(idx), Y: yMin}, {X: float64(idx), Y: yMax}})
		if err != nil {
			return err
		}
		sep.Color = color.Black
		sep.Width = vg.Points(1)
		p.Add(sep)
	}

	if s.Reference != 0 {
		ref, err := plotter.NewLine(plotter.XYs{{X: 0, Y: s.Reference}, {X: float64(n - 1), Y: s.Reference}})
		if err != nil {
			return err
		}
		ref.Color = color.Black
		ref.Width = vg.Points(1.5)
		ref.Dashes = []vg.Length{vg.Points(6), vg.Points(4)}
		p.Add(ref)
	}

	pts := make(plotter.XYs, n)
	for i, v := range s.Closes {
		pts[i].X, pts[i].Y = float64(i), v
	}
	line, err := plotter.NewLine(pts)
	if err != nil {
		return err
	}
	line.Color = color.Black
	line.Width = vg.Points(3)
	p.Add(line)

	labels, err := plotter.NewLabels(plotter.XYLabels{
		XYs:    plotter.XYs{{X: float64(hiIdx), Y: hiClose}, {X: float64(loIdx), Y: loClose}},
		Labels: []string{fmt.Sprintf("%.2f", hiClose), fmt.Sprintf("%.2f", loClose)},
	})
	if err != nil {
		return err
	}
	for i := range labels.TextStyle {
		labels.TextStyle[i].Font.Size = vg.Points(13)
	}
	labels.TextStyle[0].YAlign = text.YBottom // above the high
	labels.TextStyle[1].YAlign = text.YTop    // below the low
	labels.TextStyle[0].XAlign = alignAwayFromEdge(hiIdx, n)
	labels.TextStyle[1].XAlign = alignAwayFromEdge(loIdx, n)
	p.Add(labels)

	// At 72 DPI one vg point is one pixel, so the canvas is exactly width×height.
	c := vgimg.NewWith(
		vgimg.UseWH(vg.Points(float64(width)), vg.Points(float64(height))),
		vgimg.UseDPI(72),
	)
	p.Draw(vgdraw.New(c))

	dst := image.Rect(point.X, point.Y, point.X+width, point.Y+height)
	draw.Draw(img, dst, c.Image(), image.Point{}, draw.Over)
	return nil
}

// alignAwayFromEdge keeps a point label inside the plot: labels on the right
// half extend leftwards, the rest extend rightwards.
func alignAwayFromEdge(idx, n int) text.XAlignment {
	if idx > n/2 {
		return text.XRight
	}
	return text.XLeft
}

func indexLabels(m map[int]string) map[float64]string {
	out := make(map[float64]string, len(m))
	for i, l := range m {
		out[float64(i)] = l
	}
	return out
}

// niceTicks produces about n major ticks at round values.
type niceTicks struct{ n int }

func (t niceTicks) Ticks(min, max float64) []plot.Tick {
	if max <= min || t.n < 1 {
		return nil
	}
	step := niceStep((max - min) / float64(t.n))
	start := math.Ceil(min / step)
	var ticks []plot.Tick
	for i := 0; ; i++ {
		v := (start + float64(i)) * step
		if v > max {
			break
		}
		ticks = append(ticks, plot.Tick{Value: v, Label: formatTick(v)})
	}
	return ticks
}

// niceStep rounds raw up to 1, 2, 2.5 or 5 times a power of ten.
func niceStep(raw float64) float64 {
	mag := math.Pow(10, math.Floor(math.Log10(raw)))
	for _, m := range []float64{1, 2, 2.5, 5} {
		if raw <= m*mag {
			return m * mag
		}
	}
	return 10 * mag
}

// formatTick prints a value with only the decimals it needs.
func formatTick(v float64) string {
	return strconv.FormatFloat(math.Round(v*100)/100, 'f', -1, 64)
}
